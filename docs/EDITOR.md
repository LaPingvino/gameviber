# GameViber editor — proposal (building since 2026-11-04 on Joop's "decide, correct later")

DESIGN.md §6 left a GUI editor out of the first version ("later, a web editor on the same engine"). The first
version now runs both sample games from their 2006 data, so this is the proposal for the next step. It is
being built with the choices under "Decided for now"; every point below is open to reshaping.

## What it is for

The old GameMaker feeling — open a game, click an object, edit its events, place things in a room, press Play —
without leaving the plain-text project behind. The editor edits the same files `gameviber new` and
`gameviber import` write (TOML, one `.gml` per event, rooms as text), so an agent, a text editor, git and the
GUI all work on one project, at the same time.

## Shape (a first version)

1. **A web page first**, built like the demo (`gameviber web`): the editor is Go compiled to WebAssembly, the
   same engine that plays the game. It runs on the Chromebook and phones as well as the desktop; the desktop
   gets it through the same page served locally by `gameviber edit DIR` (which also reads and writes the files).
2. **Panels** (LateralGM's layout, slimmed):
   - the resource tree: sprites, sounds, backgrounds, objects, rooms, scripts, timelines;
   - an object: its settings (sprite, solid, visible, depth, parent, persistent) and its events, each a GML text
     box with the line numbers `gameviber check` uses — errors and its hints shown under the box as you type;
   - a room: the grid (as Stelo's level editor: pick an object, click to place, right-click to remove), its
     settings (size, speed, caption, background, views), the creation code;
   - sprites and backgrounds: view, origin, bounding box, precise; import a PNG (drawing pixels later);
   - **Play**: the room or the game runs in the panel, with the step / pause controls of `gameviber play`.
3. **Drawn with the engine** (Ebitengine, the text overlay and menu widget Stelo and Pacmanio already have)
   or **HTML around a game canvas** — see question 2.
4. **Agents inside, later**: the same commands (`check`, `play -json`, `reference`) are what an assistant panel
   would call; DESIGN's "an MCP server over the same commands" fits here too. Not in the first version.

## Steps (each a few loop ticks, each usable on its own)

E1. `gameviber edit DIR`: serves the project and the editor page; the resource tree; objects with their event
    texts, saved back to the files; `check` on every save.
E2. Rooms: the grid, placing and removing instances, room settings.
E3. Play inside the editor (room or game), with step / pause.
E4. Sprites and backgrounds: view, origin, box, PNG import.
E5. (later) Drawing pixels; sounds; timelines; the assistant panel.

## Decided for now (2026-11-04) — say otherwise and it changes

1. Web first: `gameviber edit DIR` serves the editor on this machine (127.0.0.1) and edits the files.
2. HTML around the editing (native text boxes, copy and paste); the game drawn by the engine inside it (E3).
3. E1–E3 as the first slice. E1 is built: the resource tree, every text file opened and saved,
   `gameviber check` after each save shown under the editor.

## Questions for Joop (as first asked)

1. Web first (one editor for the desktop, the Chromebook and phones; the desktop via a local page) — or a
   desktop window first?
2. Drawn with the engine (one look with the games, Stelo's widgets, works the same everywhere; text needs the
   font rasteriser — golang.org/x/image, still an open question) — or HTML / CSS around the game canvas
   (native text boxes, selection and copy-paste come free; a second UI technology to keep)?
3. Is E1–E3 the right first slice, or should rooms come before events?
