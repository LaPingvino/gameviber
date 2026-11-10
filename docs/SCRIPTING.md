---
title: GameViber — game code in JavaScript, Lua and Go (proposal)
---

# Game code in JavaScript, Lua and Go — proposal

Status: **approved** (Joop, 2026-11-05, modules included): "yaegi is probably unavoidable for the wasm version,
but it would be nice to have as much compiled as possible where possible too." Compiled wherever a compiler is
at hand; interpreted only where it must be. Asked 2026-11-05: "what about adding
the ability to support JS, Lua and pure Go in our own game code?"

## Why

GameViber's code is GameMaker 5's GML, which is right for running 2006 games unchanged — and GameViber's own
reference covers it. But most people (and every language model) write JavaScript, Lua or Go far better than
GML. Letting a game's events be written in those, beside GML, makes GameViber easier to start with and makes
its assistants better at it, without giving up the GM 5 compatibility.

## The shape: one game API, several languages

Today every event is a `runner.Handler` (`func(room, self)`), made from source by `Game.Code` at the five
places the loader compiles code (load/load.go), and every built-in — `instance_create`, `draw_text`,
`move_contact_solid`, … — is a `gml.Func` in one table (`Game.Funcs`), with instances read and written
through `Get` / `Set`. That is the seam: a language adapter turns source into a `Handler` and calls the same
table. Nothing in the runner changes; GML stays the default and the reference for behaviour.

- **Which language** is the event file's extension: `objects/player/step.gml`, `step.js`, `step.lua`,
  `step.go`. One object may mix them; a game may too. Scripts likewise (`scripts/jump.js`), callable from any
  language by name.
- **The same names everywhere.** Each language sees GameViber's functions under their GML names and the
  instance's variables as fields: in JavaScript `x += 4; if (place_free(x, y + 1)) vspeed = 2;` reads the
  instance (`this` is self, `other` in collisions), Lua the same through a metatable, Go through a small
  package (below). Constants (`vk_left`, `c_red`, object names) are there as values. `docs/reference.md`
  stays the one list of what exists, with a line per language on how to call it.
- **Errors** carry file and line like GML's, and land in the same places (the editor's problems, `check`,
  the chat session's results). `check` also looks up JavaScript's calls to plain names as it does GML's: one
  that neither GameViber, the project's scripts, the file itself nor JavaScript defines is reported with
  file:line:col (2026-11-10).

## Each language

| | Library (pure Go) | Where it runs | Notes |
|---|---|---|---|
| JavaScript | goja | desktop and browser | ES2015+; instance fields through a goja DynamicObject; first, since models know it best |
| Lua | gopher-lua | desktop and browser | Lua 5.1; small; instance fields through a metatable |
| Go | yaegi (while editing), the Go compiler (export) | interpreted everywhere; compiled into exported games | a package `gv` with the API as typed functions; the same source runs interpreted in the editor and compiled at full speed in `gameviber export` |

All three are pure Go — no cgo — so the browser editor and the web player keep working; each adds to the
wasm's size (to measure; a build without a language can leave it out).

## What it costs

- Three new modules (Joop's yes): goja, gopher-lua, yaegi.
- A binding layer per language (calling `gml.Func`s with the right self / other, values both ways, errors with
  lines), the loader choosing by extension, `check` and the editor knowing the new files (syntax colours,
  problems), the assistant's first message saying which languages a project uses.
- Tests: the same small game written four times (GML, JS, Lua, Go) must play identically — the existing
  test-play harness compares their runs step for step.

## Order

1. This note → Joop's yes on the modules.
2. JavaScript end to end (loader, binding, errors, editor, check, the four-way test with GML).
3. Lua.
4. Go: interpreted first; compiled in export after. **Done 2026-11-06**: `export -desktop` (this system, and
   Windows from anywhere) and the web export compile a game's `.go` events and scripts into its player
   (`cmd/gameviber/compiled.go`); without Go, or for macOS / Linux from another system, the export says why and
   ships the interpreting player. The editor's Play stays interpreted (the code changes between Plays).
5. The templates and tutorial offering a language choice.
