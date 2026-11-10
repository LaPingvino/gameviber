package js

import (
	"reflect"
	"testing"
)

func TestCalls(t *testing.T) {
	src := `instance_destroy();
var r = Math.floor(random(10)) + parseInt("3");
function helper(n) { return n * 2; }
helper(r);
const twice = (x) => helper(x);
twice(2);
other.instance_destroy();
no_such_thing(1);
argument0;`
	got, err := Calls("objects/o/step.js", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []Call{{"instance_destroy", 1, 1}, {"random", 2, 20}, {"no_such_thing", 8, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if _, err := Calls("x.js", "if ("); err == nil {
		t.Error("a syntax error went unnoticed")
	}
}
