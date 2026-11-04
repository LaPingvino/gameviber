package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gameviber setup-agents [AGENT …]: GameViber's MCP tools (`gameviber mcp`,
// which serves the folder the agent was started in) registered user-wide
// for the coding agents installed here — through each agent's own command
// where it has one (agy mcp add, claude mcp add --scope user, gemini mcp add
// --scope user), OpenCode's user config file otherwise. Run by the user, on
// purpose: it changes those agents' own configuration (the editor's per
// project files stay as they are). With no AGENT: every one found.
func cmdSetupAgents(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	want := map[string]bool{}
	for _, a := range args {
		want[a] = true
	}
	all := len(want) == 0
	did := 0
	run := func(name string, argv ...string) {
		if !all && !want[name] {
			return
		}
		if _, err := exec.LookPath(argv[0]); err != nil {
			if !all {
				fmt.Printf("%s: not installed here\n", name)
			}
			return
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		out, err := cmd.CombinedOutput()
		msg := strings.TrimSpace(string(out))
		if err != nil {
			fmt.Printf("%s: failed (%v) %s\n", name, err, msg)
			return
		}
		fmt.Printf("%s: GameViber's tools added (%s)\n", name, strings.Join(argv, " "))
		did++
	}
	run("agy", "agy", "mcp", "add", "gameviber", exe, "mcp")
	run("claude", "claude", "mcp", "add", "--scope", "user", "gameviber", "--", exe, "mcp")
	run("gemini", "gemini", "mcp", "add", "--scope", "user", "gameviber", exe, "mcp")
	if all || want["opencode"] {
		if _, err := exec.LookPath("opencode"); err == nil || want["opencode"] {
			if file, err := setupOpenCodeUser(exe); err != nil {
				fmt.Printf("opencode: failed (%v)\n", err)
			} else {
				fmt.Printf("opencode: GameViber's tools added (%s)\n", file)
				did++
			}
		}
	}
	if did == 0 {
		fmt.Println("no coding agent set up (installed here: none of agy, claude, gemini, opencode — or name one: gameviber setup-agents claude)")
	} else {
		fmt.Println("the agents now have GameViber's tools (check, play, reference) in any GameViber project folder they start in")
	}
	return nil
}

// setupOpenCodeUser adds the gameviber server to OpenCode's user config
// ($XDG_CONFIG_HOME/opencode/opencode.json), keeping everything else.
func setupOpenCodeUser(exe string) (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	file := filepath.Join(base, "opencode", "opencode.json")
	cfg := map[string]any{}
	if data, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return "", fmt.Errorf("%s is not plain JSON — add it by hand: \"mcp\": {\"gameviber\": {\"type\": \"local\", \"command\": [%q, \"mcp\"]}}", file, exe)
		}
	}
	if _, ok := cfg["$schema"]; !ok {
		cfg["$schema"] = "https://opencode.ai/config.json"
	}
	child(cfg, "mcp")["gameviber"] = map[string]any{"type": "local", "command": []string{exe, "mcp"}, "enabled": true}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	return file, os.WriteFile(file, append(data, '\n'), 0o644)
}
