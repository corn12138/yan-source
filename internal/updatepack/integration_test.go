package updatepack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the real MyGo downloader and bundle replacement, confined to a disposable app.
// The helper does not start a native UI or touch an installed copy of Yan.
func TestMyGoUpdateIntegration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS bundle replacement")
	}
	bundle, pub, key := fixture(t)
	out := t.TempDir()
	m, err := Pack(bundle, out, "v0.1.0-alpha.11", "darwin-arm64", "", key)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(out, "yan-v0.1.0-alpha.11-darwin-arm64.update.tar.gz")
	serveDir := t.TempDir()
	server := httptest.NewServer(http.FileServer(http.Dir(serveDir)))
	defer server.Close()
	m.URL = server.URL + "/archive.tar.gz"
	b, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(serveDir, "archive.tar.gz"), b, 0644); err != nil {
		t.Fatal(err)
	}
	writeManifest := func(v Manifest) {
		t.Helper()
		data, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(serveDir, "manifest.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(m)
	installed := filepath.Join(t.TempDir(), "yan.app")
	exe := filepath.Join(installed, "Contents", "MacOS", "yan")
	if err = os.MkdirAll(filepath.Dir(exe), 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(installed, "old-marker")
	if err = os.WriteFile(marker, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "main.go")
	code := `package main
import("context";"fmt";"os";"time";"github.com/egoist/mygo")
func main(){mygo.App.SetVersion(os.Args[1]);ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();up,err:=mygo.Updater.Check(ctx);if err!=nil{fmt.Println(err);os.Exit(1)};if up==nil{fmt.Println("current");return};if os.Args[2]=="install"{if err=up.Install(ctx,nil);err!=nil{fmt.Println(err);os.Exit(1)}};fmt.Println(up.Version)}
`
	if err = os.WriteFile(source, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	flags := fmt.Sprintf("-X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageUpdateFeed=%s/manifest.json -X github.com/egoist/mygo.packageUpdateKey=%s", server.URL, pub)
	cmd := exec.Command("go", "build", "-ldflags", flags, "-o", exe, source)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	run := func(current, action, want string, success bool) {
		t.Helper()
		out, err := exec.Command(exe, current, action).CombinedOutput()
		if (err == nil) != success || !strings.Contains(string(out), want) {
			t.Fatalf("%s: %s, %v", action, out, err)
		}
	}
	run("0.1.0-alpha.9", "check", "0.1.0-alpha.11", true)
	run("0.1.0-alpha.11", "check", "current", true)
	run("0.1.0", "check", "current", true)
	bad := m
	bad.Signature = "invalid"
	writeManifest(bad)
	run("0.1.0-alpha.9", "install", "not signed", false)
	if _, err = os.Stat(marker); err != nil {
		t.Fatal("bad signature touched installed app")
	}
	writeManifest(m)
	badBytes := append([]byte(nil), b...)
	badBytes[len(badBytes)/2] ^= 1
	if err = os.WriteFile(filepath.Join(serveDir, "archive.tar.gz"), badBytes, 0644); err != nil {
		t.Fatal(err)
	}
	run("0.1.0-alpha.9", "install", "not signed", false)
	if _, err = os.Stat(marker); err != nil {
		t.Fatal("corrupt archive touched installed app")
	}
	if err = os.WriteFile(filepath.Join(serveDir, "archive.tar.gz"), b, 0644); err != nil {
		t.Fatal(err)
	}
	run("0.1.0-alpha.9", "install", "0.1.0-alpha.11", true)
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("old bundle remained")
	}
	got, err := os.ReadFile(exe)
	if err != nil || string(got) != "new executable" {
		t.Fatalf("new bundle: %q, %v", got, err)
	}
}
