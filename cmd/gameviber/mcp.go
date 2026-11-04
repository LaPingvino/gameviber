package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// gameviber mcp DIR: the project's commands as an MCP server on stdin /
// stdout (DESIGN's "an MCP server over the same commands"; docs/EDITOR.md,
// the assistant's step 3), for assistants that edit the files themselves
// (Claude Code, other MCP clients): check, play (its state, and the frame
// as an image when asked), reference. JSON-RPC 2.0, one message per line.
func cmdMCP(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: gameviber mcp [DIR]   (an MCP server on stdin / stdout for the project in DIR; default: the current folder — the one the agent was started in)")
	}
	where := "."
	if len(args) == 1 {
		where = args[0]
	}
	dir, err := filepath.Abs(where)
	if err != nil {
		return err
	}
	return serveMCP(dir, os.Stdin, os.Stdout)
}

const mcpProtocol = "2025-06-18"

type mcpMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

var mcpTools = []map[string]any{
	{"name": "check", "description": "Check the GameViber project: every .gml file's syntax and calls (with file:line and hints) and the names in the TOML files. Run it after every change.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"variant": map[string]any{"type": "string", "description": "a build variant: NAME.toml merged over game.toml (e.g. demo)"}}}},
	{"name": "play", "description": "Run the game headless for some steps under a key script and report the state after: rooms passed, room, score, lives, instances by object, runtime errors, the game's show_debug_message lines, and an expectation's pass / fail. shot: also the last frame as an image.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
			"room":    map[string]any{"type": "string", "description": "the room to start in (default: the first)"},
			"steps":   map[string]any{"type": "integer", "description": "steps to run (default 300)"},
			"keys":    map[string]any{"type": "string", "description": "keys held per span, e.g. 'right:40,right+up:25,:10,left:30' (names: left right up down space enter; empty = none; the number: steps)"},
			"shot":    map[string]any{"type": "boolean", "description": "also return the last frame as a PNG image"},
			"variant": map[string]any{"type": "string", "description": "a build variant: NAME.toml merged over game.toml (e.g. demo)"},
			"expect":  map[string]any{"type": "string", "description": "a GML expression checked after the run, e.g. 'score >= 10' or 'instance_number(coin) == 0': pass / fail in the result"},
		}}},
	{"name": "outline", "description": "The project's shape without its code: objects with their events (and each event's length), scripts, rooms, sprites, sounds. Start here.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	{"name": "read", "description": "A file's lines, numbered: PATH, PATH:FROM-TO (e.g. objects/player/step.gml:10-30), or OBJECT.EVENT (e.g. player.step).",
		"inputSchema": map[string]any{"type": "object", "required": []string{"ref"}, "properties": map[string]any{
			"ref": map[string]any{"type": "string", "description": "PATH, PATH:FROM-TO or OBJECT.EVENT"}}}},
	{"name": "find", "description": "Every line of the project's text files holding the text (case-insensitive), as path:line: text.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"text"}, "properties": map[string]any{
			"text": map[string]any{"type": "string"}}}},
	{"name": "edit", "description": "Change a file with search / replace blocks — each <<<<<<< SEARCH, the exact old lines, =======, the new lines, >>>>>>> REPLACE — applied only when the old lines are found exactly once (else the nearest lines come back to retry). An empty SEARCH writes a new file. The check report follows.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"path", "blocks"}, "properties": map[string]any{
			"path":   map[string]any{"type": "string"},
			"blocks": map[string]any{"type": "string", "description": "one or more search / replace blocks"}}}},
	{"name": "reference", "description": "What GameViber understands: the GML language, the functions and actions, the event files.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
}

