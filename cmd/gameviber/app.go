//go:build !js

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// gameviber app [DIR] — GameViber as a desktop program (Joop 2026-11-19: "a gui that opens via a .desktop … I want
// to play around with it more locally"): the local editor (gameviber edit) in a window of its own, with a start
// page for the projects — the recent ones, a new one from a template, any folder. One server per user: a second
// start (another .desktop click, a folder opened from the file manager) opens a window on the running one. Each
// project lives under /p/<id>/, so two windows on two projects never write into each other's.
//
// The window is a Chromium-family browser in app mode (its own profile, no tabs or address bar); the server stops
// when that window's browser ends. Without one, the default browser opens the page and the server stops once no
// page has called in for a while (each page pings).

const appIdle = 90 * time.Second // without an app window: stop after this long with no page open

func cmdApp(args []string) error {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	noWindow := fs.Bool("no-window", false, "serve only: print the address (for tests, or a browser of your own)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: gameviber app [-no-window] [DIR]")
	}
	dir := ""
	if fs.NArg() == 1 {
		d, err := filepath.Abs(fs.Arg(0))
		if err != nil {
			return err
		}
		dir = d
	}
	appEnv()

	// A running GameViber: a window on it.
	if inst, ok := runningApp(); ok {
		url := inst.base() + "/start/"
		if dir != "" {
			u, err := inst.open(dir)
			if err != nil {
				return err
			}
			url = inst.base() + u
		}
		if *noWindow {
			fmt.Println(url)
			return nil
		}
		return openWindow(url, false)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s := newAppServer(newToken())
	inst := appInstance{Port: ln.Addr().(*net.TCPAddr).Port, Token: s.token, PID: os.Getpid()}
	if err := inst.write(); err != nil {
		fmt.Fprintln(os.Stderr, "gameviber: could not note the running app (a second start will start another):", err)
	}
	defer removeAppInstance(inst)
	go http.Serve(ln, s)

	url := inst.base() + "/start/"
	if dir != "" {
		u, err := s.openProject(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gameviber:", err)
		} else {
			url = inst.base() + u
		}
	}
	if *noWindow {
		fmt.Printf("GameViber at %s (Ctrl+C to stop)\n", url)
		select {}
	}
	opened := time.Now()
	if err := openWindow(url, true); err != nil {
		fmt.Fprintln(os.Stderr, "gameviber:", err)
	} else if time.Since(opened) > 5*time.Second {
		return nil // the app's browser ended: it held every window (a second start's window joins it)
	}
	// No app window (the default browser has the page), or the browser handed the window to one of its own
	// already running and returned at once, or a page still calls in: serve while pages call in.
	for time.Since(s.lastSeen()) < appIdle {
		time.Sleep(5 * time.Second)
	}
	return nil
}

// appEnv prepares the environment a desktop launcher doesn't give: the user's own program folders on the PATH (a
// .desktop start has the session's PATH, often without ~/.local/bin, where coding agents such as claude live), and
// the packaged GameViber source for exports.
func appEnv() {
	home, _ := os.UserHomeDir()
	path := filepath.SplitList(os.Getenv("PATH"))
	for _, d := range []string{"/usr/local/bin", filepath.Join(home, ".local", "bin"), filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".npm-global", "bin"), filepath.Join(home, ".bun", "bin"), filepath.Join(home, ".cargo", "bin")} {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() && !slices.Contains(path, d) {
			path = append(path, d)
		}
	}
	os.Setenv("PATH", strings.Join(path, string(os.PathListSeparator)))
	if os.Getenv("GAMEVIBER_SRC") == "" {
		if _, err := os.Stat(filepath.Join(packagedSource, "go.mod")); err == nil {
			os.Setenv("GAMEVIBER_SRC", packagedSource)
		}
	}
}

// packagedSource is where the Arch package (packaging/arch/PKGBUILD) puts GameViber's source, for exports.
const packagedSource = "/usr/share/gameviber/src"

// --- the window ---

