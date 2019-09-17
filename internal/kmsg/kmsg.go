// Package kmsg contains support for parsing Linux kernel log entries read from
// /dev/kmsg. These are the same log entries that can be read via the `dmesg`
// command. Each read from /dev/kmsg is guaranteed to return a single log entry,
// so no line-splitting is required. If the read buffer is not large enough to
// contain the log entry, `os.ErrInvalid` is returned.
//
// More information can be found here:
// https://www.kernel.org/doc/Documentation/ABI/testing/dev-kmsg
package kmsg

import (
	"errors"
	"strconv"
	"strings"
)

// Entry is a single log entry in kmsg.
type Entry struct {
	SyslogPriority     uint8
	SyslogFacility     uint8
	Seq                uint64
	TimeSinceBootMicro uint64
	Flags              string
	Message            string
}

var (
	// ErrInvalidFormat indicates the kmsg entry failed to parse.
	ErrInvalidFormat = errors.New("invalid kmsg format")
)

// Parse takes a single kmsg log entry string and returns a struct representing
// the components of the log entry.
func Parse(s string) (*Entry, error) {
	fields := strings.SplitN(s, ";", 2)
	if len(fields) < 2 {
		return nil, ErrInvalidFormat
	}
	prefixFields := strings.SplitN(fields[0], ",", 5)
	if len(prefixFields) < 4 {
		return nil, ErrInvalidFormat
	}
	syslog, err := strconv.ParseUint(prefixFields[0], 10, 16)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	seq, err := strconv.ParseUint(prefixFields[1], 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	timestamp, err := strconv.ParseUint(prefixFields[2], 10, 64)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	return &Entry{
		SyslogPriority:     uint8(syslog & 0x7),
		SyslogFacility:     uint8(syslog >> 3),
		Seq:                seq,
		TimeSinceBootMicro: timestamp,
		Flags:              prefixFields[3],
		Message:            fields[1],
	}, nil
}
