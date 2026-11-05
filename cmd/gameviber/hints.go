package main

import "sort"

// laterGM are functions from GameMaker 6, 7, 8 and Studio that agents (and
// people) reach for; GameViber's GML is 5.3's, so check says how to write
// them there.
var laterGM = map[string]string{
	"irandom":                    "GM 6+; in 5.3: floor(random(n + 1))",
	"irandom_range":              "GM 8+; in 5.3: a + floor(random(b - a + 1))",
	"random_range":               "GM 8+; in 5.3: a + random(b - a)",
	"clamp":                      "GM Studio; in 5.3: min(max(x, lo), hi)",
	"lerp":                       "GM Studio; in 5.3: a + (b - a) * t",
	"point_in_rectangle":         "GM Studio; in 5.3: x >= x1 and x <= x2 and y >= y1 and y <= y2",
	"draw_set_color":             "GM 6+; in 5.3: brush_color = c (fills) and pen_color = c (lines), draw_set_font's colour is font_color",
	"draw_set_colour":            "GM 6+; in 5.3: brush_color = c and pen_color = c",
	"draw_set_font":              "GM 6+; in 5.3: font_name, font_size, font_color, font_style",
	"draw_set_alpha":             "GM 6+; 5.3 draws without alpha",
	"draw_self":                  "GM 8+; in 5.3: draw_sprite(sprite_index, image_index, x, y)",
	"make_color_rgb":             "GM 6+; in 5.3: make_color(r, g, b)",
	"make_colour_rgb":            "GM 6+; in 5.3: make_color(r, g, b)",
	"keyboard_check_pressed":     "GM 6+; in 5.3: a key press event (keypress-<code>.gml)",
	"keyboard_check_released":    "GM 6+; in 5.3: a key release event (keyrelease-<code>.gml)",
	"mouse_check_button":         "GM 6+; in 5.3: mouse_button (the button held) or a mouse event (mouse-<n>.gml)",
	"mouse_check_button_pressed": "GM 6+; in 5.3: a mouse event (mouse-4.gml … left pressed on the instance, mouse-53.gml anywhere)",
	"instance_create_layer":      "GM Studio 2; in 5.3: instance_create(x, y, object)",
	"instance_create_depth":      "GM Studio 2; in 5.3: instance_create(x, y, object), then set depth",
	"audio_play_sound":           "GM Studio; in 5.3: sound_play(snd) or sound_loop(snd)",
	"audio_stop_sound":           "GM Studio; in 5.3: sound_stop(snd)",
	"array_length":               "GM Studio 2.3; 5.3 has no array length — keep a count variable",
	"ds_list_create":             "GM 6+ (data structures); 5.3 has arrays only",
	"ds_map_create":              "GM 6+ (data structures); 5.3 has arrays only",
	// Studio-era names models write most often (added 2026-11-05).
	"audio_is_playing":            "GM Studio; in 5.3: sound_isplaying(snd)",
	"mouse_check_button_released": "GM 6+; in 5.3: a mouse event (mouse-7.gml … released on the instance, mouse-56.gml anywhere)",
	"draw_set_halign":             "GM 6+; 5.3 draws text from its top-left: centre it yourself with x - string_width(text) / 2",
	"draw_set_valign":             "GM 6+; 5.3 draws text from its top-left: y - string_height(text) / 2 centres it",
	"draw_text_color":             "GM 6+; in 5.3: font_color = c; draw_text(x, y, text)",
	"draw_text_colour":            "GM 6+; in 5.3: font_color = c; draw_text(x, y, text)",
	"draw_sprite_ext":             "GM 6+; in 5.3: draw_sprite(spr, img, x, y), or draw_sprite_stretched(spr, img, x, y, w, h) for a size (no rotation or blending)",
	"draw_rectangle_color":        "GM 6+; in 5.3: brush_color = c (fill) and pen_color = c (outline), then draw_rectangle(x1, y1, x2, y2)",
	"draw_rectangle_colour":       "GM 6+; in 5.3: brush_color = c and pen_color = c, then draw_rectangle(x1, y1, x2, y2)",
	"ds_list_add":                 "GM 6+ (data structures); in 5.3 use an array and a count: list[n] = value; n += 1",
	"window_set_caption":          "GM 6+; in 5.3: room_caption = \"…\"",
	"layer_get_id":                "GM Studio 2 (layers); 5.3 orders drawing by each instance's depth (lower depth is drawn on top)",
	"array_create":                "GM Studio; 5.3 makes an array by assigning to it: a[0] = 0 (a[i, j] for two dimensions)",
	"alarm_set":                   "GM Studio; in 5.3: alarm[n] = steps",
	"sprite_get_width":            "GM 6+; GameViber's GML has no call for a sprite's size — its width is in sprites/<name>/sprite.toml; keep it in a variable",
	"sprite_get_height":           "GM 6+; GameViber's GML has no call for a sprite's size — its height is in sprites/<name>/sprite.toml; keep it in a variable",
	"game_set_speed":              "GM Studio 2; in 5.3: room_speed = steps per second",
}

// hint says how to fix an unknown function: its 5.3 form when it is from a
// later GameMaker, else the nearest name GameViber knows (a typo), else "".
func hint(fn string, known map[string]bool) string {
	if h, ok := laterGM[fn]; ok {
		return h
	}
	names := make([]string, 0, len(known))
	for k := range known {
		names = append(names, k)
	}
	sort.Strings(names) // the first of equally near names, every run alike
	best, bestD := "", 3
	for _, k := range names {
		if d := editDistance(fn, k); d < bestD {
			best, bestD = k, d
		}
	}
	if best != "" {
		return "did you mean " + best + "?"
	}
	return ""
}

// editDistance is Levenshtein's distance (insertions, deletions, changes).
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
