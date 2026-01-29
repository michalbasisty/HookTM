package replay

import (
	"testing"
)

func TestLooksLikeJSON(t *testing.T) {
	if !looksLikeJSON([]byte(` {"a":1}`)) {
		t.Fatalf("expected true")
	}
	if looksLikeJSON([]byte(`hello`)) {
		t.Fatalf("expected false")
	}
}
