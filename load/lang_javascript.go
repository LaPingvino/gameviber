//go:build !gv_nojs

package load

// Event code in JavaScript (.js files). A build tagged gv_nojs leaves it out: gameviber web
// does so for a project with no such files, which keeps the browser player
// smaller.
import _ "github.com/LaPingvino/gameviber/lang/js"
