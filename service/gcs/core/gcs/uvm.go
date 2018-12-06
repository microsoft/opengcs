package gcs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/Microsoft/opengcs/service/gcs/gcserr"
	"github.com/Microsoft/opengcs/service/gcs/oslayer"
	"github.com/Microsoft/opengcs/service/gcs/prot"
	"github.com/Microsoft/opengcs/service/gcs/runtime"
	"github.com/Microsoft/opengcs/service/gcs/stdio"
	"github.com/Microsoft/opengcs/service/gcs/transport"
	oci "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// UVMContainerID is the ContainerID that will be sent on any prot.MessageBase
// for V2 where the specific message is targeted at the UVM itself.
const UVMContainerID = "00000000-0000-0000-0000-000000000000"

// Host is the structure tracking all UVM host state including all containers
// and processes.
type Host struct {
	containersMutex sync.Mutex
	containers      map[string]*Container

	// Rtime is the Runtime interface used by the GCS core.
	rtime runtime.Runtime
	osl   oslayer.OS
	vsock transport.Transport
}

func NewHost(rtime runtime.Runtime, osl oslayer.OS, vsock transport.Transport) *Host {
	return &Host{rtime: rtime, osl: osl, vsock: vsock, containers: make(map[string]*Container)}
}

func (h *Host) getContainerLocked(id string) (*Container, error) {
	if c, ok := h.containers[id]; !ok {
		return nil, errors.WithStack(gcserr.NewContainerDoesNotExistError(id))
	} else {
		return c, nil
	}
}

func (h *Host) GetAllProcessPids() []uint32 {
	h.containersMutex.Lock()
	defer h.containersMutex.Unlock()

	pids := make([]uint32, 0)
	for _, c := range h.containers {
		c.processesMutex.Lock()
		for _, p := range c.processes {
			pids = append(pids, p.pid)
		}
		c.processesMutex.Unlock()
	}
	return pids
}

func (h *Host) GetContainer(id string) (*Container, error) {
	h.containersMutex.Lock()
	defer h.containersMutex.Unlock()

	return h.getContainerLocked(id)
}

func (h *Host) CreateContainer(id string, settings *prot.VMHostedContainerSettingsV2) (*Container, error) {
	h.containersMutex.Lock()
	defer h.containersMutex.Unlock()

	c, err := h.getContainerLocked(id)
	if err == nil {
		return c, nil
	}

	// Container doesnt exit. Create it here
	// Create the BundlePath
	if err := h.osl.MkdirAll(settings.OCIBundlePath, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to create OCIBundlePath: '%s'", settings.OCIBundlePath)
	}
	configFile := path.Join(settings.OCIBundlePath, "config.json")
	f, err := h.osl.Create(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create config.json at: '%s'", configFile)
	}
	defer f.Close()
	writer := bufio.NewWriter(f)
	if err := json.NewEncoder(writer).Encode(settings.OCISpecification); err != nil {
		return nil, errors.Wrapf(err, "failed to write OCISpecification to config.json at: '%s'", configFile)
	}
	if err := writer.Flush(); err != nil {
		return nil, errors.Wrapf(err, "failed to flush writer for config.json at: '%s'", configFile)
	}

	con, err := h.rtime.CreateContainer(id, settings.OCIBundlePath, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create container")
	}
	c = &Container{
		id:        id,
		vsock:     h.vsock,
		spec:      settings.OCISpecification,
		container: con,
		processes: make(map[uint32]*Process),
	}
	// Add the WG count for the init process
	c.processesWg.Add(1)
	c.initProcess = newProcess(c, settings.OCISpecification.Process, con.(runtime.Process), uint32(c.container.Pid()))
	h.containers[id] = c
	return c, nil
}

