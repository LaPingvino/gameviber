package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LaPingvino/gameviber/sfx"
)

// cmdSfx is `gameviber sfx [-seed N] PRESET NAME DIR` (LOOP item 9): a sound
// effect of a preset kind (coin, laser, explosion, powerup, hit, jump, blip),
// random but repeatable by its seed, written as DIR/sounds/NAME.wav.
func cmdSfx(args []string) error {
	fs := flag.NewFlagSet("sfx", flag.ContinueOnError)
	seed := fs.Uint64("seed", 1, "which sound of the kind (the same seed, the same sound)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 3 || !slices.Contains(sfx.Presets, fs.Arg(0)) {
		return fmt.Errorf("usage: gameviber sfx [-seed N] PRESET NAME DIR   (PRESET: %s)", strings.Join(sfx.Presets, ", "))
	}
	name, dir := fs.Arg(1), fs.Arg(2)
	if !safeName(name) {
		return fmt.Errorf("a sound name is letters, digits, - and _")
	}
	if _, err := os.Stat(filepath.Join(dir, "game.toml")); err != nil {
		return fmt.Errorf("%s is not a GameViber project (no game.toml)", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sounds"), 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "sounds", name+".wav")
	samples := sfx.Generate(sfx.Preset(fs.Arg(0), *seed))
	if err := os.WriteFile(path, sfx.WAV(samples), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %s sound, %.2f s (seed %d)\n", path, fs.Arg(0), float64(len(samples))/44100, *seed)
	return nil
}
