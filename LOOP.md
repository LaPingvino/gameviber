# GameViber — a GameMaker clone in Go, with LLM use as a first-class citizen

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

## LLM use as a first-class citizen (Joop 2026-11-02: "it would be nice to make LLM usage with the new thing a
first class citizen as well. Maybe we can call it GameViber")

RULE (Joop 2026-11-02: "it's also nice to err towards things that are nice for LLMs to work with"): when two
designs are otherwise close, pick the one that is easier for a model to read, write, run and check.

Directions for the design (to be weighed there, not decided here):
- A project format that is plain text, small files, diffable — easy for a model to read and edit (GML scripts as
  .gml files, objects / rooms as readable text), with a schema the tools validate.
- A command-line tool an agent can drive end to end: create, import (.exe / .gmd / .gm6 / .gmk / .gm81),
  validate, build (desktop / wasm), run headless for N steps with scripted input, screenshot, dump state —
  every result checkable without a human watching.
- Deterministic runs (fixed seeds, step-exact) so a change can be verified by replay, as the Stelo / Pacmanio
  bot playthroughs do.
- Error messages and docs written for agents too (an AGENTS.md / CLAUDE.md in each project, the GML reference
  as text).
- Later, maybe: an MCP server over the same commands; an assistant inside the editor.

## Structure (Joop 2026-11-02: "Same structure, include a wasm version for testing, Github Pages")

As Stelo: Go + Ebitengine, one codebase for desktop and the browser; a public GitHub repo with a GitHub Pages
site carrying a wasm test build (the GitHub Actions workflow builds and deploys it on each push to main).
Pacmanio stays private: as the sample it lives outside the public repo (its data is never committed there);
the public test page shows our own small sample games instead.

## Stance towards ENIGMA and LateralGM (Joop 2026-11-02)

"Borrow from the vibes of those two projects, they are basically dead in the water at this point and a bit too
complex to my taste." So: take their ideas (what a GM clone needs, which formats matter, what users missed),
not their code or architecture (both GPL; we are MIT). Keep GameViber small and simple — simplicity is a design
goal of its own, weighed alongside the LLM-friendly rule.

## Starting points we already have

- ~/Projects/gm53-reader (MIT, public): reads GameMaker 5.3 .exe / project data (Python). Stelo's original/ and
  Pacmanio's recovered source came out of it.
- Stelo and Pacmanio: hand ports in Go + Ebitengine on a small GM engine core (internal/gm: rooms, instances,
  events in GM 5's step order, alarms, collisions, views) and the rules docs (STELO-RULES.md §2,
  PACMANIO-RULES.md) — what GM 5.3 itself does, written down and tested against play.

## Queue (top = next)

1. [x] Research note (docs/RESEARCH.md; parts 1–5 done 2026-11-02): ENIGMA and LateralGM — the vibes only (what they cover, what made them
   too complex, what users valued); no code taken (GPL; GameViber is MIT), the GameMaker
   formats (5.x .gmd, 6 .gm6, 7 .gmk, 8 .gm81, the .exe data of each, GMS2 .yyp JSON; LateralGM's/ENIGMA's EGM),
   GML across versions (5.3 → 8.1), the backends (Ebitengine; Azul3D — check whether it is still maintained;
   Mach — state, and what a Zig GML library would mean), and what Stelo + Pacmanio actually use (the 76
   built-ins, event types).
