# GameViber — design (APPROVED by Joop 2026-11-02)

From docs/RESEARCH.md and Joop's directions (LOOP.md): a GameMaker-style engine in Go, simple, LLM-friendly,
MIT, desktop and browser; Pacmanio as the first sample; ENIGMA / LateralGM for vibes only.

## 1. Shape: one Go module, a few packages, one command
```
gameviber/
  gml/        lexer, parser (one AST, version flags), interpreter
  runner/     the GameMaker world: rooms, instances, events in GM's step order, alarms, timelines,
              collisions, views, globals — grown from Stelo / Pacmanio's internal/gm (already step-exact)
  builtins/   the functions and drag-and-drop actions, a table per GM version (name → Go)
  project/    GameViber's own text project format: read, write, validate
  import/     gmfile (GM 5.3 .exe / .gmd first — gm53-reader in Go), then .gm6 / .gmk / .gm81, …
  backend/    a narrow interface (draw sprite / text / rect, input, audio); ebiten/ implements it
  export/     desktop binary and the web page (the "export options": sharp scaling, touch controls,
              full screen, sound button, Opus via gopus, cache-safe page …)
  cmd/gameviber/  the command line (below)
```

## 2. The project format (plain text, small files, diffable)
```
mygame/
  game.toml                    name, size, speed, start room, GM version for GML, export options
  sprites/player/strip.png     frames side by side + sprite.toml (origin, box, frame size, speed)
  objects/player/object.toml   sprite, solid, visible, depth, parent
  objects/player/create.gml    one .gml file per event (step.gml, collision-wall.gml, alarm-0.gml …)
  rooms/level1.room            a text grid with a legend (as Stelo's levels/) or an instance list
  scripts/*.gml, sounds/*.opus, fonts/, backgrounds/
  AGENTS.md                    for LLM agents: what the game is, how to build / test it
```
Read and written only by GameViber; old GameMaker formats are imported into it, never written.

## 3. The command line (people and agents alike)
- `gv new mygame` · `gv import game.exe|game.gmd|game.gmk mygame` · `gv check` (validate, parse all GML)
- `gv run` (desktop) · `gv serve` (the web build locally) · `gv build --web|--desktop`
- `gv play --headless --steps 600 --input moves.txt --shot frame.png --state state.json` — deterministic runs
  with scripted input, screenshots and a state dump, so a change is verified by replay without watching.
- `gv test` — runs a game's replays and compares their results (the way Stelo's bot playthroughs do).
Every command prints plain, actionable errors (file:line, what was expected).

## 4. Running GML
A tree-walking interpreter first (no compile step in the edit-run loop); GM 5.3's subset (RESEARCH §1) first,
then 6–8.1, GMS2's newer language later as its own mode. Determinism: fixed seeds, step-exact timing,
`random` from the runner's own generator, so replays match.

## 5. How it is proven
1. Pacmanio: imported from its 2006 executable, run on GameViber's GML, compared step for step with the hand
   port (its tests and bot runs as the oracle). Then Stelo the same way.
2. The web export of Pacmanio = today's page (sharp, touch controls, sound, Opus).
3. Then one GM 6–8.1 game to widen the importer and the built-ins.

## 6. Not in the first version
A GUI editor (later, a web editor on the same engine); GMS2's language; compiled data.win; Mach (the backend
interface keeps it possible).

## Decisions (Joop, 2026-11-02)
1. The command is `gameviber` (not `gv`): read `gv` above as `gameviber`.
2. TOML for settings, text grids for rooms, .gml per event. Go has no TOML in its standard library: a small
   parser of our own for the subset used (tables, strings, numbers, booleans, arrays) — no new module.
3. Pacmanio first (its assets imported into a local folder, never into the public repo), then Stelo.
