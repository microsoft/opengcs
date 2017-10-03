package commonutils

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

type stacklogger struct {
	levels []logrus.Level
}

// NewStackHook creates a new hook to append the stack to log messages.
func NewStackHook(levels []logrus.Level) logrus.Hook {
	return &stacklogger{levels}
}

func (h *stacklogger) Levels() []logrus.Level {
	return h.levels
}

func (h *stacklogger) Fire(e *logrus.Entry) error {
	// Skip logrus's 7 frames
	skip := 7
	if len(e.Data) != 0 {
		// Called with WithFields(), so skip 5 frames instead
		skip = 5
	}

	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return fmt.Errorf("failed to get caller info")
	}

	f := runtime.FuncForPC(pc)
	// Remove the github.com/.../service part because it's not useful
	name := strings.TrimPrefix(f.Name(), "github.com/Microsoft/opengcs/service/")
	e.Message = fmt.Sprintf("%s:%d %s() %s", filepath.Base(file), line, name, e.Message)
	return nil
}
