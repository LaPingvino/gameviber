---
title: GameViber
---

# GameViber — progress

A GameMaker-style engine and toolset in Go: it runs GML, opens old GameMaker games and project files, builds
for the desktop and the browser (WebAssembly), and is made to be driven by people and LLM agents alike.
MIT licensed · [source on GitHub](https://github.com/LaPingvino/gameviber)

**Status (2026-11-03):** both sample games import from their 2006 executables and play — in a window, in the browser and headless — step for step identical to their hand ports. Every GameMaker 5 event type but path end runs; sound and music play; resources go by GameMaker's numeric ids. Text is drawn (sharp at the screen's resolution in the window and the browser), collisions can be precise, and there is an editor (below). Still to come: paths.

| Step | What | Status |
|---|---|---|
| 1 | [Research](RESEARCH.md): ENIGMA / LateralGM (for the vibes), GameMaker's file formats, GML across versions, backends | done |
| 2 | [Design](DESIGN.md): packages, the plain-text project format, the `gameviber` command, how it is proven | approved |
| 3a | GML lexer | **done** — all 42 recovered GML files of Stelo and Pacmanio lex clean (2216 tokens) |
| 3a | GML parser (one AST, GM 5.3 first) | **done** — every piece of GML code in the two games parses (30 code pieces); the rest of their logic is drag-and-drop actions, read by the importer |
| 3b | Interpreter | **done** — values, variables (instance / global / var), every statement, `with`, operators, errors with positions, GM 5's math / string / random built-ins (seeded, replayable); the runner's built-ins come with the runner |
| 3c | Runner (rooms, instances, events in GameMaker's step order), grown from Stelo's engine | **done for GM 5's events** — create / destroy, begin / step / end step, alarms, keyboard / key press / key release, mouse (on the instance and global), collisions, outside the room, intersect boundary, room start / end, game start / end, no more lives, animation end, user events (`event_user`), draw; instance ids (`instance_create` returns one, `with (n)`, `n.x`); views, backgrounds, timelines, persistent rooms; pauses (`action_sleep`) keep the screen alive. Left: path end (no paths yet), precise collisions |
| 3d | Importer for GameMaker 5.3 games | **done for 5.3** — reads a whole GameMaker 5.3 game from its executable — settings, sounds, sprites, backgrounds, scripts, timelines, objects with their events and actions, rooms — matching gm53-reader on Stelo; drag-and-drop actions become readable GML; the game is written out in GameViber's text format (Stelo: 56 GML files, 22 PNGs, rooms in order); MIDI music is rendered to Opus next to the original |
| 3e | Pacmanio imported and run, compared step by step with the hand port; then Stelo | **done** — Pacmanio's first level loads from the 2006 executable and runs headless — creation code, room start, scripts, sounds through a backend hook, the game's constants; plays from its title room through the region's title room into the first level, drawn with its HUD; Pacman's moves, eating and scoring identical to the hand port at every step (ghosts, which move at random, taken out); Stelo's level 2 runs with its view following the player — **step for step identical** to Stelo's hand port: levels 1–3 under a scripted run (jumps, deaths, scoring, lives), position / score / lives equal at every one of 360 steps |

## Try it

**New here? [Your first game, in about ten minutes](tutorial.html)** — pick a starting point in the editor, change how it plays, change a level, and work on it with an AI assistant.

**[Play the demo](demo/)** — GameViber's own starter game (`gameviber new`), running in your browser: the arrow keys (or a finger held towards where to go)
collect the coins. It is what `gameviber new` writes, built with `gameviber web`.


`gameviber import GAME.exe DIR` turns a GameMaker 5.3 game into a GameViber project; `gameviber play` runs it (or a project folder — edit the .gml and run again)
headless (`-keys "right:40,right+up:25"`, `-shot frame.png`, `-json` for the state).

## Try the editor

**[Open the editor](editor/)** — GameViber's editor, running entirely in your browser (the same editor
`gameviber edit DIR` serves on your own machine): the starter game's files in a tree, each object's events
as GML text checked as you save, rooms drawn with their sprites — click to place things, right click to
remove — sprites with their origin and bounding box, new objects, events and rooms, new sprites from PNG pictures (+ Sprite), and **▶ Play** to run what you
saved — with ⏸ pause and ⏭ one step at a time. Your changes stay
in this browser (Reset brings back the starter game). Objects open as a form (sprite, solid, depth, parent …); sounds can be listened to and added (+ Sound); sprites can be drawn on (pencil, eraser).

## With an assistant

In Chrome or Edge, **Open folder…** in the editor works on a GameViber project folder on your disk instead of in
the browser: run a coding agent (Claude Code, Antigravity, OpenCode …) in that folder in your own terminal, and the
editor follows its changes as you both work.


GameViber is made to be worked on together with an AI assistant: everything is plain text, and `gameviber check`
explains its problems with the file and line.

- In the editor: **Copy for an assistant** puts the project's context (its AGENTS.md, the files, the open file,
  the check) on the clipboard for any chat; **Paste an answer** takes the reply back — its files listed as new or
  changed, the ones you tick written, then checked.
- For assistants that work on the files themselves (Claude Code and other MCP clients): `gameviber mcp DIR` serves
  the project's `check`, `play` (with a screenshot when asked) and `reference` as MCP tools — in Claude Code:
  `claude mcp add gameviber -- gameviber mcp DIR`. Or once for every agent you have installed (Claude Code,
  Antigravity, Gemini CLI, OpenCode): `gameviber setup-agents` — then they have the tools in any GameViber project.

## Playing: the window and the browser

`gameviber run GAME` opens a window; `gameviber web DIR OUT` builds a page. Both scale sharply (whole steps, never
blurred), take the keyboard, the mouse and touch (arrow pads beside or under the game once the screen is touched), play WAV effects and Opus
music, show the game's caption as the title, and have a sound button (M) and full screen (F11, Alt+Enter) in the
margin beside the game. In the browser `get_string` and `show_message` use the page's dialogs (a phone shows its
keyboard), and a new build is never served stale.

## For agents (and people) writing GML

`gameviber new DIR` writes a small game with an `AGENTS.md`; `gameviber check DIR` reports syntax errors and
unknown functions — naming the GameMaker 5.3 way for later GameMaker names (`irandom`, `clamp`,
`draw_set_color` …) and the nearest known name for a typo; `gameviber play` runs a game headless with scripted
keys and writes the state as JSON or the last frame as a PNG.

## Reference

[What GameViber's GML understands](reference.md) — the events (one .gml file each), every function, the variables
and constants; generated by `gameviber reference`, checked by the tests.

## The sample games

The first proofs are two games made with GameMaker in 2006 and remade in Go by hand, step for step:
[Stelo](https://lapingvino.github.io/stelo/) and [Pacmanio](https://lapingvino.github.io/pacmanio/).
GameViber is done with its first milestone when it runs Pacmanio from the 2006 data and matches the hand port.

## Decisions

- The command is `gameviber` (a UI tool people launch, so a full name).
- Projects are plain text: TOML settings, one `.gml` file per event, rooms as text grids, an `AGENTS.md` for LLM agents.
- Ebitengine is the backend, behind a narrow interface.
