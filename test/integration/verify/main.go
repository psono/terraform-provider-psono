package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"unicode"

	"gitlab.com/esaqa/psono/psono-terraform-provider/internal/psono"
)

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("expected operation: present, absent, hash, changed, or delete")
	}
	operation := os.Args[1]
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	secretID := flags.String("secret-id", "", "Psono Environment Variables entry ID")
	name := flags.String("name", "", "environment-variable name")
	expectedLength := flags.Int("length", 0, "expected value length")
	minLower := flags.Int("min-lower", 0, "minimum lowercase characters")
	minUpper := flags.Int("min-upper", 0, "minimum uppercase characters")
	minNumeric := flags.Int("min-numeric", 0, "minimum numeric characters")
	minSpecial := flags.Int("min-special", 0, "minimum special characters")
	previousHash := flags.String("previous-hash", "", "SHA-256 hash that the value must differ from")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *secretID == "" || *name == "" {
		return errors.New("--secret-id and --name are required")
	}

	client, err := psono.NewClient(psono.Credentials{
		ServerURL:         os.Getenv("PSONO_SERVER_URL"),
		APIKeyID:          os.Getenv("PSONO_API_KEY_ID"),
		APISecretKey:      os.Getenv("PSONO_API_SECRET_KEY"),
		CABundle:          []byte(os.Getenv("PSONO_CA_BUNDLE")),
		AllowInsecureHTTP: os.Getenv("PSONO_ALLOW_INSECURE_HTTP") == "true",
	})
	if err != nil {
		return fmt.Errorf("configure Psono client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	value, _, exists, err := client.GetEnvironmentVariable(ctx, *secretID, *name)
	if err != nil {
		return fmt.Errorf("read test key %s: %w", *name, err)
	}

	switch operation {
	case "absent":
		if exists {
			return fmt.Errorf("test key %s unexpectedly exists", *name)
		}
		return nil
	case "delete":
		if !exists {
			return nil
		}
		if _, err := client.DeleteEnvironmentVariable(ctx, *secretID, *name); err != nil {
			return fmt.Errorf("delete test key %s: %w", *name, err)
		}
		_, _, exists, err = client.GetEnvironmentVariable(ctx, *secretID, *name)
		if err != nil {
			return fmt.Errorf("verify deletion of test key %s: %w", *name, err)
		}
		if exists {
			return fmt.Errorf("test key %s still exists after deletion", *name)
		}
		return nil
	case "present", "hash", "changed":
		if !exists {
			return fmt.Errorf("test key %s does not exist", *name)
		}
	default:
		return fmt.Errorf("unknown operation %q", operation)
	}

	if expected := os.Getenv("EXPECTED_VALUE"); expected != "" && subtle.ConstantTimeCompare([]byte(value), []byte(expected)) != 1 {
		return fmt.Errorf("test key %s does not contain the expected value", *name)
	}
	if *expectedLength > 0 && len(value) != *expectedLength {
		return fmt.Errorf("test key %s has length %d, expected %d", *name, len(value), *expectedLength)
	}
	lower, upper, numeric, special := characterCounts(value)
	if lower < *minLower || upper < *minUpper || numeric < *minNumeric || special < *minSpecial {
		return fmt.Errorf(
			"test key %s does not satisfy character minimums: lower=%d upper=%d numeric=%d special=%d",
			*name, lower, upper, numeric, special,
		)
	}
	digest := sha256.Sum256([]byte(value))
	digestHex := hex.EncodeToString(digest[:])
	if operation == "changed" && digestHex == *previousHash {
		return fmt.Errorf("test key %s was not rotated", *name)
	}
	if operation == "hash" {
		fmt.Println(digestHex)
	}
	return nil
}

func characterCounts(value string) (lower, upper, numeric, special int) {
	for _, character := range value {
		switch {
		case unicode.IsLower(character):
			lower++
		case unicode.IsUpper(character):
			upper++
		case unicode.IsDigit(character):
			numeric++
		default:
			special++
		}
	}
	return lower, upper, numeric, special
}
