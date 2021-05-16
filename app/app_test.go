package main

import "testing"

func TestVerifyAddr(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		doesErr bool
	}{
		{"ip no port", "127.0.0.1", true},
		{"just a string", "banana", true},
		{"good", "127.0.0.1:53", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := verifyAddr(test.host); (err == nil) == test.doesErr {
				t.Fatal(err)
			}
		})
	}
}
