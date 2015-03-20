package tools

import (
	"testing"
	"errors"
)

func checkErr(t *testing.T, err error, value string, comps ...string) {
	if err.Error() != value {
		t.Errorf("Value mismatch: %s vs %s", value, err.Error())
	}
}

func TestComponents(t *testing.T) {
	if NewError(nil, "foo") != nil {
		t.Errorf("Incorrect nil handling")
	}

	checkErr(t, NewError(errors.New("foo"), "foo1"), "foo1: foo", "foo1")
	checkErr(t, NewError(errors.New("foo"), "foo1", "foo2"), "foo1/foo2: foo", "foo1", "foo2")
	checkErr(t, NewError(NewError(errors.New("foo"), "test1", "test2"), "test0", "test1"), "test0/test1/test2: foo", "test0", "test1", "test2")
	if err := NewError(errors.New("palevo"), "p1", "p2"); !HasAnnotation(err, "p1") {
		t.Errorf("Can't find annotation p2 in %s", err.Error())
	}
}
