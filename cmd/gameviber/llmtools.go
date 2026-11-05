package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The tools an assistant works with (LOOP item 11), shared by the editor's API
// (the chat session) and the MCP server: search / replace edits, ranged reads,
// find, an outline of the project.

// Edit is one search / replace change to a file.
type Edit struct {
	Search  string `json:"search"`
	Replace string `json:"replace"`
}

// editBlockRe finds the blocks models write for search / replace edits:
//
//	<<<<<<< SEARCH
//	old lines
//	=======
//	new lines
//	>>>>>>> REPLACE
var editBlockRe = regexp.MustCompile(`(?ms)^<{5,9} ?SEARCH[^\n]*\n(.*?)^={5,9}[ \t]*\n(.*?)^>{5,9} ?REPLACE[^\n]*$`)

// ParseEdits returns the search / replace blocks in text.
func ParseEdits(text string) []Edit {
	var out []Edit
	for _, m := range editBlockRe.FindAllStringSubmatch(text, -1) {
		out = append(out, Edit{Search: m[1], Replace: m[2]})
	}
	return out
}

// ApplyEdits applies edits to text in order, each only when its search text is
// found exactly once (an empty search on an empty file writes it whole). On a
// miss the error names the edit and the file's nearest lines, so the one who
// wrote it can try again.
func ApplyEdits(text string, edits []Edit) (string, error) {
	for i, e := range edits {
		if e.Search == "" {
			if text != "" {
				return text, fmt.Errorf("edit %d: an empty SEARCH only on a new or empty file", i+1)
			}
			text = e.Replace
			continue
		}
		switch n := strings.Count(text, e.Search); {
		case n == 1:
			text = strings.Replace(text, e.Search, e.Replace, 1)
		case n > 1:
			return text, fmt.Errorf("edit %d: its SEARCH text is found %d times — include more lines so it is found once", i+1, n)
		default:
			return text, fmt.Errorf("edit %d: its SEARCH text is not in the file (it must match exactly, spaces included); the nearest lines:\n%s", i+1, nearest(text, e.Search))
		}
	}
	return text, nil
}

// nearest is the part of text most like the first non-empty line of search,
// numbered, with two lines around it.
func nearest(text, search string) string {
	want := ""
	for _, l := range strings.Split(search, "\n") {
		if strings.TrimSpace(l) != "" {
			want = strings.TrimSpace(l)
			break
		}
	}
	lines := strings.Split(text, "\n")
	best, bestScore := 0, -1
	for i, l := range lines {
		if s := commonWords(strings.TrimSpace(l), want); s > bestScore {
			best, bestScore = i, s
		}
	}
	return numbered(lines, max(0, best-2), min(len(lines), best+3))
}

func commonWords(a, b string) int {
	n := 0
	for _, w := range strings.Fields(b) {
		if strings.Contains(a, w) {
			n++
		}
	}
	return n
}

// numbered is lines[from:to] with their 1-based numbers.
func numbered(lines []string, from, to int) string {
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%4d  %s\n", i+1, lines[i])
	}
	return b.String()
}

// ReadRange is a file's lines from..to (1-based, inclusive; 0 = from the start /
// to the end), numbered.
func ReadRange(text string, from, to int) string {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if from < 1 {
		from = 1
	}
	if to < 1 || to > len(lines) {
		to = len(lines)
	}
	if from > to {
		return fmt.Sprintf("(the file has %d lines)\n", len(lines))
	}
	return numbered(lines, from-1, to)
}

// textFiles are the project's files an assistant reads and edits.
func textFiles(dir string) []string {
	var out []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(p) {
		case ".gml", ".js", ".lua", ".toml", ".md", ".txt":
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// Find is every line of the project's text files holding pattern (plain text,
// case-insensitive), as path:line: text, at most limit of them.
func Find(dir, pattern string, limit int) string {
	if strings.TrimSpace(pattern) == "" {
		return "(find what?)\n"
	}
	var b strings.Builder
	n, low := 0, strings.ToLower(pattern)
	for _, f := range textFiles(dir) {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		for i, l := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(l), low) {
				if n++; n > limit {
					fmt.Fprintf(&b, "(more than %d — narrow the search)\n", limit)
					return b.String()
				}
				fmt.Fprintf(&b, "%s:%d: %s\n", f, i+1, strings.TrimSpace(l))
			}
		}
	}
	if n == 0 {
		return "(nothing found)\n"
	}
	return b.String()
}