2. [x] Design proposal (docs/DESIGN.md) — APPROVED by Joop 2026-11-02 (command `gameviber`, TOML via BurntSushi/toml (Joop OK'd the library), Pacmanio first): the GML front end (lexer, parser, one AST) and how it
   runs (tree-walking interpreter first? compile GML to Go for release builds?), the runner (start from Stelo's
   internal/gm), the importers, our own text-based diffable project format, the editor question (a
   LateralGM-like editor: desktop / web?), the backend boundary (so Mach or others can come later), licence.
3. [ ] NEXT — Pacmanio as the first sample: import its 2006 data, run its GML unchanged, compare with
   the hand port step by step.

## Libraries around it (Joop 2026-11-02: an Opus decoder "would fit right along the gml stuff … and the game
maker file decoder")

Each a separate, reusable Go module (MIT), useful outside GameViber too:
- **GML**: lexer, parser, interpreter (the engine's core).
- **GameMaker files**: a Go reader for .exe data and project files across versions — the Go successor of
  gm53-reader (Python), which already reads GM 5.3.
- **Opus**: a pure-Go Opus decoder (none exists that covers the whole codec: pion/opus does SILK only; the
  others wrap C libopus, which rules out wasm). RFC 6716 (range decoder, SILK, CELT, hybrid) + RFC 7845 (Ogg
  Opus); port from libopus (BSD-licensed, compatible with MIT); verified bit-exact against the official test
  vectors (RFC 8251) as goloco is verified against OpenLoco. A long series of ticks. Until it exists, the games
  use Ogg Vorbis (Ebitengine's pure-Go decoder).
  Joop: browsers decode Opus natively (WebRTC), so the web builds can hand Opus to the browser
  (decodeAudioData) right away — "but it's still nice to have one" for the desktop and for one code path.
These belong in the research note (item 1) and the design (item 2); none is started before the design's yes.

## Export options (Joop 2026-11-02: "You could copy these tweaks to GameViber's compilation options")

What Stelo, Pacmanio and Pits needed for today's screens becomes standard in a GameViber build / export, each a
setting (on by default) rather than per-game code:
- sharp scaling: whole steps without smoothing when they fill ≥ 90%, else the largest fraction (never blurred);
  the screen's real resolution for text;
- text and menus at the real resolution (Noto Sans, OFL; the overlay of texts and rectangles);
- touch controls for any touch screen (incl. Chromebooks: any-pointer / the first touch), laid out beside or under
  the game, never over it; a d-pad with Hold / Toggle / Push-like modes where the game wants them;
- full screen (a button, F11, Alt+Enter) and a sound on / off button (M);
- the phone keyboard for text input; Esc / Cancel for text prompts;
- the web page: cache revalidation (no stale build after a deploy), the audio unlock on the first touch;
- audio as Ogg Opus via gopus (pure Go), music streamed, effects decoded up front;
- pauses (action_sleep) that keep the screen alive.

## Questions for Joop
- (ANSWERED 2026-11-02: GameViber) The name.
- (ANSWERED 2026-11-02: public, with Pages + a wasm test build, as Stelo) Public repo or private?
- (ANSWERED 2026-11-02: MIT) Licence.
- (DONE 2026-11-02) GitHub repo: https://github.com/LaPingvino/gameviber (public; Pages with the first code).

## Blocked
- (RESOLVED 2026-11-02 with Joop's go-ahead: created) `gh repo create LaPingvino/gameviber --public …` refused by the auto-mode check ('Create Public
  Surface'). Joop: `cd ~/Projects/gameviber && gh repo create LaPingvino/gameviber --public --source . --remote
  origin --push` (or create it on github.com and say so; I add the remote and push).

## Tick log
- 2026-11-02 — project set up from Joop's message: LOOP.md only, local git, no remote.
- 2026-11-02 — Joop: same structure as Stelo — public repo, wasm test build on GitHub Pages.
- 2026-11-02 — Joop: name GameViber; LLM use first-class. Directory renamed ~/Projects/gmgo → ~/Projects/gameviber.
- 2026-11-02 — Joop: err towards what is nice for LLMs to work with (recorded as a design rule). Public repo creation refused by the permission check → Blocked.
- 2026-11-02 — public repo created with Joop's go-ahead and pushed: https://github.com/LaPingvino/gameviber
- 2026-11-02 — Joop: MIT (LICENSE added); borrow the vibes of ENIGMA / LateralGM, not their complexity.
- 2026-11-02 — Joop: a pure-Go Opus decoder and the GameMaker file reader as libraries alongside the GML work (recorded; design first).
- 2026-11-02 — item 1 part 1: docs/RESEARCH.md §1, counted from the recovered sources: ~13 event types, 45 drag-and-drop actions (two unresolved: action_1_809 / 813), 37 GML functions, the statements used; the hand ports as the oracle. Next: §2 ENIGMA / LateralGM (vibes), §3 formats.
- 2026-11-02 — item 1 part 2: ENIGMA (GML→C++ via EDL, a C++ toolchain per user, alive but slow) and LateralGM (Java editor, every old format, runs nothing itself) — the vibes, and five takeaways (one Go program; interpreter first; GM 5.3 subset first, then version by version; import everything into one text format; command line before a GUI). Next: §3 formats.
- 2026-11-02 — item 1 part 3: the GameMaker formats (.gmd → .gm81, the .exe data per version, .lib / .gex / .gmspr, .gmx XML, .yyp JSON, compiled data.win) and the importer order: GM 5.3 exe / gmd (gm53-reader → Go) → GM 6–8.1 projects → their exes → gmx / yyp → data.win last; GameViber's own text project format sketched. Next: §4 GML across versions, §5 backends.
- 2026-11-02 — Joop: the Stelo / Pacmanio / Pits web and phone tweaks become GameViber's export options (listed).
- 2026-11-02 — item 1 done: §4 GML across versions (5.x → 6 → 7/8/8.1 → GMS1 → GMS2's newer language; one parser with version flags, built-ins per version) and §5 backends (Ebitengine the backend; Azul3D dormant since 2021, its author points to Mach; Mach active but not ready for 2D games — keep a narrow backend interface). Next: item 2, the design proposal for Joop.
- 2026-11-02 — item 2: docs/DESIGN.md written (packages, the text project format, the gv command with headless deterministic replays, interpreter first, proven on Pacmanio then Stelo, export options, no editor / GMS2 / data.win in v1; three questions). WAITING for Joop's yes.
- 2026-11-02 — Joop approved the design (asked directly): command `gameviber`, TOML (BurntSushi/toml, Joop OK'd), Pacmanio then Stelo. Item 3 next, in steps: 3a the GML lexer + parser (GM 5.3 subset, one AST, tests on Pacmanio's and Stelo's recovered .gml — every file must parse), 3b the interpreter, 3c the runner from Stelo's internal/gm, 3d the importer, 3e step-by-step comparison with the hand port.
- 2026-11-03 (item 3a, part 1: the lexer) — go.mod (github.com/LaPingvino/gameviber, go 1.24) and package gml: Token / Kind / Pos, GM 5.3's keywords (word operators too; built-in constants stay identifiers), every operator longest-first (incl. <>, :=, ^^, <<=), $hex, '…' / "…" strings without escapes that may span lines, // and /* */ comments, '#' at a line start as a Directive (the recovery format's #event headers); never fails — Illegal tokens for the parser to report. TestLex (cases) + TestLexCorpus: all 42 recovered files of Stelo and Pacmanio (scripts whole, objects' code actions; the objects' D&D lines are the recovery tool's output, not GML) lex clean, 2216 tokens. NEXT 3a part 2: the parser (one AST; statements, expressions with GM 5's precedence, begin/end, optional ';' and 'then', with/repeat/until).
- 2026-11-03 — GitHub Pages (Joop asked, with explicit permission for the gh changes): served from main:/docs (GitHub renders the Markdown); docs/index.md is the progress page (step table, sample games, decisions) — RULE: every tick that moves a step updates its row. Later: a wasm playground there (lexer / parser on typed GML, then games).
