//go:build selftest

package gui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo"
)

// The marker is inside the disposable fixture: an accidental submit is observable
// without changing the user's files, shell configuration or installed application.
func (f *Frontend) SelfTestReview(dir string, report func(string)) {
	go func() {
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		defer onMain(func() { mygo.App.Quit() })
		time.Sleep(2 * time.Second)
		var s *termSession
		onMain(func() {
			f.newTerminal(dir)
			if len(f.terms) > 0 {
				s = f.terms[len(f.terms)-1]
			}
		})
		if s == nil {
			report("FAIL native terminal did not open")
			return
		}
		deadline := time.Now().Add(15 * time.Second)
		for !bytes.Contains(s.t.Snapshot(), []byte("\x1b[?2004h")) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if !bytes.Contains(s.t.Snapshot(), []byte("\x1b[?2004h")) {
			report("FAIL shell did not enable bracketed paste")
			return
		}
		marker := filepath.Join(dir, "unexpected-submit")
		var fr view.Frame
		onMain(func() {
			f.panelToColumn(s, 0, 0)
			if err := f.ed.OpenFile(filepath.Join(dir, "review.go")); err != nil {
				report("FAIL open fixture: " + err.Error())
				return
			}
			f.ed.Run("review.comment")
			f.ed.SetReviewInput("yan-review-native-paste\ntouch " + marker)
			f.ed.Run("review.save")
			f.ed.Run("review.send")
			f.ed.Frame(&fr)
		})
		deadline = time.Now().Add(5 * time.Second)
		for !strings.Contains(s.t.Text(), "yan-review-native-paste") && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if !strings.Contains(s.t.Text(), "yan-review-native-paste") || fr.ReviewCount != 0 {
			report(fmt.Sprintf("FAIL paste/count: %d %s", fr.ReviewCount, fr.Status.Message))
			return
		}
		time.Sleep(time.Second)
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			report("FAIL review was submitted to shell")
			return
		}
		report("PASS native terminal tab received multiline review without submitting")
		onMain(func() { f.ed.Run("review.undo"); f.ed.Frame(&fr) })
		if fr.ReviewCount != 1 {
			report("FAIL undo after handoff")
			return
		}
		report("PASS successful handoff can restore saved review")
		// Cancel the pending shell input before closing the isolated terminal.
		s.t.Send([]byte{3})
	}()
}
