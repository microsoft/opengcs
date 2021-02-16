package loopback

import (
	"context"
	"fmt"
	"os"

	"go.opencensus.io/trace"
	"golang.org/x/sys/unix"

	"github.com/Microsoft/opengcs/internal/log"
	"github.com/Microsoft/opengcs/internal/oc"
	"github.com/pkg/errors"
)

const (
	EmptyMode         = 0x0
	LOOP_CTL_ADD      = 0x4c80
	LOOP_CTL_GET_FREE = 0x4c82
	LOOP_CTL_REMOVE   = 0x4c81
	LOOP_CLR_FD       = 0x4c01
	LOOP_SET_FD       = 0x4c00
	SYS_IOCTL         = 16
)

const ext4Options = "noatime,barrier=0,errors=remount-ro,ro"

// Test dependencies
var (
	osMkdirAll  = os.MkdirAll
	osRemoveAll = os.RemoveAll
	unixMount   = unix.Mount
)

// Mount mounts `backingFile` as a loopback device on device number `device`. The loopback device
// is then ext4 mounted at `target`. If `target` doesn't exist it will be created.
func Mount(ctx context.Context, device int, target, backingFile string) (err error) {
	_, span := trace.StartSpan(ctx, "loopback.Mount")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()

	span.AddAttributes(
		trace.StringAttribute("target", target),
		trace.Int64Attribute("device", int64(device)),
		trace.StringAttribute("backingFile", backingFile))

	if err := osMkdirAll(target, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			osRemoveAll(target)
		}
	}()

	// If the device already exists `getLoopID` will return -1, so skip this if the device
	// we're asking for already exists. The UVM starts with 8 loop devices (0,1,2....7)
	// already so this should be hit exactly 8 times before we have to actually allocate
	// a new loop device.
	loopID := device
	if _, err := os.Stat(fmt.Sprintf("/dev/loop%d", device)); err != nil {
		loopCtrl, err := getLoopController(context.Background())
		if err != nil {
			return errors.Wrap(err, "failed to get loop controller")
		}

		// Provision loop device
		loopID = getLoopID(ctx, loopCtrl, device)
		if loopID == -1 {
			return errors.New("failed to provision loop device")
		}
	}

	// Pair loop dev with backing file
	source, err := pairLoopDevice(ctx, loopID, backingFile)
	if err != nil {
		return errors.Wrapf(err, "failed to pair backing file %s to loop device with ID %d", backingFile, loopID)
	}

	return unixMount(source, target, "ext4", uintptr(unix.MS_RDONLY), "noload")
}

// Teardown a file backed loop device
func Teardown(ctx context.Context, device int) error {
	if err := RemoveBackingFile(ctx, device); err != nil {
		return errors.Wrap(err, "could not teardown loop device")
	}

	loopCtrl, err := getLoopController(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get loop controller")
	}

	// remove loop device
	deallocateLoopDevice(ctx, loopCtrl, device)
	return nil
}

func getLoopController(ctx context.Context) (int, error) {
	// Get loop controller
	fd, err := safeOpen(ctx, "/dev/loop-control", unix.O_RDWR, EmptyMode)
	if err != nil {
		return 0, err
	}
	return fd, nil
}

func getLoopID(ctx context.Context, loopControllerFD, device int) int {
	loopID, err := allocateLoopDevice(ctx, loopControllerFD, device)
	if err != nil {
		return -1
	}
	return loopID
}

// Implement some methods from https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=770fe30a46a12b6fb6b63fbe1737654d28e8484
/*
Note: devnr means device number
Example in C:
 cfd = open("/dev/loop-control", O_RDWR);

 # add a new specific loop device
 err = ioctl(cfd, LOOP_CTL_ADD, devnr);

 # remove a specific loop device
 err = ioctl(cfd, LOOP_CTL_REMOVE, devnr);

 # find or allocate a free loop device to use
 devnr = ioctl(cfd, LOOP_CTL_GET_FREE);

 sprintf(loopname, "/dev/loop%i", devnr);
 ffd = open("backing-file", O_RDWR);
 lfd = open(loopname, O_RDWR);
 err = ioctl(lfd, LOOP_SET_FD, ffd);
*/

func allocateLoopDevice(ctx context.Context, loopController, deviceNumber int) (int, error) {
	device, err := safeIoctl(ctx, loopController, LOOP_CTL_ADD, deviceNumber)
	if err == unix.EEXIST {
		return deviceNumber, unix.EEXIST
	}
	return int(device), nil
}

func deallocateLoopDevice(ctx context.Context, loopController, deviceNumber int) int {
	device, err := safeIoctl(ctx, loopController, LOOP_CTL_REMOVE, deviceNumber)
	if err == unix.EBUSY {
		log.G(ctx).WithField("deviceNumber", deviceNumber).Error("loop device is busy")
		return int(device)
	}

	log.G(ctx).WithField("deviceNumber", device).Debug("Removed loop device")
	return int(device)
}

// PairLoopDevice pairs a backing file `backingFile` to the loop device with device number
// `deviceNumber`. Returns the loop device path, e.g. "/dev/loop9"
func pairLoopDevice(ctx context.Context, deviceNumber int, backingFile string) (loopName string, err error) {
	loopName = fmt.Sprintf("/dev/loop%d", deviceNumber)
	ffd, err := safeOpen(ctx, backingFile, unix.O_RDONLY, EmptyMode)
	if err != nil {
		return "", err
	}
	defer unix.Close(ffd)

	lfd, err := safeOpen(ctx, loopName, unix.O_RDONLY, EmptyMode)
	if err != nil {
		return "", err
	}
	defer unix.Close(lfd)

	_, err = safeIoctl(ctx, lfd, LOOP_SET_FD, ffd)
	if err != nil {
		return "", err
	}
	return loopName, nil
}

func RemoveBackingFile(ctx context.Context, deviceNumber int) error {
	loopName := fmt.Sprintf("/dev/loop%d", deviceNumber)
	log.G(ctx).WithField("loopName", loopName).Debug("Trying to remove backing file")

	lfd, err := safeOpen(ctx, loopName, unix.O_RDONLY, EmptyMode)
	if err != nil {
		return err
	}
	defer unix.Close(lfd)

	_, err = safeIoctl(ctx, lfd, LOOP_CLR_FD, 0)
	return err
}
