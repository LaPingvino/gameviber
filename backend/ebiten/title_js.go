//go:build js

package ebiten

import "syscall/js"

// setTitle: in the browser, the page's title (Ebitengine's window title is a
// no-op there).
func setTitle(s string) { js.Global().Get("document").Set("title", s) }