func (h *Host) ModifyHostSettings(settings *prot.ModifySettingRequest) error {
	type modifyFunc func(interface{}) error

	requestTypeFn := func(req prot.ModifyRequestType, setting interface{}, add, remove, update modifyFunc) error {
		switch req {
		case prot.MreqtAdd:
			if add != nil {
				return add(setting)
			}
			break
		case prot.MreqtRemove:
			if remove != nil {
				return remove(setting)
			}
			break
		case prot.MreqtUpdate:
			if update != nil {
				return update(setting)
			}
			break
		}

		return errors.Errorf("the RequestType \"%s\" is not supported", req)
	}

	var add modifyFunc
	var remove modifyFunc
	var update modifyFunc

	switch settings.ResourceType {
	case prot.MrtMappedVirtualDisk:
		add = func(setting interface{}) error {
			mvd := setting.(*prot.MappedVirtualDiskV2)
			return h.mountScsi(mvd.MountPath, prot.ScsiMount{
				Controller: mvd.Controller,
				Lun:        mvd.Lun,
				Writable:   !mvd.ReadOnly,
			})
		}
		remove = func(setting interface{}) error {
			mvd := setting.(*prot.MappedVirtualDiskV2)
			return h.removeScsi(mvd.MountPath, prot.ScsiMount{
				Controller: mvd.Controller,
				Lun:        mvd.Lun,
				Writable:   !mvd.ReadOnly,
			})
		}
	case prot.MrtMappedDirectory:
		add = func(setting interface{}) error {
			md := setting.(*prot.MappedDirectoryV2)
			return mountPlan9Share(h.osl, h.vsock, md.MountPath, md.ShareName, md.Port, md.ReadOnly)
		}
		remove = func(setting interface{}) error {
			md := setting.(*prot.MappedDirectoryV2)
			return unmountPath(h.osl, md.MountPath, true)
		}
	case prot.MrtVPMemDevice:
		add = func(setting interface{}) error {
			vpd := setting.(*prot.MappedVPMemDeviceV2)
			return h.mountPmem(vpd.MountPath, prot.PMemMount{
				DeviceNumber: vpd.DeviceNumber,
			})
		}
		remove = func(setting interface{}) error {
			vpd := setting.(*prot.MappedVPMemDeviceV2)
			return unmountPath(h.osl, vpd.MountPath, true)
		}
	case prot.MrtCombinedLayers:
		add = func(setting interface{}) error {
			cl := setting.(*prot.CombinedLayersV2)
			if cl.ContainerRootPath == "" {
				return errors.New("cannot combine layers with empty ContainerRootPath")
			}
			if err := h.osl.MkdirAll(cl.ContainerRootPath, 0700); err != nil {
				return errors.Wrapf(err, "failed to create ContainerRootPath directory '%s'", cl.ContainerRootPath)
			}

			layerPaths := make([]string, len(cl.Layers))
			for i, layer := range cl.Layers {
				layerPaths[i] = layer.Path
			}

			var upperdirPath string
			var workdirPath string
			var mountOptions uintptr
			if cl.ScratchPath == "" {
				// The user did not pass a scratch path. Mount overlay as readonly.
				mountOptions |= syscall.O_RDONLY
			} else {
				upperdirPath = filepath.Join(cl.ScratchPath, "upper")
				workdirPath = filepath.Join(cl.ScratchPath, "work")
			}

			return mountOverlay(h.osl, layerPaths, upperdirPath, workdirPath, cl.ContainerRootPath, mountOptions)
		}
		remove = func(setting interface{}) error {
			cl := setting.(*prot.CombinedLayersV2)
			return unmountPath(h.osl, cl.ContainerRootPath, true)
		}
	case prot.MrtBulkCombineLayers:
		add = func(setting interface{}) error {
			bcl := setting.(*prot.BulkCombineLayersV2)
			return h.bulkCombineLayers(bcl)
		}
	default:
		return errors.Errorf("the resource type \"%s\" is not supported", settings.ResourceType)
	}

	if err := requestTypeFn(settings.RequestType, settings.Settings, add, remove, update); err != nil {
		return errors.Wrapf(err, "Failed to modify ResourceType: \"%s\"", settings.ResourceType)
	}
	return nil
}

