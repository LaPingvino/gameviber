package runner

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/LaPingvino/gameviber/gml"
)

// HTTP from GML, asynchronous, for a game that lives inside a web site
// (GameMaker Studio's model, with GameMaker 5-style functions instead of its
// ds_map): http_get(url) and http_post(url, name, value, …) start a request
// and return its id at once; when the answer is in, the "Async HTTP" event
// (other-62.gml, Studio's number) runs for every instance at the start of a
// step, where http_id(), http_status() and http_result() describe it. Status
// 0 means no answer (offline, refused): the game decides whether to retry.
//
// The transport is the backend's (Game.HTTP: in the browser a same-origin
// fetch, the page's cookies with it). Without one — the desktop, headless
// play — it is net/http with a cookie jar of its own, relative addresses
// taken against BaseURL.

// HTTPRequest is one request as the transport gets it.
type HTTPRequest struct {
	ID          int
	Method, URL string
	Body        string // a form (application/x-www-form-urlencoded) for POST
}

// HTTPResponse is its answer: Status 0 when there was none.
type HTTPResponse struct {
	ID, Status int
	Body       string
}

type httpState struct {
	// HTTP sends a request and calls done with the answer, from any
	// goroutine (the backend's); nil: net/http (see above).
	HTTP func(req HTTPRequest, done func(HTTPResponse))
	// BaseURL is the address relative requests are taken against without a
	// page, and page_url's answer then (gameviber play -base).
	BaseURL string
	// HTTPWait makes each step wait for the answers already asked for, so a
	// headless run is the same every time (gameviber play).
	HTTPWait bool

	httpMu      sync.Mutex
	httpNext    int
	httpDone    []HTTPResponse
	httpPending sync.WaitGroup
	httpCur     HTTPResponse
	httpClient  *http.Client
}

func (g *Game) pageURL() string {
	if g.PageURL != nil {
		if u := g.PageURL(); u != "" {
			return u
		}
	}
	return g.BaseURL
}

// startHTTP sends a request; its answer is queued for DeliverHTTP.
func (g *Game) startHTTP(method, u, body string) int {
	g.httpMu.Lock()
	g.httpNext++
	id := g.httpNext
	g.httpMu.Unlock()
	req := HTTPRequest{ID: id, Method: method, URL: u, Body: body}
	g.httpPending.Add(1)
	done := func(r HTTPResponse) {
		r.ID = id
		g.httpMu.Lock()
		g.httpDone = append(g.httpDone, r)
		g.httpMu.Unlock()
		g.httpPending.Done()
	}
	if g.HTTP != nil {
		g.HTTP(req, done)
		return id
	}
	go func() { done(g.netHTTP(req)) }()
	return id
}

// netHTTP is the transport without a backend's: net/http.
func (g *Game) netHTTP(req HTTPRequest) HTTPResponse {
	g.httpMu.Lock()
	if g.httpClient == nil {
		jar, _ := cookiejar.New(nil)
		g.httpClient = &http.Client{Jar: jar, Timeout: 20 * time.Second}
	}
	c := g.httpClient
	g.httpMu.Unlock()
	u, err := url.Parse(req.URL)
	if err != nil {
		return HTTPResponse{}
	}
	if !u.IsAbs() {
		base, err := url.Parse(g.BaseURL)
		if err != nil || !base.IsAbs() {
			return HTTPResponse{} // no page to be relative to
		}
		u = base.ResolveReference(u)
	}
	hr, err := http.NewRequest(req.Method, u.String(), strings.NewReader(req.Body))
	if err != nil {
		return HTTPResponse{}
	}
	if req.Method == "POST" {
		hr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.Do(hr)
	if err != nil {
		return HTTPResponse{}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return HTTPResponse{Status: resp.StatusCode, Body: string(b)}
}

// DeliverHTTP runs the Async HTTP event (other-62) for each answer that has
// come in, in the order they came; called at the start of a step.
func (g *Game) DeliverHTTP(r *Room) {
	if g.HTTPWait {
		g.httpPending.Wait()
	}
	g.httpMu.Lock()
	done := g.httpDone
	g.httpDone = nil
	g.httpMu.Unlock()
	for _, resp := range done {
		g.httpCur = resp
		r.Every(func(e *Events) Handler { return e.Other[62] })
	}
	g.httpCur = HTTPResponse{}
}

func (g *Game) httpFuncs() map[string]gml.Func {
	str := func(a []gml.Value, i int) string {
		if i < len(a) {
			return a[i].String()
		}
		return ""
	}
	return map[string]gml.Func{
		"page_url": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (): the page's address in the browser (headless: -base); "" elsewhere
			return gml.StrV(g.pageURL()), nil
		},
		"http_get": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (url): starts a request; its id (the answer: other-62)
			if str(a, 0) == "" {
				return gml.RealV(-1), nil
			}
			return gml.RealV(float64(g.startHTTP("GET", str(a, 0), ""))), nil
		},
		"http_post": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (url, name, value, name, value …): posts a form; its id (the answer: other-62)
			if str(a, 0) == "" {
				return gml.RealV(-1), nil
			}
			form := url.Values{}
			for i := 1; i+1 < len(a); i += 2 {
				form.Add(a[i].String(), a[i+1].String())
			}
			return gml.RealV(float64(g.startHTTP("POST", str(a, 0), form.Encode()))), nil
		},
		"http_id": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (): in other-62, the answered request's id
			return gml.RealV(float64(g.httpCur.ID)), nil
		},
		"http_status": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (): in other-62, its HTTP status (0: no answer)
			return gml.RealV(float64(g.httpCur.Status)), nil
		},
		"http_result": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (): in other-62, the answer's text
			return gml.StrV(g.httpCur.Body), nil
		},
	}
}