// appBrowsers are the browsers with an app mode (a window without tabs or address bar), in order of preference;
// GAMEVIBER_BROWSER names another.
var appBrowsers = []string{"chromium", "google-chrome-stable", "google-chrome", "brave", "brave-browser",
	"microsoft-edge-stable", "vivaldi-stable", "vivaldi"}

// openWindow opens url in an app window; wait: until that browser ends. Without an app-mode browser, the default
// browser opens it (and wait returns at once).
func openWindow(url string, wait bool) error {
	names := appBrowsers
	if b := os.Getenv("GAMEVIBER_BROWSER"); b != "" {
		names = []string{b}
	}
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		profile := filepath.Join(dataHome(), "gameviber", "window")
		cmd := exec.Command(path, "--app="+url, "--user-data-dir="+profile, "--class=GameViber",
			"--no-first-run", "--no-default-browser-check")
		if err := cmd.Start(); err != nil {
			continue
		}
		if !wait {
			go cmd.Wait()
			return nil
		}
		return cmd.Wait()
	}
	for _, opener := range []string{"xdg-open", "open"} {
		if path, err := exec.LookPath(opener); err == nil {
			return exec.Command(path, url).Start()
		}
	}
	return fmt.Errorf("no browser found: open %s yourself", url)
}

func dataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func stateHome() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

