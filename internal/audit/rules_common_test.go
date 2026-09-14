package audit

import (
	"reflect"
	"testing"
)

func TestEncodeDecodeStrings(t *testing.T) {
	cases := []struct {
		name string
		in   []string
	}{
		{"values", []string{"a", "b"}},
		{"empty", []string{}},
		{"nil", nil},
	}
	for _, c := range cases {
		got := decodeStrings(encodeStrings(c.in))
		if len(got) != len(c.in) {
			t.Errorf("%s: round-trip changed length: %#v", c.name, got)
			continue
		}
		for i := range c.in {
			if got[i] != c.in[i] {
				t.Errorf("%s: round-trip mismatch at %d: %q != %q", c.name, i, got[i], c.in[i])
			}
		}
	}
}

func TestDecodeStringsToleratesJunk(t *testing.T) {
	// Patterns and fields live in a TEXT column; a legacy or hand-edited row
	// must degrade to "no values" rather than break rule loading.
	for _, in := range []string{"", "not json", "null", "[1,2]", `{"a":1}`} {
		if got := decodeStrings(in); len(got) != 0 {
			t.Errorf("decodeStrings(%q) = %#v, want empty", in, got)
		}
	}
}

func TestEncodeStringsAlwaysProducesValidJSON(t *testing.T) {
	for _, in := range [][]string{nil, {}, {"a"}} {
		got := encodeStrings(in)
		if got == "" {
			t.Fatalf("encodeStrings(%#v) returned an empty string", in)
		}
		if decoded := decodeStrings(got); !reflect.DeepEqual(decoded, []string{}) && len(decoded) != len(in) {
			t.Errorf("encodeStrings(%#v) produced unparseable JSON: %q", in, got)
		}
	}
}
