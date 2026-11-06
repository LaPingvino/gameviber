//go:build js

package player

import (
	"syscall/js"

	ebbackend "github.com/LaPingvino/gameviber/backend/ebiten"
)

// The editor's Play controls, for the page around the game: pause (true /
// false) and one step while paused.
func init() {
	js.Global().Set("gameviberPause", js.FuncOf(func(this js.Value, args []js.Value) any {
		ebbackend.Pause(len(args) > 0 && args[0].Truthy())
		return ebbackend.Paused()
	}))
	js.Global().Set("gameviberStep", js.FuncOf(func(this js.Value, args []js.Value) any {
		ebbackend.StepOnce()
		return nil
	}))
}
