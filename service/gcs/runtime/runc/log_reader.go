package runc

import (
	"encoding/json"
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

type logReader struct {
	path      string
	lastError interface{}
	reader    *os.File
}

func newLogReader(logPath string) (*logReader, error) {
	_, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		if err := syscall.Mkfifo(logPath, 0777); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return &logReader{
		path: logPath,
	}, nil
}

func (r *logReader) startRead() {
	reader, err := os.Open(r.path)
	if err != nil {
		return
	}
	r.reader = reader
	defer func() {
		r.stopRead()
	}()

	dec := json.NewDecoder(reader)
	for i := 0; i < 1; i++ {
		var out map[string]interface{}
		if err := dec.Decode(&out); err != nil {
			break
		}
		logLevel, ok := out["level"].(string)
		if !ok {
			continue
		}

		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			continue
		}
		delete(out, "time")
		delete(out, "level")
		if level <= logrus.ErrorLevel {
			r.lastError = out["msg"]
		}
		logrus.NewEntry(logrus.StandardLogger()).Log(level, out)
	}
}

func (r *logReader) getLastError() interface{} {
	return r.lastError
}

func (r *logReader) stopRead() error {
	if r.reader != nil {
		err := r.reader.Close()
		r.reader = nil
		return err
	}
	return nil
}
