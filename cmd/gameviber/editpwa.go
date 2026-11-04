package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The Pages editor as an installable web app (LOOP item 5i): a manifest, two
// icons and a service worker written next to it by editor-web. The worker
// answers from the network first and keeps a copy for when there is none, so
// a new deploy always wins and the editor still opens offline.

// pwaManifest is a web app manifest: the name, how it opens (standalone for
// the editor, fullscreen for a game), the colours and the two icons.
func pwaManifest(name, short, description, display, background, theme string) []byte {
	m := map[string]any{"name": name, "short_name": short, "description": description, "start_url": "./", "scope": "./",
		"display": display, "background_color": background, "theme_color": theme,
		"icons": []map[string]string{{"src": "icon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "any maskable"}}}
	if display == "standalone" { // the editor opens a project's game.toml from the file manager
		m["file_handlers"] = []map[string]any{{"action": "./", "accept": map[string][]string{"application/toml": {".toml"}}}}
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return append(b, '\n')
}

// pwaWorker is the service worker: network first (a new deploy always wins),
// the last copy of each file kept for offline use, in the named cache.
func pwaWorker(cache string) []byte {
	return []byte(strings.Replace(pwaWorkerJS, "CACHE_NAME", strconv.Quote(cache), 1))
}

const pwaWorkerJS = `// GameViber's service worker: network first (a new deploy always wins),
// the last copy of each file kept for offline use.
const CACHE = CACHE_NAME;
self.addEventListener("install", (e) => self.skipWaiting());
self.addEventListener("activate", (e) => e.waitUntil(self.clients.claim()));
self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET" || new URL(req.url).origin !== location.origin) return;
  e.respondWith(fetch(req, { cache: "no-cache" }).then((resp) => {
    if (resp.ok) { const copy = resp.clone(); caches.open(CACHE).then((c) => c.put(req, copy)); }
    return resp;
  }).catch(() => caches.match(req).then((hit) => hit || new Response("offline", { status: 503 }))));
});
`

// pwaRegister goes in the page's head: the manifest, the theme colour, and
// the worker registered where browsers allow it (https, localhost).
const pwaRegister = `<link rel="manifest" href="manifest.webmanifest"><meta name="theme-color" content="#c2541b">
<script>if ("serviceWorker" in navigator && (location.protocol === "https:" || location.hostname === "localhost" || location.hostname === "127.0.0.1")) navigator.serviceWorker.register("sw.js").catch(() => {});</script>
`

// writePWA writes the editor's manifest, worker and icons into out.
func writePWA(out string) error {
	return writePWAFiles(out, pwaManifest("GameViber editor", "GameViber", "Make GameMaker-style games as plain text files — with your coding agent.",
		"standalone", "#f5f1e8", "#c2541b"), "gameviber-editor")
}

// writePWAFiles writes a manifest, the worker (its cache named) and the icons into out.
func writePWAFiles(out string, manifest []byte, cache string) error {
	files := map[string][]byte{"manifest.webmanifest": manifest, "sw.js": pwaWorker(cache)}
	for _, n := range []int{192, 512} {
		var b bytes.Buffer
		if err := png.Encode(&b, pwaIcon(n)); err != nil {
			return err
		}
		files[map[int]string{192: "icon-192.png", 512: "icon-512.png"}[n]] = b.Bytes()
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// pwaIcon is GameViber's icon: the editor's orange, a white play triangle
// (with room around it for maskable crops).
func pwaIcon(n int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	orange, white := color.RGBA{0xc2, 0x54, 0x1b, 0xff}, color.RGBA{0xff, 0xfd, 0xf8, 0xff}
	f := float64(n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			img.Set(x, y, orange)
			// a triangle pointing right: x from 0.38 to 0.70, its height shrinking with x
			fx, fy := float64(x)/f, float64(y)/f
			if fx >= 0.38 && fx <= 0.70 {
				half := 0.20 * (0.70 - fx) / 0.32
				if fy >= 0.5-half && fy <= 0.5+half {
					img.Set(x, y, white)
				}
			}
		}
	}
	return img
}