// serveMCP answers MCP requests from in until it ends.
func serveMCP(dir string, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(out)
	reply := func(id json.RawMessage, result any) error {
		return enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	fail := func(id json.RawMessage, code int, msg string) error {
		return enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg}})
	}
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m mcpMessage
		if err := json.Unmarshal(line, &m); err != nil {
			if err := fail(json.RawMessage("null"), -32700, "parse error: "+err.Error()); err != nil {
				return err
			}
			continue
		}
		if len(m.ID) == 0 { // a notification (initialized, cancelled …): no answer
			continue
		}
		var err error
		switch m.Method {
		case "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			json.Unmarshal(m.Params, &p)
			version := mcpProtocol
			if p.ProtocolVersion != "" {
				version = p.ProtocolVersion
			}
			err = reply(m.ID, map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo":   map[string]any{"name": "gameviber", "version": "0.1"},
				"instructions": "GameViber project " + filepath.Base(dir) + ": edit its files, then call check; play runs it headless. AGENTS.md in the project explains the layout."})
		case "ping":
			err = reply(m.ID, map[string]any{})
		case "tools/list":
			err = reply(m.ID, map[string]any{"tools": mcpTools})
		case "tools/call":
			var p struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if e := json.Unmarshal(m.Params, &p); e != nil {
				err = fail(m.ID, -32602, e.Error())
				break
			}
			content, isErr, known := mcpCall(dir, p.Name, p.Arguments)
			if !known {
				err = fail(m.ID, -32602, "no tool "+p.Name)
				break
			}
			err = reply(m.ID, map[string]any{"content": content, "isError": isErr})
		default:
			err = fail(m.ID, -32601, "method not found: "+m.Method)
		}
		if err != nil {
			return err
		}
	}
	return sc.Err()
}

// mcpCall runs a tool: its content (text, an image), whether it failed, and
// whether the tool exists.
func mcpCall(dir, name string, args json.RawMessage) ([]map[string]any, bool, bool) {
	text := func(s string) []map[string]any { return []map[string]any{{"type": "text", "text": s}} }
	if name == "check" || name == "play" { // set up user-wide, the server starts in any folder
		if _, err := os.Stat(filepath.Join(dir, "game.toml")); err != nil {
			return text(dir + " is not a GameViber project (no game.toml) — start the agent in a project folder, or make one with `gameviber new`"), true, true
		}
	}
	switch name {
	case "check":
		var a struct {
			Variant string `json:"variant"`
		}
		if len(args) > 0 {
			json.Unmarshal(args, &a)
		}
		var b strings.Builder
		problems, err := checkProjectVariant(dir, a.Variant, &b)
		if err != nil {
			return text(err.Error()), true, true
		}
		return text(b.String()), problems > 0, true
	case "outline":
		return text(Outline(dir)), false, true
	case "read":
		var a struct {
			Ref string `json:"ref"`
		}
		json.Unmarshal(args, &a)
		out, err := readRef(dir, a.Ref)
		if err != nil {
			return text(err.Error()), true, true
		}
		return text(out), false, true
	case "find":
		var a struct {
			Text string `json:"text"`
		}
		json.Unmarshal(args, &a)
		return text(Find(dir, a.Text, 60)), false, true
	case "edit":
		var a struct{ Path, Blocks string }
		json.Unmarshal(args, &a)
		rep, err := editFile(dir, a.Path, a.Blocks)
		if err != nil {
			return text(err.Error()), true, true
		}
		r, _ := rep["report"].(string)
		return text("edited " + a.Path + "\n" + r), rep["problems"] != 0, true
	case "reference":
		var b strings.Builder
		writeReference(&b)
		return text(b.String()), false, true
	case "play":
		a := struct {
			Room    string `json:"room"`
			Steps   int    `json:"steps"`
			Keys    string `json:"keys"`
			Shot    bool   `json:"shot"`
			Variant string `json:"variant"`
			Expect  string `json:"expect"`
		}{Steps: 300}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return text(err.Error()), true, true
			}
		}
		var png bytes.Buffer
		var shot io.Writer
		if a.Shot {
			shot = &png
		}
		state, err := playProjectOpts(dir, playOpts{Variant: a.Variant, Room: a.Room, Steps: a.Steps, Keys: a.Keys, Shot: shot, Expect: a.Expect})
		if err != nil {
			return text(err.Error()), true, true
		}
		js, _ := json.Marshal(state)
		out := text(string(js))
		if a.Shot && png.Len() > 0 {
			out = append(out, map[string]any{"type": "image", "mimeType": "image/png", "data": base64.StdEncoding.EncodeToString(png.Bytes())})
		}
		errs, _ := state["errors"].([]string)
		return out, len(errs) > 0, true
	}
	return nil, false, false
}
