package domain

import "testing"

func TestIsValidUserPhoneNumber(t *testing.T) {
	for phone, want := range map[string]bool{
		"+998901234567":    true,
		"+79161234567":     true,
		"+12025550123":     true,
		"+4915123456789":   true,
		"+99890123456":     false, // Uzbek with 8 digits
		"+9989012345678":   false, // Uzbek with 10 digits
		"998901234567":     false, // no +
		"+0123456789":      false, // country code cannot start with 0
		"+123456":          false, // too short
		"+123456789012345": false, // 15 digits do not fit VARCHAR(15)
		"+998 90 1234567":  false,
		"":                 false,
	} {
		if got := IsValidUserPhoneNumber(phone); got != want {
			t.Errorf("IsValidUserPhoneNumber(%q) = %v, want %v", phone, got, want)
		}
	}
}