func (h *Host) bulkCombineLayers(bcl *prot.BulkCombineLayersV2) (err error) {
	var (
		mountedLayers []prot.Mount
		layerPaths    []string
		upperdirPath  string
		workdirPath   string
		mountOptions  uintptr
	)
	if bcl.RootfsPath == "" {
		return errors.New("cannot bulk combine layers with empty rootfs")
	}
	defer func() {
		if err != nil {
			for _, ml := range mountedLayers {
				h.removeLayer(ml)
			}
		}
	}()
	// TODO: No need to do this sync.
	for _, m := range bcl.Layers {
		if err = h.mountLayer(m); err != nil {
			return err
		}
		layerPaths = append(layerPaths, m.MountPath)
	}
	if bcl.Scratch.MountPath == "" {
		// The user did not pass a scratch path. Mount overlay as readonly.
		mountOptions |= syscall.O_RDONLY
	} else {
		if err = h.mountScratch(bcl.Scratch); err != nil {
			return err
		}
		defer func() {
			if err != nil {
				h.removeLayer(bcl.Scratch)
			}
		}()
		upperdirPath = filepath.Join(bcl.Scratch.MountPath, "upper")
		workdirPath = filepath.Join(bcl.Scratch.MountPath, "work")
	}
	return mountOverlay(h.osl, layerPaths, upperdirPath, workdirPath, bcl.RootfsPath, mountOptions)
}

func (h *Host) mountLayer(m prot.Mount) error {
	if m.MountPath == "" {
		return errors.New("failed to mount layer with empty mount path")
	}
	if (m.Scsi == nil && m.PMem == nil) ||
		(m.Scsi != nil && m.PMem != nil) {
		return errors.New("failed to mount layer must specify exactly one of `Scsi` or `PMem`")
	} else if m.Scsi != nil {
		if m.Scsi.Writable {
			return errors.New("failed to mount SCSI layer must not be `Writable`")
		}
		return h.mountScsi(m.MountPath, *m.Scsi)
	}

	return h.mountPmem(m.MountPath, *m.PMem)
}

func (h *Host) removeLayer(m prot.Mount) error {
	if m.Scsi != nil {
		return h.removeScsi(m.MountPath, *m.Scsi)
	}
	return unmountPath(h.osl, m.MountPath, true)
}

// mountScratch mounts a scratch mount to the mount path requested.
//
// It is the callers responsibility to check for m.MountPath != "" before
// calling this function.
func (h *Host) mountScratch(m prot.Mount) error {
	if m.PMem != nil {
		return errors.New("failed to mount scratch, pmem mount not currently supported")
	}
	if m.Scsi == nil {
		return errors.New("failed to mount scratch, no scsi mount provided")
	}
	return h.mountScsi(m.MountPath, *m.Scsi)
}

func (h *Host) mountScsi(mountPath string, s prot.ScsiMount) error {
	scsiName, err := scsiControllerLunToName(h.osl, s.Controller, s.Lun)
	if err != nil {
		return errors.Wrapf(err, "failed to mount SCSI")
	}
	ms := mountSpec{
		Source:     scsiName,
		FileSystem: defaultFileSystem,
		Flags:      uintptr(0),
	}
	if !s.Writable {
		ms.Flags |= syscall.MS_RDONLY
		ms.Options = append(ms.Options, mountOptionNoLoad)
	}
	if mountPath != "" {
		if err := h.osl.MkdirAll(mountPath, 0700); err != nil {
			return errors.Wrapf(err, "failed to create directory for SCSI mount %s", mountPath)
		}
		if err := ms.MountWithTimedRetry(h.osl, mountPath); err != nil {
			return errors.Wrapf(err, "failed to mount directory for SCSI mount %s", mountPath)
		}
	}
	return nil
}

func (h *Host) removeScsi(mountPath string, s prot.ScsiMount) error {
	if mountPath != "" {
		if err := unmountPath(h.osl, mountPath, true); err != nil {
			return errors.Wrapf(err, "failed to remove SCSI mount path: '%s'", mountPath)
		}
	}
	return h.osl.UnplugSCSIDisk(fmt.Sprintf("0:0:%d:%d", s.Controller, s.Lun))
}

