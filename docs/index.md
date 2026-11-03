---
title: GameViber
---

# GameViber — progress

A GameMaker-style engine and toolset in Go: it runs GML, opens old GameMaker games and project files, builds
for the desktop and the browser (WebAssembly), and is made to be driven by people and LLM agents alike.
MIT licensed · [source on GitHub](https://github.com/LaPingvino/gameviber)

**Status (2026-11-03):** the design is approved; the GML front end and interpreter are done; the runner is being built from Stelo's engine.

| Step | What | Status |
|---|---|---|
| 1 | [Research](RESEARCH.md): ENIGMA / LateralGM (for the vibes), GameMaker's file formats, GML across versions, backends | done |
| 2 | [Design](DESIGN.md): packages, the plain-text project format, the `gameviber` command, how it is proven | approved |
| 3a | GML lexer | **done** — all 42 recovered GML files of Stelo and Pacmanio lex clean (2216 tokens) |
| 3a | GML parser (one AST, GM 5.3 first) | **done** — every piece of GML code in the two games parses (30 code pieces); the rest of their logic is drag-and-drop actions, read by the importer |
| 3b | Interpreter | **done** — values, variables (instance / global / var), every statement, `with`, operators, errors with positions, GM 5's math / string / random built-ins (seeded, replayable); the runner's built-ins come with the runner |
| 3c | Runner (rooms, instances, events in GameMaker's step order), grown from Stelo's engine | **in progress** — Stelo's engine in, its tests pass; instances and globals bound to the interpreter; objects whose events are GML run in GameMaker's step order |
| 3d | Importer for GameMaker 5.3 games | **in progress** — reads a whole GameMaker 5.3 game from its executable — settings, sounds, sprites, backgrounds, scripts, timelines, objects with their events and actions, rooms — matching gm53-reader on Stelo; drag-and-drop actions become readable GML; the game is written out in GameViber's text format (Stelo: 56 GML files, 22 PNGs, rooms in order) |
| 3e | Pacmanio imported and run, compared step by step with the hand port; then Stelo | **in progress** — Pacmanio's first level loads from the 2006 executable and runs headless — creation code, room start, scripts, sounds through a backend hook, the game's constants; plays from its title room through the region's title room into the first level, drawn with its HUD; Stelo's level 2 runs with its view following the player — **step for step identical** to Stelo's hand port (150 steps, the player's position at every one) |

## The sample games

The first proofs are two games made with GameMaker in 2006 and remade in Go by hand, step for step:
[Stelo](https://lapingvino.github.io/stelo/) and [Pacmanio](https://lapingvino.github.io/pacmanio/).
GameViber is done with its first milestone when it runs Pacmanio from the 2006 data and matches the hand port.

## Decisions

- The command is `gameviber` (a UI tool people launch, so a full name).
- Projects are plain text: TOML settings, one `.gml` file per event, rooms as text grids, an `AGENTS.md` for LLM agents.
- Ebitengine is the backend, behind a narrow interface.