func configHome() string {
	if d, err := os.UserConfigDir(); err == nil {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// --- the running app, found by a second start ---

type appInstance struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

func appInstancePath() string { return filepath.Join(stateHome(), "gameviber", "app.json") }

func (i appInstance) base() string { return fmt.Sprintf("http://127.0.0.1:%d", i.Port) }

func (i appInstance) write() error {
	if err := os.MkdirAll(filepath.Dir(appInstancePath()), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(i)
	return os.WriteFile(appInstancePath(), b, 0o600)
}

// removeAppInstance removes the note, when it is still this one's.
func removeAppInstance(i appInstance) {
	if cur, err := readAppInstance(); err == nil && cur == i {
		os.Remove(appInstancePath())
	}
}

func readAppInstance() (appInstance, error) {
	var i appInstance
	b, err := os.ReadFile(appInstancePath())
	if err != nil {
		return i, err
	}
	return i, json.Unmarshal(b, &i)
}

// runningApp is the app already running for this user, when it answers.
func runningApp() (appInstance, bool) {
	i, err := readAppInstance()
	if err != nil || i.PID <= 0 || syscall.Kill(i.PID, 0) != nil {
		return i, false
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(i.base() + "/start/ping")
	if err != nil {
		return i, false
	}
	defer resp.Body.Close()
	var b bytes.Buffer
	b.ReadFrom(resp.Body)
	return i, resp.StatusCode == 200 && strings.TrimSpace(b.String()) == "gameviber"
}

// open asks the running app to open dir; its path there.
func (i appInstance) open(dir string) (string, error) {
	body, _ := json.Marshal(map[string]string{"dir": dir})
	req, _ := http.NewRequest("POST", i.base()+"/start/open", bytes.NewReader(body))
	req.Header.Set("X-GameViber-Token", i.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct{ URL, Error string }
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Error != "" {
		return "", errors.New(out.Error)
	}
	return out.URL, nil
}

// --- the server: the start page and the projects ---

type appServer struct {
	token    string
	seen     atomic.Int64 // the last request's time (UnixNano)
	mu       sync.Mutex
	projects map[string]http.Handler // by id: the editor of that folder
	dirs     map[string]string
}

func newAppServer(token string) *appServer {
	s := &appServer{token: token, projects: map[string]http.Handler{}, dirs: map[string]string{}}
	s.seen.Store(time.Now().UnixNano())
	return s
}

func (s *appServer) lastSeen() time.Time { return time.Unix(0, s.seen.Load()) }

// appPing keeps the app alive while a page is open, without an app window (cmdApp's idle wait).
const appPing = `<script>setInterval(() => fetch("/start/ping").catch(() => {}), 20000)</script>`

func (s *appServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host != "127.0.0.1" && host != "localhost" {
		http.Error(w, "GameViber answers this machine only", http.StatusForbidden)
		return
	}
	s.seen.Store(time.Now().UnixNano())
	p := r.URL.Path
	switch {
	case p == "/":
		http.Redirect(w, r, "/start/", http.StatusFound)
	case p == "/start/ping":
		fmt.Fprintln(w, "gameviber")
	case p == "/start/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(bytes.Replace(appStartPage, []byte("<head>"), []byte(`<head><meta name="gameviber-token" content="`+s.token+`">`), 1))
	case strings.HasPrefix(p, "/start/"):
		if r.Method != http.MethodPost || r.Header.Get("X-GameViber-Token") != s.token {
			http.Error(w, "missing the app's token", http.StatusForbidden)
			return
		}
		s.startAPI(w, r, strings.TrimPrefix(p, "/start/"))
	case strings.HasPrefix(p, "/p/"):
		id, _, _ := strings.Cut(strings.TrimPrefix(p, "/p/"), "/")
		s.mu.Lock()
		h := s.projects[id]
		s.mu.Unlock()
		if h == nil {
			http.Redirect(w, r, "/start/", http.StatusFound) // a project of an earlier run: choose again
			return
		}
		if p == "/p/"+id { // the page's relative URLs need the slash
			http.Redirect(w, r, p+"/", http.StatusFound)
			return
		}
		http.StripPrefix("/p/"+id, h).ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

// appEditorPage is the editor's page in the app: GameViber in its header leads back to the projects.
var appEditorPage = bytes.Replace(bytes.Replace(editPage, []byte("<header><b>GameViber</b>"),
	[]byte(`<header><a href="/start/" title="your projects: open another, or start a new one" style="color:inherit;text-decoration:none"><b>GameViber</b> ▾</a>`), 1),
	[]byte("</body>"), []byte(appPing+"</body>"), 1)

// openProject serves the project in dir (made absolute, with game.toml) and notes it as recent; its path.
func (s *appServer) openProject(dir string) (string, error) {
	dir = filepath.Clean(dir)
	if _, err := os.Stat(filepath.Join(dir, "game.toml")); err != nil {
		return "", fmt.Errorf("%s is not a GameViber project (no game.toml)", dir)
	}
	sum := sha256.Sum256([]byte(dir))
	id := hex.EncodeToString(sum[:5])
	s.mu.Lock()
	if s.projects[id] == nil {
		s.projects[id] = guardLocalPage(editHandler(dir), s.token, appEditorPage)
		s.dirs[id] = dir
	}
	s.mu.Unlock()
	addRecent(dir)
	return "/p/" + id + "/", nil
}

func (s *appServer) startAPI(w http.ResponseWriter, r *http.Request, what string) {
	var in struct{ Dir, Name, Template, Parent string }
	json.NewDecoder(r.Body).Decode(&in)
	fail := func(err error) { writeJSON(w, map[string]string{"error": err.Error()}) }
	switch what {
	case "recent":
		type entry struct {
			Dir, Name string
			Exists    bool
		}
		var out []entry
		for _, d := range readRecent() {
			_, err := os.Stat(filepath.Join(d, "game.toml"))
			out = append(out, entry{d, filepath.Base(d), err == nil})
		}
		writeJSON(w, map[string]any{"recent": out, "templates": templates, "parent": defaultProjectsDir()})
	case "open":
		u, err := s.openProject(expandHome(in.Dir))
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]string{"url": u})
	case "new":
		name := strings.TrimSpace(in.Name)
		if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			fail(errors.New("a name for the game, without slashes"))
			return
		}
		parent := expandHome(strings.TrimSpace(in.Parent))
		if parent == "" {
			parent = defaultProjectsDir()
		}
		if err := os.MkdirAll(parent, 0o755); err != nil {
			fail(err)
			return
		}
		tmpl := in.Template
		if tmpl == "" {
			tmpl = "coins"
		}
		dir := filepath.Join(parent, name)
		if err := cmdNew([]string{"-template", tmpl, dir}); err != nil {
			fail(err)
			return
		}
		u, err := s.openProject(dir)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]string{"url": u})
	case "browse":
		dir, err := pickFolder(expandHome(in.Dir))
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, map[string]string{"dir": dir})
	case "forget":
		forgetRecent(in.Dir)
		writeJSON(w, map[string]string{})
	default:
		http.NotFound(w, r)
	}
}

// pickFolder asks for a folder with the desktop's own dialog (zenity or kdialog); "" when cancelled.
func pickFolder(start string) (string, error) {
	if start == "" {
		start = defaultProjectsDir()
	}
	var cmd *exec.Cmd
	if p, err := exec.LookPath("zenity"); err == nil {
		cmd = exec.Command(p, "--file-selection", "--directory", "--title=Open a GameViber project", "--filename="+start+"/")
	} else if p, err := exec.LookPath("kdialog"); err == nil {
		cmd = exec.Command(p, "--getexistingdirectory", start, "--title", "Open a GameViber project")
	} else {
		return "", errors.New("no folder dialog here (zenity or kdialog): type the folder's path")
	}
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", nil // cancelled
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

// defaultProjectsDir is where new games go unless the start page says otherwise: ~/GameViber.
func defaultProjectsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "GameViber")
}

