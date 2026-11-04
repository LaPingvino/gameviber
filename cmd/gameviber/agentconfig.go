package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// writeAgentConfig (LOOP item 5b) gives the coding agent about to start in
// the project folder GameViber's MCP tools — `gameviber mcp DIR` (check,
// play with a screenshot, reference) — in the project config that agent
// reads: Claude Code's .mcp.json, OpenCode's opencode.json, Gemini CLI's
// .gemini/settings.json. Other entries in those files are kept; nothing
// outside the project is touched (Codex CLI's config is global: left alone;
// Aider has no MCP). Returns the file written, "" if none.
func writeAgentConfig(dir, command string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	agent := filepath.Base(strings.Fields(command + " x")[0])
	var file string
	var add func(cfg map[string]any)
	switch agent {
	case "claude":
		file = filepath.Join(abs, ".mcp.json")
		add = func(cfg map[string]any) {
			servers := child(cfg, "mcpServers")
			servers["gameviber"] = map[string]any{"type": "stdio", "command": exe, "args": []string{"mcp", abs}}
		}
	case "opencode":
		file = filepath.Join(abs, "opencode.json")
		add = func(cfg map[string]any) {
			if _, ok := cfg["$schema"]; !ok {
				cfg["$schema"] = "https://opencode.ai/config.json"
			}
			servers := child(cfg, "mcp")
			servers["gameviber"] = map[string]any{"type": "local", "command": []string{exe, "mcp", abs}, "enabled": true}
		}
	case "gemini":
		file = filepath.Join(abs, ".gemini", "settings.json")
		add = func(cfg map[string]any) {
			servers := child(cfg, "mcpServers")
			servers["gameviber"] = map[string]any{"command": exe, "args": []string{"mcp", abs}}
		}
	default:
		return "", nil
	}
	cfg := map[string]any{}
	if data, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return "", nil // someone else's file we can't read: leave it alone
		}
	}
	add(cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	return file, os.WriteFile(file, append(data, '\n'), 0o644)
}

// child is cfg[key] as an object, made if missing.
func child(cfg map[string]any, key string) map[string]any {
	if m, ok := cfg[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	cfg[key] = m
	return m
}

// inGitRepo: dir is inside a git repository (a .git above or at it).
func inGitRepo(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

// gitInit makes a new project a git repository (DECIDED best effort,
// 2026-11-04: GameViber projects are plain text meant for git, and OpenCode
// reads a project's opencode.json only inside one) — when git is installed
// and the folder is not inside a repository already. Quiet when it can't.
func gitInit(dir string) {
	if inGitRepo(dir) {
		return
	}
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	cmd := exec.Command("git", "init", "-q", dir)
	cmd.Run()
}

// agentConfigFiles are the agents' project configs writeAgentConfig writes:
// they hold this machine's paths, so a web build never ships them.
var agentConfigFiles = map[string]bool{".mcp.json": true, "opencode.json": true}

// absDir is dir made absolute (as given when that fails).
func absDir(dir string) string {
	if a, err := filepath.Abs(dir); err == nil {
		return a
	}
	return dir
}
