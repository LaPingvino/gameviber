# GameViber research note

LOOP item 1. Parts: (1) what the two sample games use — DONE below; (2) ENIGMA and LateralGM, the vibes only;
(3) GameMaker file formats; (4) GML across versions; (5) backends (Ebitengine, Azul3D, Mach); (6) libraries
around it (GML, GameMaker files, gopus — published). The design (item 2) is written from this.

## 1. What Stelo and Pacmanio use (counted 2026-11-02 from the recovered sources)

Source: `stelo/original/` (22 object / script / timeline files) and `pacmanio-recovered/pacmanio/` (20), as
gm53-reader exports them: GameMaker 5.3 pseudo-GML — drag-and-drop actions written as `action_*` calls next to
code actions. Their hand ports (Go, `internal/gm` + `internal/game`) already implement all of this, which gives a
ready-made oracle: the interpreter running the original GML must step exactly as the ports do (their tests and
bot playthroughs).

### Events (counts: Stelo / Pacmanio)
Collision 16 / 10 · Create 9 / 7 · Step 5 / 4 · Keyboard (held) 4 / 5 · End step 3 / 0 · Room start 3 / 5 ·
Room end 0 / 4 · Draw 3 / 3 · Alarm 1 / 6 · Key press 1 / 0 · Outside room 1 / 2 · No more lives 1 / 1 ·
Game start 1 / 1 · Game end 1 / 0. Plus timelines (Stelo: faruMonsterojn, faruFinmonsterojn).

### Drag-and-drop actions (45 distinct across both)
- Movement: `action_move`, `action_move_to`, `action_move_start`, `action_move_contact`, `action_set_vspeed`,
  `action_set_gravity`, `action_reverse_xdir`, `action_reverse_ydir`.
- Instances: `action_create_object`, `action_kill_object`, `action_change_object`, `action_set_sprite`.
- Questions: `action_if`, `action_if_variable`, `action_if_empty`, `action_if_object`, `action_if_collision`,
  `action_if_number`, `action_if_next_room`, `action_if_aligned`, `action_if_life`.
- Score / lives: `action_set_score`, `action_set_life`, `action_draw_score`, `action_draw_life`,
  `action_draw_life_images`.
- Rooms / game: `action_next_room`, `action_another_room`, `action_sleep`, `action_set_alarm`,
  `action_set_timeline`, `action_set_caption`, `action_execute_script`.
