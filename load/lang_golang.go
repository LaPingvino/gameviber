//go:build !gv_nogo

package load

// Event code in Go (.go files), interpreted by yaegi where not compiled in. A build tagged gv_nogo leaves it out: gameviber web
// does so for a project with no such files, which keeps the browser player
// smaller.
import _ "github.com/LaPingvino/gameviber/lang/golang"
