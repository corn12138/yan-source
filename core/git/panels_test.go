package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPanelReads(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	bare := filepath.Join(t.TempDir(), "o.git")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a"), []byte("1\n"), 0o644)
	run("add", ".")
	run("commit", "-qm", "one")
	run("init", "-q", "--bare", bare)
	run("remote", "add", "o", bare)
	run("push", "-q", "-u", "o", "main")
	os.WriteFile(filepath.Join(dir, "a"), []byte("2\n"), 0o644)
	run("commit", "-qam", "two")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs, err := r.PanelBranches()
	if err != nil || len(bs) != 2 || bs[0].Name != "main" || !bs[0].Current || bs[0].Ahead != 1 || bs[0].Upstream != "o/main" || !bs[1].Remote {
		t.Fatalf("branches %+v %v", bs, err)
	}
	out, err := r.LogRange(bs[0].UpstreamRef, bs[0].Ref(), 10)
	if err != nil || len(out) != 1 || out[0].Subject != "two" {
		t.Fatalf("range %+v %v", out, err)
	}
	all, err := r.LogRef(bs[0].Ref(), 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("log %+v %v", all, err)
	}
	if _, err := r.LogRef("--all", 1); err == nil {
		t.Fatal("an option passed as a ref")
	}
	rm, err := r.RemoteURLs("o")
	if err != nil || len(rm.FetchURLs) != 1 || len(rm.PushURLs) != 1 {
		t.Fatalf("remote %+v %v", rm, err)
	}
	urls, err := r.ConfiguredRemoteURLs()
	if err != nil || urls["o"] != bare {
		t.Fatalf("urls %v %v", urls, err)
	}
}
