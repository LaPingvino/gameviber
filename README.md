# GameViber

A GameMaker-style game engine and toolset in Go: runs GML, opens old GameMaker games and project files, builds
for the desktop and the browser (WebAssembly), and is made to be driven by people and LLM agents alike.

Design approved; the GML front end is being built. Progress: https://lapingvino.github.io/gameviber/ — the plan is in LOOP.md.

## Trying it

```
go run ./cmd/gameviber new mygame                   # a small starter game to build on
go run ./cmd/gameviber import GAME.exe mygame      # a GameMaker 5.3 game → a GameViber project (text, PNG, .gml)
go run ./cmd/gameviber play -steps 300 -keys "right:40,right+up:25" -shot frame.png GAME.exe
go run ./cmd/gameviber play -json GAME.exe          # the state as JSON, for scripts and agents
go run ./cmd/gameviber-run GAME.exe                 # play it in a window (arrow keys)
go run ./cmd/gameviber check mygame                 # every .gml parses, every function exists
go run ./cmd/gameviber web mygame site              # a web page that plays it (index.html, game.wasm, project.tar)
go run ./cmd/gameviber run GAME.exe                 # play it in a window (arrow keys)
go run ./cmd/gameviber check mygame                 # every .gml parses, every function exists
```
