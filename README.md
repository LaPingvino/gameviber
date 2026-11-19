# GameViber

A GameMaker-style game engine and toolset in Go: runs GML, opens old GameMaker games and project files, builds
for the desktop and the browser (WebAssembly), and is made to be driven by people and LLM agents alike.

Design approved; the GML front end is being built. Progress: https://lapingvino.github.io/gameviber/ — the plan is in LOOP.md.

## As a desktop program

`gameviber app` opens GameViber in a window of its own: your recent projects, a new game from a template, any
project folder, and the editor with its Terminal, where Claude Code (or OpenCode, Aider, Codex, Gemini …) works in
the project with GameViber's tools. On Arch Linux, `packaging/arch/PKGBUILD` installs it with a desktop launcher:

```
cd packaging/arch && makepkg -si      # gameviber, gameviber-run, the launcher and icon; the source for exports
```

The window is Chromium's (or Chrome's, Brave's …) app mode; without one the default browser opens it.

## Trying it

```
go run ./cmd/gameviber new mygame                   # a small starter game to build on
go run ./cmd/gameviber new -template platformer mygame   # or start from: platformer, topdown, shooter
go run ./cmd/gameviber import GAME.exe mygame      # a GameMaker 5.3 game → a GameViber project (text, PNG, .gml)
go run ./cmd/gameviber play -steps 300 -keys "right:40,right+up:25" -shot frame.png GAME.exe
go run ./cmd/gameviber play -json GAME.exe          # the state as JSON, for scripts and agents
go run ./cmd/gameviber-run GAME.exe                 # play it in a window (arrow keys)
go run ./cmd/gameviber check mygame                 # every .gml parses, every function exists
go run ./cmd/gameviber web mygame site              # a web page that plays it (index.html, game.wasm, project.tar)
go run ./cmd/gameviber run GAME.exe                 # play it in a window (arrow keys)
go run ./cmd/gameviber check mygame                 # every .gml parses, every function exists
```
