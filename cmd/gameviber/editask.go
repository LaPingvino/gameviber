package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// "Ask" (LOOP item 5c): one question to a coding agent in its
// non-interactive mode, in the project folder, the answer streamed into the
// editor (Server-Sent Events, one line each). The agent's own login — no key.
// It may change files only when the user ticks "may change files".

// askArgs is the command line for a one-shot question to a preset agent;
// edits lets it change files without asking. nil for an unknown agent.
func askArgs(agent, prompt string, edits bool) []string {
	switch agent {
	case "claude":
		a := []string{"claude", "-p", prompt}
		if edits {
			a = append(a, "--permission-mode", "acceptEdits")
		}
		return a
	case "opencode":
		return []string{"opencode", "run", prompt} // OpenCode's run mode applies its own permission settings
	case "gemini":
		a := []string{"gemini", "-p", prompt}
		if edits {
			a = append(a, "--approval-mode", "auto_edit")
		}
		return a
	case "codex":
		a := []string{"codex", "exec", prompt}
		if edits {
			a = append(a, "--full-auto")
		}
		return a
	case "aider":
		a := []string{"aider", "--message", prompt, "--no-pretty"}
		if edits {
			a = append(a, "--yes-always")
		} else {
			a = append(a, "--dry-run")
		}
		return a
	}
	return nil
}

type editAsk struct {
	dir string
	mu  sync.Mutex
	cmd *exec.Cmd
}

func (a *editAsk) handle(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ask", a.ask)
	mux.HandleFunc("POST /api/ask/stop", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		if a.cmd != nil && a.cmd.Process != nil {
			a.cmd.Process.Kill()
		}
		a.mu.Unlock()
	})
}

// ask runs the question (JSON {agent, command, prompt, edits, file}) and
// streams the agent's output lines, then "done" with the exit status.
func (a *editAsk) ask(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Agent, Command, Prompt, File string
		Edits                        bool
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q); err != nil || strings.TrimSpace(q.Prompt) == "" {
		http.Error(w, "a question is needed", http.StatusBadRequest)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming here", http.StatusInternalServerError)
		return
	}
	prompt := q.Prompt
	if q.File != "" {
		prompt = "(GameViber project; the file open in the editor is " + q.File + ")\n\n" + prompt
	}
	var cmd *exec.Cmd
	if strings.TrimSpace(q.Command) != "" { // the user's own command: the question in $GAMEVIBER_PROMPT
		cmd = exec.Command("/bin/sh", "-c", q.Command)
		cmd.Env = append(os.Environ(), "GAMEVIBER_PROMPT="+prompt)
	} else if args := askArgs(q.Agent, prompt, q.Edits); args != nil {
		writeAgentConfig(a.dir, q.Agent) // its GameViber tools, as for the terminal
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		http.Error(w, "no such agent: "+q.Agent, http.StatusBadRequest)
		return
	}
	cmd.Dir = a.dir
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "PWD="+absDir(a.dir)) // not the editor's own folder
	out, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cmd.Stderr = cmd.Stdout // one stream, in order
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	a.mu.Lock()
	if a.cmd != nil && a.cmd.Process != nil {
		a.cmd.Process.Kill() // one question at a time
	}
	a.cmd = cmd
	a.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	go func() { <-r.Context().Done(); cmd.Process.Kill() }() // the page went away
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		b, _ := json.Marshal(sc.Text())
		fmt.Fprintf(w, "data: %s\n\n", b)
		fl.Flush()
	}
	status := "ok"
	if err := cmd.Wait(); err != nil {
		status = err.Error()
	}
	b, _ := json.Marshal(status)
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", b)
	fl.Flush()
}
