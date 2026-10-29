//go:build linux

package gui

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo/ui"
)

// On Linux the window's buttons sit in a GTK header bar over the title bar. Its empty title
// labels catch clicks meant for Yan's buttons beside them, and the system theme gives the
// buttons embossed shadows and colors of its own. A CSS provider on the screen collapses the
// labels and colors the buttons from Yan's palette, replaced as the theme changes (after Mayoi
// PR #57 by bet4it).

var gtk struct {
	once  sync.Once
	ready bool

	cssProviderNew       func() uintptr
	cssProviderLoad      func(provider uintptr, data *byte, length int, err unsafe.Pointer) bool
	screenDefault        func() uintptr
	addProviderForScreen func(screen, provider uintptr, priority uint32)
	removeProvider       func(screen, provider uintptr)
	unref                func(obj uintptr)

	labels      uintptr // the provider collapsing the labels
	buttons     uintptr // the provider coloring the buttons
	muted, text ui.Color
}

func loadGTK() bool {
	gtk.once.Do(func() {
		libGTK, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		libGDK, err := purego.Dlopen("libgdk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		libGObject, err := purego.Dlopen("libgobject-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&gtk.cssProviderNew, libGTK, "gtk_css_provider_new")
		purego.RegisterLibFunc(&gtk.cssProviderLoad, libGTK, "gtk_css_provider_load_from_data")
		purego.RegisterLibFunc(&gtk.screenDefault, libGDK, "gdk_screen_get_default")
		purego.RegisterLibFunc(&gtk.addProviderForScreen, libGTK, "gtk_style_context_add_provider_for_screen")
		purego.RegisterLibFunc(&gtk.removeProvider, libGTK, "gtk_style_context_remove_provider_for_screen")
		purego.RegisterLibFunc(&gtk.unref, libGObject, "g_object_unref")
		gtk.ready = true
	})
	return gtk.ready
}

// gtkProvider adds a provider of css to the screen and returns it, or 0.
func gtkProvider(css string) uintptr {
	screen := gtk.screenDefault()
	if screen == 0 {
		return 0
	}
	p := gtk.cssProviderNew()
	if p == 0 {
		return 0
	}
	data := append([]byte(css), 0)
	gtk.cssProviderLoad(p, &data[0], len(data)-1, nil)
	gtk.addProviderForScreen(screen, p, 800) // GTK_STYLE_PROVIDER_PRIORITY_USER
	return p                                 // kept: it is removed and released when replaced
}

const gtkLabelsCSS = `
headerbar.mygo-window-controls label.title,
headerbar.mygo-window-controls label.subtitle,
headerbar.mygo-window-controls .title {
    font-size: 0px; min-width: 0px; min-height: 0px; margin: 0px; padding: 0px;
}`

// styleWindowControls styles the header bar's window buttons with the palette's colors. It runs
// on the main thread as the window is drawn; a palette that did not change costs nothing.
func styleWindowControls(muted, text ui.Color) {
	if !loadGTK() {
		return
	}
	if gtk.labels == 0 {
		gtk.labels = gtkProvider(gtkLabelsCSS)
	}
	if gtk.buttons != 0 && muted == gtk.muted && text == gtk.text {
		return
	}
	css := fmt.Sprintf(`
headerbar.mygo-window-controls button.titlebutton,
headerbar.mygo-window-controls:backdrop button.titlebutton,
headerbar.mygo-window-controls button.titlebutton:backdrop {
    color: %s; -gtk-icon-shadow: none; text-shadow: none;
}
headerbar.mygo-window-controls button.titlebutton:hover,
headerbar.mygo-window-controls button.titlebutton:active {
    color: %s;
}`, cssColor(muted), cssColor(text))
	p := gtkProvider(css)
	if p == 0 {
		return
	}
	if old := gtk.buttons; old != 0 {
		if screen := gtk.screenDefault(); screen != 0 {
			gtk.removeProvider(screen, old)
		}
		gtk.unref(old)
	}
	gtk.buttons, gtk.muted, gtk.text = p, muted, text
}

// cssColor writes a color for CSS.
func cssColor(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.3f)", c.R, c.G, c.B, float64(c.A)/255)
}