func (h *Host) mountPmem(mountPath string, p prot.PMemMount) error {
	ms := &mountSpec{
		Source:     "/dev/pmem" + strconv.FormatUint(uint64(p.DeviceNumber), 10),
		FileSystem: defaultFileSystem,
		Flags:      syscall.MS_RDONLY,
		Options:    []string{mountOptionNoLoad, mountOptionDax},
	}
	return mountLayer(h.osl, mountPath, ms)
}

// Shutdown terminates this UVM. This is a destructive call and will destroy all
// state that has not been cleaned before calling this function.
func (h *Host) Shutdown() {
	h.osl.Shutdown()
}

type Container struct {
	id    string
	vsock transport.Transport

	spec *oci.Spec

	container   runtime.Container
	initProcess *Process

	processesMutex sync.Mutex
	processesWg    sync.WaitGroup
	processes      map[uint32]*Process
}

func (c *Container) Start(conSettings stdio.ConnectionSettings) (int, error) {
	stdioSet, err := stdio.Connect(c.vsock, conSettings)
	if err != nil {
		return -1, err
	}
	if c.initProcess.spec.Terminal {
		ttyr := c.container.Tty()
		ttyr.ReplaceConnectionSet(stdioSet)
		ttyr.Start()
	} else {
		pr := c.container.PipeRelay()
		pr.ReplaceConnectionSet(stdioSet)
		pr.CloseUnusedPipes()
		pr.Start()
	}
	err = c.container.Start()
	if err != nil {
		stdioSet.Close()
	}
	return int(c.initProcess.pid), err
}

func (c *Container) ExecProcess(process *oci.Process, conSettings stdio.ConnectionSettings) (int, error) {
	logrus.Debugf("container::ExecProcess %+v", *process)
	stdioSet, err := stdio.Connect(c.vsock, conSettings)
	if err != nil {
		return -1, err
	}

	// Increment the waiters before the call so that WaitContainer cannot complete in a race
	// with adding a new process. When the process exits it will decrement this count.
	c.processesMutex.Lock()
	c.processesWg.Add(1)
	c.processesMutex.Unlock()

	p, err := c.container.ExecProcess(process, stdioSet)
	if err != nil {
		// We failed to exec any process. Remove our early count increment.
		c.processesMutex.Lock()
		c.processesWg.Done()
		c.processesMutex.Unlock()
		stdioSet.Close()
		return -1, err
	}

	pid := p.Pid()
	c.processesMutex.Lock()
	c.processes[uint32(pid)] = newProcess(c, process, p, uint32(pid))
	c.processesMutex.Unlock()
	return pid, nil
}

// GetProcess returns the *Process with the matching 'pid'. If the 'pid' does
// not exit returns error.
func (c *Container) GetProcess(pid uint32) (*Process, error) {
	logrus.Debugf("container: %s, get process: %d", c.id, pid)
	if c.initProcess.pid == pid {
		return c.initProcess, nil
	}

	c.processesMutex.Lock()
	defer c.processesMutex.Unlock()

	p, ok := c.processes[pid]
	if !ok {
		return nil, errors.WithStack(gcserr.NewProcessDoesNotExistError(int(pid)))
	}
	return p, nil
}

// Kill sends 'signal' to the container process.
func (c *Container) Kill(signal oslayer.Signal) error {
	logrus.Debugf("container: %s, sending kill %v", c.id, signal)
	return c.container.Kill(signal)
}

// Wait waits for all processes exec'ed to finish as well as the init process
// representing the container.
func (c *Container) Wait() int {
	logrus.Debugf("container: %s, beginning wait", c.id)
	c.processesWg.Wait()
	return c.initProcess.exitCode
}