// cmdIcon is gameviber icon SIZE FILE.png: GameViber's icon (the editor's, editpwa.go pwaIcon) at SIZE pixels —
// for packaging (the desktop launcher's icon).
func cmdIcon(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: gameviber icon SIZE FILE.png")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 16 || n > 1024 {
		return fmt.Errorf("icon size: 16 to 1024")
	}
	f, err := os.Create(args[1])
	if err != nil {
		return err
	}
	if err := png.Encode(f, pwaIcon(n)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// --- recent projects (~/.config/gameviber/recent.json, newest first) ---

const maxRecent = 15

func recentPath() string { return filepath.Join(configHome(), "gameviber", "recent.json") }

func readRecent() []string {
	var list []string
	if b, err := os.ReadFile(recentPath()); err == nil {
		json.Unmarshal(b, &list)
	}
	return list
}

func writeRecent(list []string) {
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	if err := os.MkdirAll(filepath.Dir(recentPath()), 0o755); err != nil {
		return
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	os.WriteFile(recentPath(), b, 0o644)
}

func addRecent(dir string) {
	list := slices.DeleteFunc(readRecent(), func(d string) bool { return d == dir })
	writeRecent(append([]string{dir}, list...))
}

func forgetRecent(dir string) {
	writeRecent(slices.DeleteFunc(readRecent(), func(d string) bool { return d == dir }))
}

var appStartPage = []byte(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>GameViber</title>
<style>
:root{--bg:#f5f1e8;--panel:#fffdf8;--text:#2a2118;--muted:#7a6c5a;--line:#e2d8c6;--accent:#c2541b;--field:#fff;--bad:#b3261e}
@media (prefers-color-scheme: dark){:root{--bg:#1b1713;--panel:#241f1a;--text:#efe6d8;--muted:#a8977f;--line:#3a3229;--accent:#f08a4b;--field:#1b1713;--bad:#f2867d}}
html,body{margin:0;background:var(--bg);color:var(--text);font:15px/1.45 system-ui,sans-serif}
main{max-width:760px;margin:0 auto;padding:32px 20px}
h1{margin:0 0 4px;font-size:28px}h1 span{color:var(--accent)}
p.sub{margin:0 0 28px;color:var(--muted)}
section{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:18px 20px;margin-bottom:18px}
h2{margin:0 0 12px;font-size:16px}
ul{list-style:none;margin:0;padding:0}
li{display:flex;align-items:center;gap:10px;padding:8px 0;border-top:1px solid var(--line)}
li:first-child{border-top:0}
li .n{font-weight:600}li .d{color:var(--muted);font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
li.gone .n{color:var(--muted);text-decoration:line-through}
button,select,input{font:inherit;color:var(--text);background:var(--field);border:1px solid var(--line);border-radius:6px;padding:7px 12px}
button{cursor:pointer}button.go{background:var(--accent);border-color:var(--accent);color:#fff;font-weight:600}
button.x{background:transparent;color:var(--muted);border:0;padding:4px 8px}
.row{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:8px}
.row input{flex:1;min-width:200px}
#msg{color:var(--bad);min-height:1.4em;margin-top:8px}
.empty{color:var(--muted)}
</style></head>
<body><main>
<h1>Game<span>Viber</span></h1>
<p class="sub">Make 2D games, GameMaker-style. Open the editor's Terminal to work with Claude Code or another coding agent in the project.</p>
<section><h2>Recent projects</h2><ul id="recent"><li class="empty">none yet</li></ul></section>
<section><h2>New game</h2>
<div class="row"><input id="name" placeholder="name, e.g. space-rocks"><select id="tmpl"></select><button class="go" id="create">Create</button></div>
<div class="row" style="color:var(--muted);font-size:13px">in <input id="parent" style="font-size:13px;padding:4px 8px"></div></section>
<section><h2>Open a project folder</h2>
<div class="row"><input id="dir" placeholder="a folder with game.toml"><button id="browse">Browse…</button><button class="go" id="open">Open</button></div></section>
<div id="msg"></div>
</main>
<script>
const token = document.querySelector('meta[name="gameviber-token"]').content;
const $ = (id) => document.getElementById(id);
async function api(what, body) {
  const r = await fetch("/start/" + what, { method: "POST", headers: { "X-GameViber-Token": token, "Content-Type": "application/json" }, body: JSON.stringify(body || {}) });
  const j = await r.json();
  if (j.error) { $("msg").textContent = j.error; throw new Error(j.error); }
  $("msg").textContent = ""; return j;
}
async function open(dir) { const j = await api("open", { dir }); location.href = j.url; }
async function load() {
  const j = await api("recent");
  if (!$("tmpl").options.length) for (const t of j.templates) $("tmpl").append(new Option(t, t));
  if (!$("parent").value) $("parent").value = j.parent;
  const ul = $("recent"); ul.innerHTML = "";
  if (!j.recent || !j.recent.length) { ul.innerHTML = '<li class="empty">none yet: make a new game below</li>'; return; }
  for (const e of j.recent) {
    const li = document.createElement("li"); if (!e.Exists) li.className = "gone";
    li.innerHTML = '<span class="n"></span><span class="d"></span>';
    li.querySelector(".n").textContent = e.Name; li.querySelector(".d").textContent = e.Dir;
    if (e.Exists) { const b = Object.assign(document.createElement("button"), { textContent: "Open", className: "go" }); b.onclick = () => open(e.Dir).catch(() => {}); li.append(b); }
    const x = Object.assign(document.createElement("button"), { textContent: "✕", className: "x", title: "forget it here (the folder stays)" });
    x.onclick = async () => { await api("forget", { dir: e.Dir }); load(); }; li.append(x);
    ul.append(li);
  }
}
$("create").onclick = async () => { try { const j = await api("new", { name: $("name").value, template: $("tmpl").value, parent: $("parent").value }); location.href = j.url; } catch (e) {} };
$("name").onkeydown = (e) => { if (e.key === "Enter") $("create").click(); };
$("open").onclick = () => open($("dir").value.trim()).catch(() => {});
$("dir").onkeydown = (e) => { if (e.key === "Enter") $("open").click(); };
$("browse").onclick = async () => { try { const j = await api("browse", { dir: $("dir").value.trim() }); if (j.dir) { $("dir").value = j.dir; open(j.dir).catch(() => {}); } } catch (e) {} };
load().catch(() => {});
</script>
` + appPing + `
</body></html>
`)
