package gmfile

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing/fstest"

	"github.com/BurntSushi/toml"
)

// Build variants (LOOP item 6b, Joop: other TOML files "would allow things
// like overlays for e.g. demo builds"): NAME.toml in the project's root holds
// only what differs from game.toml and is merged over it — tables key by key,
// everything else (a shorter room_order, a name, a constant such as DEMO = 1)
// replaced. game.toml stays the project's one entry point.

// MergeTOML is base with overlay merged over it, as TOML.
func MergeTOML(base, overlay []byte) ([]byte, error) {
	var b, o map[string]any
	if _, err := toml.Decode(string(base), &b); err != nil {
		return nil, fmt.Errorf("game.toml: %w", err)
	}
	if _, err := toml.Decode(string(overlay), &o); err != nil {
		return nil, fmt.Errorf("the variant: %w", err)
	}
	mergeTables(b, o)
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(b); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func mergeTables(dst, src map[string]any) {
	for k, v := range src {
		if sub, ok := v.(map[string]any); ok {
			if d, ok := dst[k].(map[string]any); ok {
				mergeTables(d, sub)
				continue
			}
		}
		dst[k] = v
	}
}

// VariantGameTOML is the project's game.toml with variant (NAME, for
// NAME.toml in the project's root) merged over it; "" is game.toml as it is.
func VariantGameTOML(dir, variant string) ([]byte, error) {
	base, err := os.ReadFile(path.Join(dir, "game.toml"))
	if err != nil || variant == "" {
		return base, err
	}
	overlay, err := os.ReadFile(path.Join(dir, variant+".toml"))
	if err != nil {
		return nil, fmt.Errorf("variant %q: %w (it is %s.toml next to game.toml)", variant, err, variant)
	}
	return MergeTOML(base, overlay)
}

// ReadDirVariant is ReadDir with a variant merged over game.toml.
func ReadDirVariant(dir, variant string) (*Project, error) {
	if variant == "" {
		return ReadDir(dir)
	}
	game, err := VariantGameTOML(dir, variant)
	if err != nil {
		return nil, err
	}
	return ReadFS(overlayFS{os.DirFS(dir), fstest.MapFS{"game.toml": {Data: game}}}, ".")
}

// overlayFS serves the files of top over those of base.
type overlayFS struct{ base, top fs.FS }

func (o overlayFS) Open(name string) (fs.File, error) {
	if f, err := o.top.Open(name); err == nil {
		if st, err := f.Stat(); err == nil && !st.IsDir() {
			return f, nil
		}
		f.Close()
	}
	return o.base.Open(name)
}
