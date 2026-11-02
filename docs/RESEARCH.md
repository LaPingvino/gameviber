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
