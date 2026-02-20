package cliutil

import "testing"

func TestPointerHelpers(t *testing.T) {
	if PointerToString(nil) != "" {
		t.Fatal("expected empty string for nil pointer")
	}
	if PointerToInt32(nil) != 0 {
		t.Fatal("expected zero for nil int pointer")
	}

	s := "abc"
	n := int32(9)
	if PointerToString(&s) != "abc" {
		t.Fatal("unexpected PointerToString value")
	}
	if PointerToInt32(&n) != 9 {
		t.Fatal("unexpected PointerToInt32 value")
	}
	if *Ptr("x") != "x" {
		t.Fatal("unexpected Ptr helper value")
	}
}