// Outline is the project's shape without its code: objects with their
// events (and each event's length), scripts, rooms, sprites, sounds.
func Outline(dir string) string {
	var b strings.Builder
	list := func(sub string) []string {
		ents, _ := os.ReadDir(filepath.Join(dir, sub))
		var out []string
		for _, e := range ents {
			out = append(out, e.Name())
		}
		sort.Strings(out)
		return out
	}
	lines := func(p string) int {
		data, _ := os.ReadFile(p)
		return strings.Count(strings.TrimSuffix(string(data), "\n"), "\n") + 1
	}
	fmt.Fprintln(&b, "objects:")
	for _, o := range list("objects") {
		var evs []string
		for _, f := range list(filepath.Join("objects", o)) {
			ev, ok := strings.CutSuffix(f, ".gml")
			if js, isJS := strings.CutSuffix(f, ".js"); isJS {
				ev, ok = js+" [JavaScript]", true
			}
			if l, isLua := strings.CutSuffix(f, ".lua"); isLua {
				ev, ok = l+" [Lua]", true
			}
			if ok {
				n, unit := lines(filepath.Join(dir, "objects", o, f)), "lines"
				if n == 1 {
					unit = "line"
				}
				evs = append(evs, fmt.Sprintf("%s (%d %s)", ev, n, unit))
			}
		}
		fmt.Fprintf(&b, "  %s: %s\n", o, strings.Join(evs, ", "))
	}
	for _, sub := range []string{"scripts", "rooms", "sprites", "sounds", "backgrounds", "timelines"} {
		if names := list(sub); len(names) > 0 {
			fmt.Fprintf(&b, "%s: %s\n", sub, strings.Join(names, ", "))
		}
	}
	return b.String()
}

// eventPath is OBJECT.EVENT (as the outline names them) as a file path.
func eventPath(ref string) (string, bool) {
	o, ev, ok := strings.Cut(ref, ".")
	if !ok || !safeName(o) || ev == "" || strings.ContainsAny(ev, "/\\") || strings.Contains(ev, "..") {
		return "", false
	}
	return "objects/" + o + "/" + ev + ".gml", true
}

// editFile applies the search / replace blocks in blocks to the project file
// rel (an editable one; a new file starts empty); the check report after.
func editFile(dir, rel, blocks string) (map[string]any, error) {
	p, ok := projectPath(dir, rel)
	if !ok || !editable(p) {
		return nil, fmt.Errorf("%s: not an editable file of this project", rel)
	}
	edits := ParseEdits(blocks)
	if len(edits) == 0 {
		return nil, fmt.Errorf("no <<<<<<< SEARCH / ======= / >>>>>>> REPLACE blocks")
	}
	old, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	text, err := ApplyEdits(string(old), edits)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		return nil, err
	}
	return checkReport(dir), nil
}

// readRef reads PATH, PATH:FROM-TO or OBJECT.EVENT, numbered.
func readRef(dir, ref string) (string, error) {
	path, from, to := ref, 0, 0
	if p, rng, ok := strings.Cut(ref, ":"); ok {
		path = p
		if a, b, ok := strings.Cut(rng, "-"); ok {
			fmt.Sscan(a, &from)
			fmt.Sscan(b, &to)
		} else {
			fmt.Sscan(rng, &from)
			to = from
		}
	}
	if !strings.Contains(path, "/") {
		if ep, ok := eventPath(path); ok {
			path = ep
		}
	}
	p, ok := projectPath(dir, path)
	if !ok {
		return "", fmt.Errorf("%s: not a file of this project", path)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("%s: no such file (outline lists them)", path)
	}
	return ReadRange(string(data), from, to), nil
}
