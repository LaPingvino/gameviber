---
title: GameViber — your first game
---

# Your first game, in about ten minutes

Everything here happens in your browser, in [GameViber's editor](editor/): no install, no account. Your work stays
in this browser until you start a new game.

## 1. Pick a starting point

Open **[the editor](editor/)** and press **New game…** at the top. Choose one of the six:

| Starting point | What it is |
|---|---|
| Collect the coins | move with the arrow keys through a walled room and take every coin |
| Collect the coins, in JavaScript | the same game, its player's code in JavaScript instead of GML |
| Collect the coins, in Lua | the same game, its player's code in Lua |
| Platformer | run and jump with gravity, collect the coins, touch the flag for the next level |
| Gem hunt (top-down) | collect gems among wall blocks while ghosts wander and chase you; three lives |
| Star defence (shooter) | fire at saucers that come down faster and faster; three lives |

This tutorial uses the **Platformer**. Press **▶ Play** and try it: the arrow keys run, **up** jumps.

## 2. Change how it plays

On the left is the game's files. Open `objects/player/step.gml`: it is the player's *step* event, the code that runs
every frame. Find the jump:

```
if keyboard_check(vk_up) && !place_free(x, y + 1) vsp = -15
```

`vsp` is the vertical speed; the jump sets it upwards. Make it `-19`, press **Save (Ctrl+S)** and **▶ Play** again:
the player now jumps much higher. Each save is checked — a typing mistake shows at the bottom with its line.

A few more things to try:

- `x -= 4` and `x += 4` are the running speed;
- `if vsp > 10 vsp = 10` is the fastest the player falls;
- in `objects/player/collision-coin.gml`, `score += 10` is what a coin is worth.

## 3. Change the level

Open `rooms/level1`. The room is drawn with everything in it: **click** to place the chosen object (a wall, a coin …),
**right click** to remove one. Put a few coins somewhere hard to reach, and play it.

## 4. Work on it with an assistant

GameViber is made to work with any AI chat or coding agent. Press **Chat session**: the editor writes a first
message — what GameViber is, your game's files, how to ask for changes. Copy it into any chat (Claude, ChatGPT,
Gemini, a local model …) with a request of your own, for example:

> Add a third level with a long gap and more coins, and make each coin worth 50.

Paste the chat's whole reply back into the editor and press **Read the reply**: you see which files it wants to write
and what it wants to run (a check, a test play with keys, a look at a frame). **Run it** does that, and gives you the
next message — the results, plus anything you changed yourself meanwhile — to copy back to the chat. Go on until the
chat says it is done.

With an API key (Anthropic, any OpenAI-compatible service, or Ollama on your own computer for free), **Send with my
key** does the copying for you, and **auto, up to N turns** lets the session go on by itself — it stops when the chat
is done, at the turn limit, or when something keeps failing, and asks you before writing files unless you allow it.

## 5. Take it home

**Download** (at the top of the editor) saves the whole game as a zip: keep it, share it, or unpack it and keep
working on it on your own computer. **New game… → Open a downloaded game (.zip)** brings one back into the
editor — yours, or one someone shared with you. With [Go](https://go.dev/dl/) installed:

```
go install github.com/LaPingvino/gameviber/cmd/gameviber@latest
go install github.com/LaPingvino/gameviber/cmd/gameviber-run@latest   # the game window
gameviber new -template platformer mygame   # the same starting points
gameviber edit mygame                       # this editor, on your files
gameviber run mygame                        # play it in a window
gameviber export mygame mygame.zip          # a web build to put online (itch.io takes the zip)
```

On disk, a coding agent in your terminal (Claude Code, OpenCode, Gemini CLI …) can work on the same folder while
the editor follows its changes — see [With an assistant](./#with-an-assistant).
