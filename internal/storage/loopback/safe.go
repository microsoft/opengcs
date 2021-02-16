package loopback

import (
	"context"
	"errors"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type (
	open struct {
		path string
		mode int
		perm uint32
	}

	opened struct {
		open
		fd int
	}

	call struct {
		fd  int
		req uint
		val int
	}

	called struct {
		call
		result uintptr
	}
)

func safeOpen(ctx context.Context, path string, mode int, perm uint32) (int, error) {
	const maxAttempts = 20
	const delay = (5 * time.Second) / maxAttempts
	file := open{
		path,
		mode,
		perm,
	}

	process := &opened{
		file,
		-1,
	}

	err := process.Wait(ctx)
	if err != nil {
		return -1, err
	}

	return process.fd, nil
}

func safeIoctl(ctx context.Context, fd int, req uint, val int) (uintptr, error) {
	const maxAttempts = 20
	const delay = (5 * time.Second) / maxAttempts
	c := call{
		fd,
		req,
		val,
	}

	process := &called{
		c,
		0,
	}

	err := process.Wait(ctx)
	if err != nil {
		return 0, err
	}

	return process.result, nil
}

func (call *opened) Wait(ctx context.Context) error {
	if (call.open == open{}) {
		return errors.New("Open parameters are not set")
	}

	fd, err := unix.Open(call.path, call.mode, call.perm)
	if err != nil {
		return err
	}

	call.fd = fd
	return nil
}

func (c *called) Wait(ctx context.Context) error {
	if (c.call == call{}) {
		return errors.New("Syscall parameters are not set")
	}

	result, errno := ioctl(c.fd, c.req, c.val)
	if err := isErr(errno); err != nil {
		return err
	}

	c.result = result
	return nil
}

func ioctl(fd int, req uint, val int) (uintptr, syscall.Errno) {
	r1, _, errno := unix.Syscall(SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(int32(val)))
	return r1, errno
}

func isErr(errno syscall.Errno) error {
	var err error
	err = nil
	if errno != 0 {
		err = errno
	}
	return err
}
