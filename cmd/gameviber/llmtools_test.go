package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyEdits(t *testing.T) {
	src := "x = 1\ny = 2\nz = 3\n"
	edits := ParseEdits("objects/a/step.gml\n```\n<<<<<<< SEARCH\ny = 2\n=======\ny = 20\n>>>>>>> REPLACE\n```\n")
	if len(edits) != 1 || edits[0].Search != "y = 2\n" || edits[0].Replace != "y = 20\n" {
		t.Fatalf("parsed %+v", edits)
	}
	if got, err := ApplyEdits(src, edits); err != nil || got != "x = 1\ny = 20\nz = 3\n" {
		t.Errorf("applied: %q %v", got, err)
	}
	if _, err := ApplyEdits("a\na\n", []Edit{{Search: "a\n", Replace: "b\n"}}); err == nil || !strings.Contains(err.Error(), "2 times") {
		t.Errorf("double match: %v", err)
	}
	_, err := ApplyEdits(src, []Edit{{Search: "y =  2\n", Replace: "q\n"}})
	if err == nil || !strings.Contains(err.Error(), "   2  y = 2") {
		t.Errorf("a miss should name the nearest lines: %v", err)
	}
	if got, err := ApplyEdits("", []Edit{{Search: "", Replace: "new\n"}}); err != nil || got != "new\n" {
		t.Errorf("new file: %q %v", got, err)
	}
	if _, err := ApplyEdits("old\n", []Edit{{Search: "", Replace: "new\n"}}); err == nil {
		t.Error("an empty SEARCH over an existing file was accepted")
	}
	if r := ReadRange("a\nb\nc\nd\n", 2, 3); r != "   2  b\n   3  c\n" {
		t.Errorf("range: %q", r)
	}
}

func TestLLMToolsAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	get := func(p string) (int, string) {
		r, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	if _, o := get("/api/outline"); !strings.Contains(o, "player: ") || !strings.Contains(o, "step (") || !strings.Contains(o, "rooms: level") {
		t.Errorf("outline:\n%s", o)
	}
	step, _ := os.ReadFile(filepath.Join(dir, "objects", "player", "step.gml"))
	first := strings.SplitN(string(step), "\n", 2)[0]
	if c, r := get("/api/read?ref=player.step:1-1"); c != 200 || r != "   1  "+first+"\n" {
		t.Errorf("read player.step:1-1: %d %q", c, r)
	}
	if c, _ := get("/api/read?ref=../x"); c != 404 {
		t.Errorf("read ../x: %d", c)
	}
	word := strings.Fields(first)[0]
	if _, f := get("/api/find?q=" + url.QueryEscape(word)); !strings.Contains(f, "objects/player/step.gml:1: ") {
		t.Errorf("find %q:\n%s", word, f)
	}
	post := func(path, body string) (int, string) {
		r, _ := http.Post(srv.URL+"/api/edit?path="+url.QueryEscape(path), "text/plain", strings.NewReader(body))
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	blk := "<<<<<<< SEARCH\n" + first + "\n=======\n" + first + "\nx += 0\n>>>>>>> REPLACE\n"
	if c, r := post("objects/player/step.gml", blk); c != 200 || !strings.Contains(r, `"problems":0`) {
		t.Errorf("edit: %d %s", c, r)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "objects", "player", "step.gml")); !strings.Contains(string(got), first+"\nx += 0\n") {
		t.Errorf("not written:\n%s", got)
	}
	if c, r := post("objects/player/step.gml", "<<<<<<< SEARCH\nnot there\n=======\nx\n>>>>>>> REPLACE\n"); c != http.StatusConflict || !strings.Contains(r, "nearest lines") {
		t.Errorf("a miss: %d %s", c, r)
	}
	if c, _ := post("game.exe", blk); c != http.StatusConflict {
		t.Errorf("a non-text file: %d", c)
	}
}

// TestPlayExpectDebug: a run's expectation (pass / fail / broken) and the
// game's show_debug_message lines.
func TestPlayExpectDebug(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	create := filepath.Join(dir, "objects", "player", "create.gml")
	data, _ := os.ReadFile(create)
	os.WriteFile(create, append(data, []byte("\nshow_debug_message(\"ready at \" + string(x))\n")...), 0o644)
	run := func(expect string) map[string]any {
		st, err := playProjectOpts(dir, playOpts{Steps: 20, Expect: expect})
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	st := run("instance_number(coin) == 6")
	if ex := st["expect"].(map[string]any); ex["pass"] != true {
		t.Errorf("coins: %v", ex)
	}
	if d := st["debug"].([]string); len(d) != 1 || d[0] != "ready at 64" {
		t.Errorf("debug: %q", d)
	}
	if ex := run("score >= 0")["expect"].(map[string]any); ex["pass"] != true {
		t.Errorf("score >= 0: %v", ex)
	}
	if ex := run("score > 1000")["expect"].(map[string]any); ex["pass"] != false || ex["value"] != "0" {
		t.Errorf("score > 1000: %v", ex)
	}
	if ex := run("score >")["expect"].(map[string]any); ex["error"] == nil {
		t.Errorf("a broken expression: %v", ex)
	}
}
