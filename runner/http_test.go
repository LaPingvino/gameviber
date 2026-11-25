package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// http_post / http_get start requests and return at once; the answers come
// in other-62 at the start of a later step, with http_id / http_status /
// http_result. Relative addresses go against BaseURL; cookies the server
// sets come back (as a browser page's would); no answer is status 0.
func TestAsyncHTTP(t *testing.T) {
	seen := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		c, _ := r.Cookie("token")
		if c == nil {
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "t1", Path: "/"})
		}
		r.ParseForm()
		fmt.Fprintf(w, "%s %s a=%s cookie=%v", r.Method, r.URL.Path, r.Form.Get("a"), c != nil)
	}))
	defer srv.Close()
	o := &Object{Name: "o"}
	g := &Game{Objects: []*Object{o}}
	g.BaseURL = srv.URL + "/ludo/"
	g.HTTPWait = true
	create, err := g.Code(`p = http_post("../provo", "a", "ĉ & b"); q = http_get("stato"); n = 0; page = page_url()`)
	if err != nil {
		t.Fatal(err)
	}
	async, err := g.Code(`n += 1; if (http_id() == p) rp = string(http_status()) + " " + http_result(); if (http_id() == q) rq = http_result()`)
	if err != nil {
		t.Fatal(err)
	}
	o.Events.Other = map[int]Handler{62: async}
	r := &Room{W: 10, H: 10, G: &Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 0, 0)
	create(r, in)
	if in.Vars["n"] != 0 {
		t.Fatal("an answer before the next step")
	}
	g.DeliverHTTP(r)
	if len(g.Errors) > 0 {
		t.Fatal(g.Errors)
	}
	if in.Vars["n"] != 2 || in.VarsStr["rp"] != "200 POST /provo a=ĉ & b cookie=false" || !strings.HasPrefix(in.VarsStr["rq"], "GET /ludo/stato a= cookie=") {
		t.Errorf("n %v, post %q, get %q", in.Vars["n"], in.VarsStr["rp"], in.VarsStr["rq"])
	}
	if in.VarsStr["page"] != srv.URL+"/ludo/" {
		t.Errorf("page_url %q", in.VarsStr["page"])
	}
	// the cookie set by the first answer goes with the next request
	again, _ := g.Code(`q = http_get("stato")`)
	again(r, in)
	g.DeliverHTTP(r)
	if in.VarsStr["rq"] != "GET /ludo/stato a= cookie=true" {
		t.Errorf("cookie not kept: %q", in.VarsStr["rq"])
	}
	// nobody answering: status 0
	srv.Close()
	off, _ := g.Code(`p = http_post("../provo", "a", "x")`)
	off(r, in)
	g.DeliverHTTP(r)
	if in.VarsStr["rp"] != "0 " {
		t.Errorf("offline: %q", in.VarsStr["rp"])
	}
}
