package names

import "testing"

func TestBlacklisted(t *testing.T) {
	n, err := New()
	if err != nil {
		t.Fatal(err)
	}
	n.tree.Add(ReverseString("google.com"))
	if !n.isBlacklisted("google.com") {
		t.Fatal("should be blacklisted")
	}
}
