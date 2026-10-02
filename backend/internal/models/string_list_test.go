package models

import (
	"reflect"
	"testing"
)

// PostgreSQL arrays can contain quoted delimiters and escaped values. These
// fixtures exercise the actual driver representation rather than comma splits.
func TestStringListDatabaseRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input any
		want  StringList
	}{
		{"postgres quoted", `{"read:A,B","https://app.example/a","quote\"value","back\\slash",""}`, StringList{"read:A,B", "https://app.example/a", "quote\"value", "back\\slash", ""}},
		{"postgres empty", `{}`, StringList{}},
		{"json bytes", []byte(`["read:A,B","write:*"]`), StringList{"read:A,B", "write:*"}},
		{"null", nil, StringList{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got StringList
			if err := got.Scan(tc.input); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
	for _, input := range []any{`{"unterminated}`, `not-json`, 42} {
		var got StringList
		if err := got.Scan(input); err == nil {
			t.Fatalf("accepted malformed representation %v", input)
		}
	}
}
