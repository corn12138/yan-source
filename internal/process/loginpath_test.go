package process

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLoginPath(t *testing.T) {
	root := t.TempDir()
	tools, base := filepath.Join(root, "tools with spaces"), filepath.Join(root, "base")
	sep := string(filepath.ListSeparator)
	frame := func(path string) string { return "welcome\n" + pathFrame + path + "\x00goodbye\n" }
	for _, tc := range []struct {
		name, output, want string
	}{
		{"ordered", frame(strings.Join([]string{tools, base, tools, ".", "", "relative"}, sep)), tools + sep + base},
		{"missing", "PATH=" + tools, ""},
		{"unterminated", pathFrame + tools, ""},
		{"empty", frame(""), ""},
		{"relative only", frame("relative"), ""},
		{"duplicate", frame(tools) + frame(base), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseLoginPath([]byte(tc.output), base); got != tc.want {
				t.Fatalf("PATH = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReplacePathPreservesEnvironment(t *testing.T) {
	env := []string{"PATH=old", "HOME=home", "GIT_SSH_COMMAND=custom ssh", "PATH=duplicate"}
	got := replacePath(env, "new")
	if strings.Join(got, "|") != "HOME=home|GIT_SSH_COMMAND=custom ssh|PATH=new" || env[0] != "PATH=old" {
		t.Fatalf("replacement mutated or dropped environment: got %v, input %v", got, env)
	}
}

func TestStopLoginShellPathWithoutOwner(t *testing.T) {
	if err := StopLoginShellPath(); err != nil {
		t.Fatal(err)
	}
}
