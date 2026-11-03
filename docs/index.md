---
title: GameViber
---

# GameViber — progress

A GameMaker-style engine and toolset in Go: it runs GML, opens old GameMaker games and project files, builds
for the desktop and the browser (WebAssembly), and is made to be driven by people and LLM agents alike.
MIT licensed · [source on GitHub](https://github.com/LaPingvino/gameviber)

**Status (2026-11-03):** the design is approved; engine work has started with the GML front end.

| Step | What | Status |
|---|---|---|
| 1 | [Research](RESEARCH.md): ENIGMA / LateralGM (for the vibes), GameMaker's file formats, GML across versions, backends | done |
| 2 | [Design](DESIGN.md): packages, the plain-text project format, the `gameviber` command, how it is proven | approved |
| 3a | GML lexer | **done** — all 42 recovered GML files of Stelo and Pacmanio lex clean (2216 tokens) |
| 3a | GML parser (one AST, GM 5.3 first) | next |
| 3b | Interpreter | — |
| 3c | Runner (rooms, instances, events in GameMaker's step order), grown from Stelo's engine | — |
| 3d | Importer for GameMaker 5.3 games | — |
| 3e | Pacmanio imported and run, compared step by step with the hand port; then Stelo | — |

## The sample games

The first proofs are two games made with GameMaker in 2006 and remade in Go by hand, step for step:
[Stelo](https://lapingvino.github.io/stelo/) and [Pacmanio](https://lapingvino.github.io/pacmanio/).
GameViber is done with its first milestone when it runs Pacmanio from the 2006 data and matches the hand port.

## Decisions

- The command is `gameviber` (a UI tool people launch, so a full name).
- Projects are plain text: TOML settings, one `.gml` file per event, rooms as text grids, an `AGENTS.md` for LLM agents.
- Ebitengine is the backend, behind a narrow interface.