// Process is a struct that defines the lifetime and operations associated with
// an oci.Process.
type Process struct {
	spec *oci.Process

	process runtime.Process
	pid     uint32
	// This is only valid post the exitWg
	exitCode int
	exitWg   sync.WaitGroup

	// Used to allow addtion/removal to the writersWg after an initial wait has
	// already been issued. It is not safe to call Add/Done without holding this
	// lock.
	writersSyncRoot sync.Mutex
	// Used to track the number of writers that need to finish
	// before the process can be marked for cleanup.
	writersWg sync.WaitGroup
	// Used to track the 1st caller to the writersWg that successfully
	// acknowledges it wrote the exit response.
	writersCalled bool
}

// newProcess returns a Process struct that has been initialized with an
// outstanding wait for process exit, and post exit an outstanding wait for
// process cleanup to release all resources once at least 1 waiter has
// successfully written the exit response.
func newProcess(c *Container, spec *oci.Process, process runtime.Process, pid uint32) *Process {
	p := &Process{
		spec:    spec,
		process: process,
		pid:     pid,
	}
	p.exitWg.Add(1)
	p.writersWg.Add(1)
	go func() {
		// Wait for the process to exit
		state, err := p.process.Wait()
		if err != nil {
			logrus.Errorf("process: %d, failed to wait for runc process", p.pid)
			p.exitCode = -1
		} else {
			p.exitCode = state.ExitCode()
		}
		logrus.Debugf("process: %d, exited with code: %d", p.pid, p.exitCode)
		// Free any process waiters
		p.exitWg.Done()
		// Decrement any container process count waiters
		c.processesMutex.Lock()
		c.processesWg.Done()
		c.processesMutex.Unlock()

		// Schedule the removal of this process object from the map once at
		// least one waiter has read the result
		go func() {
			p.writersWg.Wait()
			c.processesMutex.Lock()
			logrus.Debugf("process: %d, all waiters have completed, removing process", p.pid)
			delete(c.processes, p.pid)
			c.processesMutex.Unlock()
		}()
	}()
	return p
}

// Kill sends 'signal' to the process.
func (p *Process) Kill(signal syscall.Signal) error {
	logrus.Debugf("process: %d, sending kill %v", p.pid, signal)
	if err := syscall.Kill(int(p.pid), signal); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Wait returns a channel that can be used to wait for the process to exit and
// gather the exit code. The second channel must be signaled from the caller
// when the caller has completed its use of this call to Wait.
func (p *Process) Wait() (<-chan int, chan<- bool) {
	logrus.Debugf("process: %d, beginning wait", p.pid)
	exitCodeChan := make(chan int, 1)
	doneChan := make(chan bool)

	// Increment our waiters for this waiter
	p.writersSyncRoot.Lock()
	p.writersWg.Add(1)
	p.writersSyncRoot.Unlock()

	go func() {
		bgExitCodeChan := make(chan int, 1)
		go func() {
			p.exitWg.Wait()
			bgExitCodeChan <- p.exitCode
		}()

		// Wait for the exit code or the caller to stop waiting.
		select {
		case exitCode := <-bgExitCodeChan:
			exitCodeChan <- exitCode

			// The caller got the exit code. Wait for them to tell us they have
			// issued the write
			select {
			case <-doneChan:
				p.writersSyncRoot.Lock()
				// Decrement this waiter
				logrus.Debugf("process: %d, wait completed, releasing wait count", p.pid)
				p.writersWg.Done()
				if !p.writersCalled {
					// We have at least 1 response for the exit code for this
					// process. Decrement the release waiter that will free the
					// process resources when the writersWg hits 0
					logrus.Debugf("process: %d, first wait completed, releasing first wait count", p.pid)
					p.writersCalled = true
					p.writersWg.Done()
				}
				p.writersSyncRoot.Unlock()
			}

		case <-doneChan:
			// In this case the caller timed out before the process exited. Just
			// decrement the waiter but since no exit code we just deal with our
			// waiter.
			p.writersSyncRoot.Lock()
			logrus.Debugf("process: %d, wait canceled before exit, releasing wait count", p.pid)
			p.writersWg.Done()
			p.writersSyncRoot.Unlock()
		}
	}()
	return exitCodeChan, doneChan
}
