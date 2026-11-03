//go:build !js

package ebiten

import eb "github.com/hajimehoshi/ebiten/v2"

// setTitle is the window's title.
func setTitle(s string) { eb.SetWindowTitle(s) }
