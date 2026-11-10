package lua

import (
	"reflect"
	"testing"
)

func TestCalls(t *testing.T) {
	src := `instance_destroy()
local r = math.floor(random(10)) + tonumber("3")
local function helper(n) return n * 2 end
function twice(n) return helper(n) end
helper(twice(r))
cb = function() end
cb()
for i, v in ipairs({}) do print(i, v) end
other:instance_destroy()
no_such_thing(1)`
	got, err := Calls("objects/o/step.lua", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []Call{{"instance_destroy", 1}, {"random", 2}, {"no_such_thing", 10}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if _, err := Calls("x.lua", "if then"); err == nil {
		t.Error("a syntax error went unnoticed")
	}
}
