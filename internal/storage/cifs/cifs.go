package cifs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/Microsoft/opengcs/internal/network"
	"github.com/Microsoft/opengcs/internal/oc"
	"github.com/pkg/errors"
	"github.com/vishvananda/netns"
	"go.opencensus.io/trace"
	"golang.org/x/sys/unix"
)

const cifsMountOptions = "cache=loose,actimeo=10000,dir_mode=0777,file_mode=0777,sec=ntlmssp,mfsymlinks,ro,async"

var (
	osMkdirAll  = os.MkdirAll
	osRemoveAll = os.RemoveAll
	unixMount   = unix.Mount
)

// Mount creates a cifs mount at `target`. If `target` doesn't exist this call
// will try and create it.
//
// The UVM generally doesn't have a network adapter in the default network namespace so this function
// will lock the current goroutines thread, join a network namespace with an available adapter and then
// perform the mount.
func Mount(ctx context.Context, source, target, username, password string) (err error) {
	_, span := trace.StartSpan(ctx, "cifs.Mount")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()

	span.AddAttributes(
		trace.StringAttribute("target", target),
		trace.StringAttribute("source", source))

	if err := osMkdirAll(target, 0700); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			osRemoveAll(target)
		}
	}()

	cmd := exec.Command(
		"mount",
		"-t",
		"cifs",
		source,
		target,
		"-o",
		fmt.Sprintf("vers=3.0,username=%s,password=%s,%s", username, password, cifsMountOptions),
	)

	pid, err := getNSPid()
	if err != nil {
		return errors.Wrap(err, "failed to get network namespace PID to perform cifs mount")
	}

	// Get a reference to the new network namespace
	ns, err := netns.GetFromPid(pid)
	if err != nil {
		return errors.Wrapf(err, "netns.GetFromPid(%d) failed", pid)
	}
	defer ns.Close()

	return network.DoInNetNS(ns, cmd.Run)
}

// This is gross, there HAS to be a better way
func getNSPid() (int, error) {
	data, err := ioutil.ReadFile("/tmp/netnscfg.log")
	if err != nil {
		return -1, err
	}

	subStr := "from PID "
	netNsStr := string(data)
	newStr := netNsStr[strings.Index(netNsStr, subStr)+len(subStr):]

	var pidStr string
	for _, char := range newStr {
		if !unicode.IsDigit(char) {
			break
		}
		pidStr += string(char)
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return -1, err
	}
	return pid, nil
}
