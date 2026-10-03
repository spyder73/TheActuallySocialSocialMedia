package config

import "testing"

func TestOrigin(t *testing.T) {
	for _, tc := range []struct {
		origin       string
		local, valid bool
	}{
		{"https://social.example.com", false, true}, {"http://localhost:18080", true, true},
		{"http://social.example.com", true, false}, {"https://social.example.com/", false, false},
		{"https://user:pass@social.example.com", false, false}, {"http://localhost:18080", false, false},
		{"", false, false}, {"https://social.example.com?x=y", false, false},
	} {
		if err := ValidateOrigin(tc.origin, tc.local); (err == nil) != tc.valid {
			t.Errorf("%q local=%v: %v", tc.origin, tc.local, err)
		}
	}
}
