//go:build !gv_nolua

package load

// Event code in Lua (.lua files). A build tagged gv_nolua leaves it out: gameviber web
// does so for a project with no such files, which keeps the browser player
// smaller.
import _ "github.com/LaPingvino/gameviber/lang/lua"
