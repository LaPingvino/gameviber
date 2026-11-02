# gmgo — a GameMaker clone in Go (working name; Joop names it)

Status: NEW, LOW PRIORITY (Joop 2026-11-02). Design first: nothing beyond research and a proposal until Joop says yes.

## The idea (Joop, 2026-11-02)

"Kinda revive Enigma/LateralGM in Go, using this support. Making it work in all these contexts is a great test.
It would be amazing to have an easy to compile well-supported GameMaker clone." Directions he named:

- Go instead of C++ (ENIGMA's language) — quick compilation.
- Backends to check: Ebitengine (what Stelo, Pacmanio and goloco already use), Azul3D, and Mach
  (https://machengine.org/, Zig) — the last would need a GML library in Zig.
- Open old GameMaker .exe files of games (as done for Stelo and Pacmanio with gm53-reader), and read several
  GameMaker file formats, while keeping its own format for further development.
- Pacmanio (private, never public) as the sample project instead of a hand port; Stelo's GML trial (its LOOP item
  15) moves here.

## Structure (Joop 2026-11-02: "Same structure, include a wasm version for testing, Github Pages")

As Stelo: Go + Ebitengine, one codebase for desktop and the browser; a public GitHub repo with a GitHub Pages
site carrying a wasm test build (the GitHub Actions workflow builds and deploys it on each push to main).
Pacmanio stays private: as the sample it lives outside the public repo (its data is never committed there);
the public test page shows our own small sample games instead.

## Starting points we already have

- ~/Projects/gm53-reader (MIT, public): reads GameMaker 5.3 .exe / project data (Python). Stelo's original/ and
  Pacmanio's recovered source came out of it.
- Stelo and Pacmanio: hand ports in Go + Ebitengine on a small GM engine core (internal/gm: rooms, instances,
  events in GM 5's step order, alarms, collisions, views) and the rules docs (STELO-RULES.md §2,
  PACMANIO-RULES.md) — what GM 5.3 itself does, written down and tested against play.

## Queue (top = next)

1. [ ] Research note (docs/RESEARCH.md): ENIGMA and LateralGM (what they cover, licences — both GPL, so we
   read their docs/behaviour, we don't copy code into an MIT project without deciding that), the GameMaker
   formats (5.x .gmd, 6 .gm6, 7 .gmk, 8 .gm81, the .exe data of each, GMS2 .yyp JSON; LateralGM's/ENIGMA's EGM),
   GML across versions (5.3 → 8.1), the backends (Ebitengine; Azul3D — check whether it is still maintained;
   Mach — state, and what a Zig GML library would mean), and what Stelo + Pacmanio actually use (the 76
   built-ins, event types).
2. [ ] Design proposal (docs/DESIGN.md) for Joop's yes: the GML front end (lexer, parser, one AST) and how it
   runs (tree-walking interpreter first? compile GML to Go for release builds?), the runner (start from Stelo's
   internal/gm), the importers, our own text-based diffable project format, the editor question (a
   LateralGM-like editor: desktop / web?), the backend boundary (so Mach or others can come later), licence.
3. [ ] (after the yes) Pacmanio as the first sample: import its 2006 data, run its GML unchanged, compare with
   the hand port step by step.

## Questions for Joop
- The name.
- (ANSWERED 2026-11-02: public, with Pages + a wasm test build, as Stelo) Public repo or private?
- Licence: MIT like gm53-reader and Stelo's code? (ENIGMA / LateralGM are GPL: we don't copy their code.)
- The GitHub repo is created once the name is chosen (renaming later moves the Pages URL).

## Tick log
- 2026-11-02 — project set up from Joop's message: LOOP.md only, local git, no remote.
- 2026-11-02 — Joop: same structure as Stelo — public repo, wasm test build on GitHub Pages.
