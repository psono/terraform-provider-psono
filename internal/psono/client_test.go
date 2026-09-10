package psono

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

func TestNewClientRequiresExplicitOptInForHTTP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		serverURL         string
		allowInsecureHTTP bool
		valid             bool
	}{
		{name: "HTTPS remote host", serverURL: "https://psono.example.com/server", valid: true},
		{name: "HTTP localhost", serverURL: "http://localhost:8080/server"},
		{name: "HTTP remote host", serverURL: "http://psono.example.com/server"},
		{name: "HTTP explicitly allowed", serverURL: "http://psono.example.com/server", allowInsecureHTTP: true, valid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClient(Credentials{
				ServerURL: test.serverURL, APIKeyID: "api-key-id", APISecretKey: strings.Repeat("00", 32), AllowInsecureHTTP: test.allowInsecureHTTP,
			})
			if test.valid {
				if err != nil {
					t.Fatalf("create client: %v", err)
				}
				client.Close()
				return
			}
			if err == nil {
				client.Close()
				t.Fatal("expected HTTP URL without explicit opt-in to be rejected")
			}
			if !strings.Contains(err.Error(), "must use https unless allow_insecure_http is enabled") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEnvironmentVariableLifecycle(t *testing.T) {
	t.Parallel()
	serverState := newEncryptedServerState(t)
	server := httptest.NewServer(serverState)
	defer server.Close()

	client, err := NewClient(Credentials{
		ServerURL: server.URL + "/server", APIKeyID: "api-key-id", APISecretKey: hex.EncodeToString(serverState.apiKey[:]), AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer client.Close()

	writeDate, generated, err := client.EnsureEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD", func() (string, error) {
		return "generated", nil
	})
	if err != nil || !generated || writeDate == "" {
		t.Fatalf("ensure variable: generated=%v writeDate=%q err=%v", generated, writeDate, err)
	}
	value, _, exists, err := client.GetEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD")
	if err != nil || !exists || value != "generated" {
		t.Fatalf("read generated variable: value=%q exists=%v err=%v", value, exists, err)
	}

	if _, err := client.UpsertEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD", "rotated"); err != nil {
		t.Fatalf("rotate variable: %v", err)
	}
	value, _, exists, err = client.GetEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD")
	if err != nil || !exists || value != "rotated" {
		t.Fatalf("read rotated variable: value=%q exists=%v err=%v", value, exists, err)
	}

	if _, err := client.DeleteEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD"); err != nil {
		t.Fatalf("delete variable: %v", err)
	}
	_, _, exists, err = client.GetEnvironmentVariable(context.Background(), "secret-id", "DB_PASSWORD")
	if err != nil || exists {
		t.Fatalf("deleted variable still exists: exists=%v err=%v", exists, err)
	}

	serverState.mu.Lock()
	defer serverState.mu.Unlock()
	var document map[string]json.RawMessage
	if err := json.Unmarshal(serverState.plaintext, &document); err != nil {
		t.Fatalf("decode final document: %v", err)
	}
	if string(document["environment_variables_title"]) != `"Managed values"` {
		t.Fatalf("unrelated Psono fields were not preserved: %s", serverState.plaintext)
	}
	if serverState.apiSecretKeySent {
		t.Fatal("API secret key was sent to Psono")
	}
}

type encryptedServerState struct {
	testingT         *testing.T
	mu               sync.Mutex
	apiKey           [32]byte
	secretKey        [32]byte
	plaintext        []byte
	revision         int
	apiSecretKeySent bool
}

func newEncryptedServerState(t *testing.T) *encryptedServerState {
	t.Helper()
	state := &encryptedServerState{
		testingT:  t,
		plaintext: []byte(`{"environment_variables_title":"Managed values","environment_variables_variables":[]}`),
		revision:  1,
	}
	if _, err := rand.Read(state.apiKey[:]); err != nil {
		t.Fatalf("generate API key: %v", err)
	}
	if _, err := rand.Read(state.secretKey[:]); err != nil {
		t.Fatalf("generate secret key: %v", err)
	}
	return state
}

func (s *encryptedServerState) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body map[string]string
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		s.testingT.Errorf("decode request: %v", err)
		response.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, exists := body["api_key_secret_key"]; exists {
		s.apiSecretKeySent = true
	}
	response.Header().Set("Content-Type", "application/json")

	switch request.Method {
	case http.MethodPost:
		secretKeyNonce := randomNonce(s.testingT)
		dataNonce := randomNonce(s.testingT)
		encryptedKey := secretbox.Seal(nil, []byte(hex.EncodeToString(s.secretKey[:])), &secretKeyNonce, &s.apiKey)
		encryptedData := secretbox.Seal(nil, s.plaintext, &dataNonce, &s.secretKey)
		_ = json.NewEncoder(response).Encode(map[string]any{
			"data": hex.EncodeToString(encryptedData), "data_nonce": hex.EncodeToString(dataNonce[:]),
			"secret_key": hex.EncodeToString(encryptedKey), "secret_key_nonce": hex.EncodeToString(secretKeyNonce[:]),
			"write_date": s.writeDate(), "read_count": 1,
		})
	case http.MethodPut:
		if body["old_write_date"] != s.writeDate() {
			response.WriteHeader(http.StatusBadRequest)
			_, _ = response.Write([]byte(`{"non_field_errors":["WRITE_DATE_MISMATCH"]}`))
			return
		}
		nonceBytes, _ := hex.DecodeString(body["data_nonce"])
		var nonce [24]byte
		copy(nonce[:], nonceBytes)
		ciphertext, _ := hex.DecodeString(body["data"])
		plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, &s.secretKey)
		if !ok {
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		s.plaintext = plaintext
		s.revision++
		_ = json.NewEncoder(response).Encode(map[string]string{"write_date": s.writeDate()})
	default:
		response.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *encryptedServerState) writeDate() string {
	return fmt.Sprintf("2026-01-01T00:00:%02dZ", s.revision)
}

func randomNonce(t *testing.T) [24]byte {
	t.Helper()
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("generate nonce: %v", err)
	}
	return nonce
}
