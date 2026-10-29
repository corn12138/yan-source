package process

import (
	"os"
	"strings"
	"testing"
)

func TestHelperOutput(t *testing.T) {
	if os.Getenv("YAN_PROCESS_HELPER") == "1" {
		os.Stdout.WriteString("helper output")
		os.Exit(0)
	}
	cmd := Command(os.Args[0], "-test.run=^TestHelperOutput$")
	cmd.Env = append(os.Environ(), "YAN_PROCESS_HELPER=1")
	b, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(b)) != "helper output" {
		t.Fatalf("helper: %q, %v", b, err)
	}
}
