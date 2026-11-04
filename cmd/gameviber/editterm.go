package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// The editor's terminal panel (LOOP item 5a): one coding agent — Claude Code,
// OpenCode, Aider, Codex CLI, Gemini CLI or any command — running in the
// project folder on a pseudo-terminal, with the agent's own login (no key
// here). Output goes to the page as Server-Sent Events (base64 chunks), input
// comes back as POSTs — the standard library only.

// agentPresets are the terminal's choices: name → command line.
var agentPresets = []struct{ Name, Command string }{
	{"Claude Code", "claude"},
	{"OpenCode", "opencode"},
	{"Aider", "aider"},
	{"Codex CLI", "codex"},
	{"Gemini CLI", "gemini"},
	{"Shell", ""}, // the user's $SHELL
}

type termSession struct {
	mu     sync.Mutex
	ptm    *os.File
	cmd    *exec.Cmd
	buf    []byte // all output so far (the last 256 KB), for a panel opened late
	subs   map[chan []byte]bool
	exited bool
}

type editTerm struct {
	dir string
	mu  sync.Mutex
	s   *termSession
}

func (t *editTerm) handle(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/term/presets", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"presets": agentPresets, "supported": ptySupported})
	})
	mux.HandleFunc("POST /api/term/start", t.start)
	mux.HandleFunc("GET /api/term/out", t.out)
	mux.HandleFunc("POST /api/term/in", func(w http.ResponseWriter, r *http.Request) {
		s := t.session()
		if s == nil {
			http.Error(w, "no terminal running", http.StatusConflict)
			return
		}
		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		s.ptm.Write(data)
	})
	mux.HandleFunc("POST /api/term/resize", func(w http.ResponseWriter, r *http.Request) {
		if s := t.session(); s != nil {
			cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
			rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
			resizePty(s.ptm, cols, rows)
		}
	})
	mux.HandleFunc("POST /api/term/stop", func(w http.ResponseWriter, r *http.Request) {
		t.stop()
	})
}

func (t *editTerm) session() *termSession {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.s
}

func (t *editTerm) stop() {
	t.mu.Lock()
	s := t.s
	t.s = nil
	t.mu.Unlock()
	if s != nil {
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
		s.ptm.Close()
	}
}

// start runs the command (a preset's or the user's) in the project folder;
// a running one is stopped first.
func (t *editTerm) start(w http.ResponseWriter, r *http.Request) {
	line := strings.TrimSpace(r.URL.Query().Get("cmd"))
	if line == "" {
		line = os.Getenv("SHELL")
		if line == "" {
			line = "/bin/sh"
		}
	}
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	t.stop()
	cfgFile, _ := writeAgentConfig(t.dir, line)        // the agent's GameViber tools (MCP)
	cmd := exec.Command("/bin/sh", "-c", "exec "+line) // the user's own command line, as a shell would run it
	cmd.Dir = t.dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "PWD="+absDir(t.dir))
	ptm, err := startPty(cmd, cols, rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s := &termSession{ptm: ptm, cmd: cmd, subs: map[chan []byte]bool{}}
	t.mu.Lock()
	t.s = s
	t.mu.Unlock()
	go func() {
		chunk := make([]byte, 8192)
		for {
			n, err := ptm.Read(chunk)
			if n > 0 {
				b := append([]byte(nil), chunk[:n]...)
				s.mu.Lock()
				s.buf = append(s.buf, b...)
				if len(s.buf) > 256<<10 {
					s.buf = s.buf[len(s.buf)-256<<10:]
				}
				for c := range s.subs {
					select {
					case c <- b:
					default: // a slow page loses output rather than stalling the agent
					}
				}
				s.mu.Unlock()
			}
			if err != nil {
				break
			}
		}
		cmd.Wait()
		s.mu.Lock()
		s.exited = true
		for c := range s.subs {
			close(c)
		}
		s.subs = map[chan []byte]bool{}
		s.mu.Unlock()
	}()
	note := ""
	if strings.HasSuffix(cfgFile, "opencode.json") && !inGitRepo(t.dir) {
		note = "OpenCode reads opencode.json only inside a git repository — run git init in this folder to give it GameViber's tools"
	}
	writeJSON(w, map[string]any{"started": line, "mcpConfig": cfgFile, "note": note})
}

// out streams the terminal's output as Server-Sent Events: what came so far,
// then each new chunk (base64), then "exit".
func (t *editTerm) out(w http.ResponseWriter, r *http.Request) {
	s := t.session()
	if s == nil {
		http.Error(w, "no terminal running", http.StatusConflict)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming here", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	c := make(chan []byte, 256)
	s.mu.Lock()
	past, exited := append([]byte(nil), s.buf...), s.exited
	if !exited {
		s.subs[c] = true
	}
	s.mu.Unlock()
	send := func(b []byte) { fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(b)); fl.Flush() }
	if len(past) > 0 {
		send(past)
	}
	if exited {
		fmt.Fprint(w, "event: exit\ndata: \n\n")
		fl.Flush()
		return
	}
	defer func() { s.mu.Lock(); delete(s.subs, c); s.mu.Unlock() }()
	for {
		select {
		case b, open := <-c:
			if !open {
				fmt.Fprint(w, "event: exit\ndata: \n\n")
				fl.Flush()
				return
			}
			send(b)
		case <-r.Context().Done():
			return
		}
	}
}
