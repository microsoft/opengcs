package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/docker/pkg/archive"
	"github.com/sirupsen/logrus"
)

func changes() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: changes RW_LAYER [RO_LAYERS...]")
	}

	c, err := archive.OverlayChanges(os.Args[2:], os.Args[1])
	if err != nil {
		return fmt.Errorf("archive.OverlayChanges failed: %s", err)
	}

	out, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("json.Marshal failed: %s", err)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		return fmt.Errorf("os.Stdout.Write failed: %s", err)
	}
	return nil
}

func changesMain() {
	if err := changes(); err != nil {
		logrus.Fatal("changes returned: ", err)
	}
	os.Exit(0)
}
