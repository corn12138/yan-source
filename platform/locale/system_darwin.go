package locale

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// System is the user's ordered language list from NSLocale.preferredLanguages, which includes a
// language chosen for Yan alone in System Settings and the per-launch override
// "-AppleLanguages (en)". The locale environment is the fallback when Foundation can't be read.
func System() []string {
	if tags := preferredLanguages(); len(tags) > 0 {
		return tags
	}
	return Terminal()
}

var foundation = sync.OnceValue(func() bool {
	_, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	return err == nil
})

func preferredLanguages() []string {
	if !foundation() {
		return nil
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	langs := objc.ID(objc.GetClass("NSLocale")).Send(objc.RegisterName("preferredLanguages"))
	if langs == 0 {
		return nil
	}
	n := objc.Send[uint](langs, objc.RegisterName("count"))
	at, utf8 := objc.RegisterName("objectAtIndex:"), objc.RegisterName("UTF8String")
	tags := make([]string, 0, n)
	for i := range n {
		if s := objc.Send[string](langs.Send(at, i), utf8); s != "" {
			tags = append(tags, s)
		}
	}
	return tags
}
