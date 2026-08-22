package provider

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

const (
	lowerCharacters   = "abcdefghijklmnopqrstuvwxyz"
	upperCharacters   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numericCharacters = "0123456789"
	defaultSpecial    = "!@#$%&*()-_=+[]{}<>:?"
)

type passwordParameters struct {
	Length       int
	Lower        bool
	Upper        bool
	Numeric      bool
	Special      bool
	MinLower     int
	MinUpper     int
	MinNumeric   int
	MinSpecial   int
	SpecialChars string
}

func generatePassword(parameters passwordParameters) (string, error) {
	if parameters.Length < 1 {
		return "", errors.New("length must be at least 1")
	}
	minimumLength := parameters.MinLower + parameters.MinUpper + parameters.MinNumeric + parameters.MinSpecial
	if minimumLength > parameters.Length {
		return "", fmt.Errorf("length must be at least the sum of all minimum character counts (%d)", minimumLength)
	}

	allowed := ""
	result := make([]byte, 0, parameters.Length)
	classes := []struct {
		enabled bool
		minimum int
		chars   string
		name    string
	}{
		{parameters.Lower, parameters.MinLower, lowerCharacters, "lower"},
		{parameters.Upper, parameters.MinUpper, upperCharacters, "upper"},
		{parameters.Numeric, parameters.MinNumeric, numericCharacters, "numeric"},
		{parameters.Special, parameters.MinSpecial, parameters.SpecialChars, "special"},
	}
	for _, class := range classes {
		if class.minimum < 0 {
			return "", fmt.Errorf("min_%s cannot be negative", class.name)
		}
		if !class.enabled && class.minimum > 0 {
			return "", fmt.Errorf("min_%s cannot be positive when %s is disabled", class.name, class.name)
		}
		if !class.enabled {
			continue
		}
		if class.chars == "" {
			return "", fmt.Errorf("%s character set cannot be empty", class.name)
		}
		allowed += class.chars
		for range class.minimum {
			character, err := randomCharacter(class.chars)
			if err != nil {
				return "", err
			}
			result = append(result, character)
		}
	}
	if allowed == "" {
		return "", errors.New("at least one character class must be enabled")
	}
	for len(result) < parameters.Length {
		character, err := randomCharacter(allowed)
		if err != nil {
			return "", err
		}
		result = append(result, character)
	}
	for index := len(result) - 1; index > 0; index-- {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(index+1)))
		if err != nil {
			return "", fmt.Errorf("shuffle generated password: %w", err)
		}
		other := int(randomIndex.Int64())
		result[index], result[other] = result[other], result[index]
	}
	return string(result), nil
}

func randomCharacter(characters string) (byte, error) {
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
	if err != nil {
		return 0, fmt.Errorf("generate random password: %w", err)
	}
	return characters[index.Int64()], nil
}
