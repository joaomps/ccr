package plan

import (
	"reflect"
	"testing"
)

func TestBundleTestSibling(t *testing.T) {
	got := Bundle([]string{"a/x.go", "a/x_test.go"})
	want := [][]string{{"a/x.go", "a/x_test.go"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestBundleSeparateDirs(t *testing.T) {
	got := Bundle([]string{"a/x.go", "b/y.go"})
	if len(got) != 2 {
		t.Fatalf("expected 2 bundles, got %v", got)
	}
}

func TestBundleLocaleSibling(t *testing.T) {
	got := Bundle([]string{"i18n/message_en.properties", "i18n/message_zh.properties"})
	if len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("expected locale pair bundled, got %v", got)
	}
}

func TestBundleNoOverGroupingCode(t *testing.T) {
	// user_db.go and user_id.go must NOT be bundled (both end _xx but are code).
	got := Bundle([]string{"a/user_db.go", "a/user_id.go"})
	if len(got) != 2 {
		t.Fatalf("expected 2 bundles (no over-grouping), got %v", got)
	}
}

func TestBundleStableOrder(t *testing.T) {
	a := Bundle([]string{"b/y.go", "a/x.go", "a/x_test.go"})
	b := Bundle([]string{"a/x_test.go", "a/x.go", "b/y.go"})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("order not stable: %v vs %v", a, b)
	}
}
