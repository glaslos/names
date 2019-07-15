package names

import "testing"

func TestBlacklisted(t *testing.T) {
	n := New()
	n.tree.Add(ReverseString("google.com"))
	if !n.isBlacklisted("google.com") {
		t.Fatal("should be blacklisted")
	}
}

