package main

import "testing"

func TestParseKeys(t *testing.T) {
	sp, err := parseKeys("right:40, right+up:25,:10")
	if err != nil {
		t.Fatal(err)
	}
	if len(sp) != 3 || sp[0].steps != 40 || len(sp[1].keys) != 2 || sp[1].keys[1] != 38 || sp[2].keys != nil || sp[2].steps != 10 {
		t.Errorf("%+v", sp)
	}
	for _, bad := range []string{"jump:3", "right", "right:-1"} {
		if _, err := parseKeys(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
