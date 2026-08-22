package provider

import (
	"strings"
	"testing"
)

func TestGeneratePasswordHonorsLengthAndMinimums(t *testing.T) {
	t.Parallel()
	password, err := generatePassword(passwordParameters{
		Length: 64, Lower: true, Upper: true, Numeric: true, Special: true,
		MinLower: 3, MinUpper: 4, MinNumeric: 5, MinSpecial: 6, SpecialChars: "!%",
	})
	if err != nil {
		t.Fatalf("generate password: %v", err)
	}
	if len(password) != 64 {
		t.Fatalf("unexpected password length: %d", len(password))
	}
	counts := map[string]int{}
	for _, character := range password {
		switch {
		case strings.ContainsRune(lowerCharacters, character):
			counts["lower"]++
		case strings.ContainsRune(upperCharacters, character):
			counts["upper"]++
		case strings.ContainsRune(numericCharacters, character):
			counts["numeric"]++
		case strings.ContainsRune("!%", character):
			counts["special"]++
		default:
			t.Fatalf("unexpected character %q", character)
		}
	}
	if counts["lower"] < 3 || counts["upper"] < 4 || counts["numeric"] < 5 || counts["special"] < 6 {
		t.Fatalf("minimum counts not met: %#v", counts)
	}
}

func TestGeneratePasswordRejectsInvalidSettings(t *testing.T) {
	t.Parallel()
	if _, err := generatePassword(passwordParameters{Length: 2, Lower: true, MinLower: 3}); err == nil {
		t.Fatal("expected minimum length error")
	}
	if _, err := generatePassword(passwordParameters{Length: 10}); err == nil {
		t.Fatal("expected disabled character classes error")
	}
}
