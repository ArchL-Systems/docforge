package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"build", "--help"}} {
		var stdout, stderr bytes.Buffer
		if err := Run(args, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(stdout.String()+stderr.String(), "Usage") {
			t.Fatal("missing help")
		}
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"build", "--unknown"}, {"build", "extra"}} {
		var stdout, stderr bytes.Buffer
		if err := Run(args, &stdout, &stderr); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
