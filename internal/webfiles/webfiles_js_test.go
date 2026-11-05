//go:build js

package webfiles

import (
	"syscall/js"
	"testing"
)

// fakeStorage puts a minimal localStorage on the global object when the
// JavaScript host has none (node without --localstorage-file).
func fakeStorage() {
	if js.Global().Get("localStorage").Truthy() {
		return
	}
	m := map[string]string{}
	o := js.Global().Get("Object").New()
	o.Set("getItem", js.FuncOf(func(_ js.Value, a []js.Value) any {
		if v, ok := m[a[0].String()]; ok {
			return v
		}
		return js.Null()
	}))
	o.Set("setItem", js.FuncOf(func(_ js.Value, a []js.Value) any { m[a[0].String()] = a[1].String(); return nil }))
	o.Set("removeItem", js.FuncOf(func(_ js.Value, a []js.Value) any { delete(m, a[0].String()); return nil }))
	js.Global().Set("localStorage", o)
}

// The browser's file store: a file written is read back, exists, is gone
// after delete; two games don't see each other's files.
func TestWebFiles(t *testing.T) {
	fakeStorage()
	a, b := New(1), New(2)
	if a == nil {
		t.Fatal("no store")
	}
	if err := a.Write("hiscore.txt", []byte("Joop\r\n42")); err != nil {
		t.Fatal(err)
	}
	got, err := a.Read("hiscore.txt")
	if err != nil || string(got) != "Joop\r\n42" || !a.Exists("hiscore.txt") || b.Exists("hiscore.txt") {
		t.Fatalf("read %q %v; exists %v / other game %v", got, err, a.Exists("hiscore.txt"), b.Exists("hiscore.txt"))
	}
	a.Delete("hiscore.txt")
	if a.Exists("hiscore.txt") {
		t.Error("still there after delete")
	}
	if _, err := a.Read("missing.txt"); err == nil {
		t.Error("a missing file read without error")
	}
}
