//go:build js

// Package webfiles is runner.FileStore in the browser: a game's files (GM 5's
// file_* functions — high scores, saves) kept in the page's localStorage,
// under keys of its own per game, so they survive a reload. Text, as GM 5's
// file functions write. Apart from the ebiten backend so node can test it.
package webfiles

import (
	"fmt"
	"syscall/js"

	"github.com/LaPingvino/gameviber/runner"
)

type webFiles struct {
	store  js.Value // localStorage
	prefix string
}

// New is the store for one game (by its GameID); nil when the page
// has no localStorage (a private window may refuse it).
func New(gameID int32) runner.FileStore {
	ls := js.Global().Get("localStorage")
	if !ls.Truthy() {
		return nil
	}
	return webFiles{store: ls, prefix: fmt.Sprintf("gameviber/%d/files/", gameID)}
}

func (w webFiles) Read(name string) ([]byte, error) {
	v := w.store.Call("getItem", w.prefix+name)
	if v.IsNull() || v.IsUndefined() {
		return nil, fmt.Errorf("no file %s", name)
	}
	return []byte(v.String()), nil
}

func (w webFiles) Write(name string, data []byte) (err error) {
	defer func() { // setItem throws when the storage is full
		if r := recover(); r != nil {
			err = fmt.Errorf("saving %s: %v", name, r)
		}
	}()
	w.store.Call("setItem", w.prefix+name, string(data))
	return nil
}

func (w webFiles) Exists(name string) bool {
	v := w.store.Call("getItem", w.prefix+name)
	return !v.IsNull() && !v.IsUndefined()
}

func (w webFiles) Delete(name string) error {
	w.store.Call("removeItem", w.prefix+name)
	return nil
}