- Sound: `action_sound`, `action_end_sound`.
- Drawing: `action_draw_text`, `action_draw_font`, `action_draw_rectangle`, `action_draw_sprite`, `action_color`.
- Unresolved by gm53-reader: `action_1_809`, `action_1_813` (library 1, ids 809 / 813 — in Pacmanio's room-end
  events; to identify from GM 5.3's action library).

### GML functions (37 distinct across both)
`place_free`, `random`, `floor`, `round`, `ord`, `string_count`, `variable_global_exists`, `instance_change`,
`instance_destroy`, `room_goto`, `room_goto_next`, `game_restart`, `game_end`, `keyboard_wait`, `get_string`,
`sound_loop`, `sound_isplaying`, `sound_stop_all`, `cd_init`, `cd_present`, `cd_number`, `cd_play`,
`cd_playing`, `cd_track`, `draw_text`, `draw_background`, `draw_highscore`, `screen_refresh`, `screen_redraw`,
`highscore_add`, `highscore_value`, `highscore_clear`, `set_graphics_mode`, `io_handle`.

### Language features
Code actions use: `if / else`, `with (obj)`, `exit`, `switch / case`, `not / and / or`, comparison and
arithmetic, `global.` variables, instance variables (`other.`, `self`), `#` as a newline in strings, room
and instance built-ins (`x`, `y`, `xstart`, `ystart`, `hspeed`, `vspeed`, `gravity`, `image_index`,
`image_speed`, `sprite_index`, `background_color`, `background_hspeed`, `view_left[0]`, `room_caption`, `score`,
`lives`, `keyboard_string`).

### What this means for the first version
A GM 5.3 subset is small and fully known: ~13 event types, 45 actions, 37 functions, a handful of
statements. Both games already have a step-exact engine model (STELO-RULES.md §2, PACMANIO-RULES.md) to test
against. Things with no modern meaning get a defined stand-in (cd_* — no CD: `cd_present()` false;
`set_graphics_mode`, `io_handle` — no-ops; `get_string` / `keyboard_wait` — the engine's own text field and
any-key wait, as the ports do).

## 2. ENIGMA and LateralGM — the vibes (Joop: "borrow from the vibes … dead in the water … a bit too complex")

Read 2026-11-02 from their repositories and documentation; no code taken (both GPL-3, GameViber is MIT).

**ENIGMA** — a GameMaker-compatible engine. Its language, EDL, is GML extended; ENIGMA parses it and
**translates it to C++**, then builds the game with a C++ toolchain on the user's machine. Usable from an IDE
(LateralGM, as a plugin) or the command line. Still alive but slow: last push to its main repository January
2026, ~370 stars, ~155 open issues; old directory entries still list a 2014 release.
- Good vibes: GML that compiles to a native game; a compatibility layer per GM function; a command line next
  to the IDE.
- What made it heavy: a C++ compiler and libraries on every user's machine, long builds, a large compatibility
  surface aimed at every GM version at once, and a separate Java editor to keep in step with it.

**LateralGM** — a Java Swing editor for GameMaker project files: reads and writes GM 5.x .gmd, 6 .gm6, 7/8
.gmk / .gm81 and Studio 1.x .gmx (as ENIGMA's EGM too), converts between them; cannot run games itself
(ENIGMA as a plugin does). Its author was the community's file-format expert.
- Good vibes: one editor that opens every old format; conversion between versions; formats documented by use.
- What made it heavy: a desktop Java GUI app, a plugin bridge to a C++ engine, two projects to keep in step.

**What GameViber takes from this**
1. One program, one language: Go engine, importers, command line and (later) editor in one module; no
   compiler on the user's machine for playing or building (Go's toolchain only for exporting a standalone
   game, and that is quick).
2. Run GML with an interpreter first — no compile step in the edit-run loop; compile to Go later only if a
   game needs the speed.
3. A narrow, proven target first: GM 5.3's subset as used by Stelo and Pacmanio (§1), checked step for step
   against their hand ports; widen version by version (GM 6, 7, 8) with a sample game for each.
4. Import every old format into **one** plain-text project format (LLM-friendly, diffable); write only that
   format (no round-tripping .gmk etc.).
5. The command line before any GUI editor (agents and people alike, §LLM in LOOP.md); a web editor later, on
   the same engine in wasm.

Sources: github.com/enigma-dev/enigma-dev (via awesome.ecosyste.ms), github.com/IsmAvatar/LateralGM,
yal.cc/gamemaker-formats, enigma-dev.org/docs/Wiki/Related_Projects:Developing.

## 3. GameMaker file formats

Overview from yal.cc/gamemaker-formats (YellowAfterlife), plus what gm53-reader already handles.

| Format | Versions | What it is | Shape |
|---|---|---|---|
| `.gmd` | GM ≤ 5 | project | binary, version-tagged resource lists (gm53-reader reads 5.3's layout) |
| `.gm6` | GM 6 | project | binary; resources zlib-compressed |
| `.gmk` | GM 7, 8 | project | binary; GM 7/8 obfuscate parts of the file |
| `.gm81` | GM 8.1 | project | as `.gmk`, 8.1 layout |
| `.gb1`–`.gb9` | GM ≤ 8.1 | backups of the above | same formats |
| `.exe` (GM ≤ 8.1) | each version | the game's data appended to the runner | 5.x: plain sections (gm53-reader: Stelo, Pacmanio); 6–8.1: compressed and, from 7 on, scrambled — the project can be recovered |
| `.lib` | GM ≤ 8.1, GMS1 | drag-and-drop action libraries | binary — the source of the action ids (Pacmanio's unresolved `action_1_809` / `813`) |
| `.gex` | GM ≤ 8.1 | extension packages | binary |
| `.gmspr` | GM ≤ 8.1 | a sprite export | zlib, PNG-like with GM metadata |
| `.project.gmx` + `*.object.gmx` … | GM:Studio 1 | project | XML, one file per resource; `.gmz` = 7-zip of it |
| `.yyp` + `.yy` | GMS2 | project | JSON, one file per resource; `.yyz` = zip |
| `data.win` / `.unx` / `.ios` / `.droid` | GMS1/2 compiled | compiled game | IFF-like (`FORM`, `GEN8` …), bytecode or native (YYC) — UndertaleModTool's territory |

**What GameViber reads, in order** (each with a sample game, and each only an importer into GameViber's own format):
1. GM 5.3: `.exe` and `.gmd` — gm53-reader's knowledge moved into a Go package (`gmfile`), checked on Stelo and
   Pacmanio byte for byte against the Python reader's exports.
2. GM 6 / 7 / 8 / 8.1 projects (`.gm6`, `.gmk`, `.gm81`) — the bulk of old hobby games; LateralGM proves they can
   be read fully (behaviour learnt from its documentation and the files, not its GPL code).
3. GM 6–8.1 `.exe` — recovering projects from games whose source is lost (Joop's own case for Stelo / Pacmanio).
4. GM:Studio `.gmx` (XML) and GMS2 `.yyp` (JSON) — easy to parse; the GML is newer (needs §4's later versions).
5. Compiled `data.win` — last, if ever (bytecode, version-specific; UndertaleModTool already covers it).

**GameViber's own format** (LLM-friendly, diffable; to be designed in item 2): a directory — `game.toml` (or
similar) for settings; one folder per resource kind; one small text file per object with its events as
`.gml` files next to it; rooms as text grids or instance lists (as Stelo's `levels/*.txt`); sprites as PNG
strips with a text sidecar (origin, box). Written only by GameViber; old formats are read, never written.

Source: yal.cc/gamemaker-formats.

## 4. GML across versions (what an interpreter grows into)

- **GM 5.x** (Stelo, Pacmanio): drag-and-drop actions next to code; untyped variables (real / string); `with`,
  `repeat`, `exit`; arrays only one-dimensional, plus `x[i, j]` two-dimensional in later 5.x; `global.`;
  `argument0…` in scripts; `#` as the newline in strings; function set as §1.
- **GM 6** — Direct3D-era drawing (`d3d_*`), particles, more data structures (`ds_list`, `ds_map` …), timelines and
  paths matured; the same language core.
- **GM 7 / 8 / 8.1** — extensions (`.gex`), `.lib` action libraries, alpha sprites, more functions; the language
  itself barely changes (no new syntax worth naming). Most old hobby games are GM 6–8.1: the main target after 5.3.
- **GM:Studio 1** — the same GML with many functions removed or renamed (e.g. file I/O sandboxed), new ones
  (`ds_grid`, shaders, `var` local declarations everywhere); compiled by YoYo's runner (bytecode or YYC).
- **GMS2 (2.3+)** — a much newer language: functions as values, structs, constructors, methods, `new`, exceptions,
  arrays as references, chained accessors; one `.gml` file per script holding many functions. Effectively GML 2.

**Plan**: one parser with version flags; the interpreter starts at 5.3 (§1's subset), grows through 6–8.1 by sample
games, and treats GMS2-style GML as a later language mode, not the first goal. The built-in functions live in a
table per version (name → Go function), so a game's version decides what exists — the way the runners differed.

## 5. Backends

- **Ebitengine** (Go) — what Stelo, Pacmanio, Pits' tools and goloco already use: desktop, browser (wasm), mobile;
  2D, an image / shader model that fits GameMaker's sprite-and-surface drawing directly; active. **The backend.**
- **Azul3D** (Go, 3D) — dormant since 2021, not formally archived; its author's last note points Go game developers
  to Zig and Mach. Not a candidate.
- **Mach** (Zig) — active (development on code.hexops.org, GitHub a mirror), but by its own docs experimental and
  not ready for games; browser support was a goal, not confirmed. A Mach target would need the engine in Zig (a
  Zig GML library), i.e. a second implementation. **Later, if ever**: keep the engine's core (GML, the runner's
  step model, the formats) independent of Ebitengine behind a small drawing / input / audio interface, so another
  backend stays possible without promising one.

**Decision proposed for the design (item 2)**: Ebitengine only, behind a narrow backend interface; Mach revisited
when it is ready for 2D games.

Sources: github.com/azul3d (via awesome.ecosyste.ms), machengine.org (docs, FAQ, roadmap), gitpedia.pages.dev/wiki/hexops/mach.
