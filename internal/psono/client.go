package psono

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

const apiKeyAccessSecretPath = "api-key-access/secret/"

var ErrWriteConflict = errors.New("Psono secret changed while it was being updated")

type Credentials struct {
	ServerURL         string
	APIKeyID          string
	APISecretKey      string
	CABundle          []byte
	AllowInsecureHTTP bool
}

type Secret struct {
	Data      []byte
	Key       [32]byte
	WriteDate string
}

type EnvironmentVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Client struct {
	endpoint     string
	apiKeyID     string
	apiSecretKey [32]byte
	httpClient   *http.Client
	locksMu      sync.Mutex
	locks        map[string]*sync.Mutex
}

type encryptedSecretResponse struct {
	Data           string `json:"data"`
	DataNonce      string `json:"data_nonce"`
	SecretKey      string `json:"secret_key"`
	SecretKeyNonce string `json:"secret_key_nonce"`
	WriteDate      string `json:"write_date"`
}

type updateSecretResponse struct {
	WriteDate string `json:"write_date"`
}

func NewClient(credentials Credentials) (*Client, error) {
	serverURL, err := url.Parse(credentials.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if serverURL.Scheme != "https" && serverURL.Scheme != "http" {
		return nil, errors.New("server URL must use http or https")
	}
	if serverURL.Host == "" || serverURL.User != nil || serverURL.RawQuery != "" || serverURL.Fragment != "" {
		return nil, errors.New("server URL must contain a host and no user info, query, or fragment")
	}
	if serverURL.Scheme == "http" && !credentials.AllowInsecureHTTP {
		return nil, errors.New("server URL must use https unless allow_insecure_http is enabled")
	}

	apiSecretKey, err := decodeKey(credentials.APISecretKey)
	if err != nil {
		return nil, fmt.Errorf("decode API secret key: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if len(credentials.CABundle) > 0 {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system certificate pool: %w", err)
		}
		if !roots.AppendCertsFromPEM(credentials.CABundle) {
			return nil, errors.New("CA bundle does not contain a valid PEM certificate")
		}
		transport.TLSClientConfig.RootCAs = roots
	}

	serverURL.Path = strings.TrimRight(serverURL.Path, "/") + "/" + apiKeyAccessSecretPath
	return &Client{
		endpoint:     serverURL.String(),
		apiKeyID:     credentials.APIKeyID,
		apiSecretKey: apiSecretKey,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		locks: make(map[string]*sync.Mutex),
	}, nil
}

func (c *Client) GetSecret(ctx context.Context, secretID string) (Secret, error) {
	body, err := json.Marshal(map[string]string{"api_key_id": c.apiKeyID, "secret_id": secretID})
	if err != nil {
		return Secret{}, fmt.Errorf("encode request: %w", err)
	}
	response, err := c.request(ctx, http.MethodPost, body)
	if err != nil {
		return Secret{}, err
	}

	var encrypted encryptedSecretResponse
	if err := json.Unmarshal(response, &encrypted); err != nil {
		return Secret{}, fmt.Errorf("decode encrypted Psono response: %w", err)
	}
	secretKeyHex, err := openHex(encrypted.SecretKey, encrypted.SecretKeyNonce, &c.apiSecretKey)
	if err != nil {
		return Secret{}, fmt.Errorf("decrypt secret key: %w", err)
	}
	secretKey, err := decodeKey(string(secretKeyHex))
	clear(secretKeyHex)
	if err != nil {
		return Secret{}, fmt.Errorf("decode decrypted secret key: %w", err)
	}
	plaintext, err := openHex(encrypted.Data, encrypted.DataNonce, &secretKey)
	if err != nil {
		clear(secretKey[:])
		return Secret{}, fmt.Errorf("decrypt secret: %w", err)
	}
	return Secret{Data: plaintext, Key: secretKey, WriteDate: encrypted.WriteDate}, nil
}

func (c *Client) updateSecret(ctx context.Context, secretID string, plaintext []byte, key *[32]byte, oldWriteDate string) (string, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	ciphertext := secretbox.Seal(nil, plaintext, &nonce, key)
	body, err := json.Marshal(map[string]string{
		"api_key_id":     c.apiKeyID,
		"secret_id":      secretID,
		"data":           hex.EncodeToString(ciphertext),
		"data_nonce":     hex.EncodeToString(nonce[:]),
		"old_write_date": oldWriteDate,
	})
	clear(ciphertext)
	if err != nil {
		return "", fmt.Errorf("encode update request: %w", err)
	}
	response, err := c.request(ctx, http.MethodPut, body)
	if err != nil {
		return "", err
	}
	var update updateSecretResponse
	if err := json.Unmarshal(response, &update); err != nil {
		return "", fmt.Errorf("decode update response: %w", err)
	}
	return update.WriteDate, nil
}

func (c *Client) GetEnvironmentVariable(ctx context.Context, secretID, name string) (string, string, bool, error) {
	secret, err := c.GetSecret(ctx, secretID)
	if err != nil {
		return "", "", false, err
	}
	defer clearSecret(&secret)
	variables, _, err := decodeEnvironmentVariables(secret.Data)
	if err != nil {
		return "", "", false, err
	}
	for _, variable := range variables {
		if variable.Key == name {
			return variable.Value, secret.WriteDate, true, nil
		}
	}
	return "", secret.WriteDate, false, nil
}

func (c *Client) EnsureEnvironmentVariable(ctx context.Context, secretID, name string, generate func() (string, error)) (string, bool, error) {
	var generated bool
	writeDate, err := c.mutateEnvironmentVariables(ctx, secretID, func(variables []EnvironmentVariable) ([]EnvironmentVariable, bool, error) {
		for _, variable := range variables {
			if variable.Key == name {
				return variables, false, nil
			}
		}
		value, err := generate()
		if err != nil {
			return nil, false, err
		}
		generated = true
		return append(variables, EnvironmentVariable{Key: name, Value: value}), true, nil
	})
	return writeDate, generated, err
}

func (c *Client) UpsertEnvironmentVariable(ctx context.Context, secretID, name, value string) (string, error) {
	return c.mutateEnvironmentVariables(ctx, secretID, func(variables []EnvironmentVariable) ([]EnvironmentVariable, bool, error) {
		for index := range variables {
			if variables[index].Key != name {
				continue
			}
			if variables[index].Value == value {
				return variables, false, nil
			}
			variables[index].Value = value
			return variables, true, nil
		}
		return append(variables, EnvironmentVariable{Key: name, Value: value}), true, nil
	})
}

func (c *Client) DeleteEnvironmentVariable(ctx context.Context, secretID, name string) (string, error) {
	return c.mutateEnvironmentVariables(ctx, secretID, func(variables []EnvironmentVariable) ([]EnvironmentVariable, bool, error) {
		for index := range variables {
			if variables[index].Key == name {
				return append(variables[:index], variables[index+1:]...), true, nil
			}
		}
		return variables, false, nil
	})
}

func (c *Client) mutateEnvironmentVariables(ctx context.Context, secretID string, mutate func([]EnvironmentVariable) ([]EnvironmentVariable, bool, error)) (string, error) {
	lock := c.secretLock(secretID)
	lock.Lock()
	defer lock.Unlock()

	for attempt := 0; attempt < 3; attempt++ {
		secret, err := c.GetSecret(ctx, secretID)
		if err != nil {
			return "", err
		}
		variables, document, err := decodeEnvironmentVariables(secret.Data)
		if err != nil {
			clearSecret(&secret)
			return "", err
		}
		updated, changed, err := mutate(variables)
		if err != nil {
			clearSecret(&secret)
			return "", err
		}
		if !changed {
			writeDate := secret.WriteDate
			clearSecret(&secret)
			return writeDate, nil
		}
		document["environment_variables_variables"], err = json.Marshal(updated)
		if err != nil {
			clearSecret(&secret)
			return "", fmt.Errorf("encode environment variables: %w", err)
		}
		plaintext, err := json.Marshal(document)
		if err != nil {
			clearSecret(&secret)
			return "", fmt.Errorf("encode Psono secret: %w", err)
		}
		writeDate, err := c.updateSecret(ctx, secretID, plaintext, &secret.Key, secret.WriteDate)
		clear(plaintext)
		clearSecret(&secret)
		if errors.Is(err, ErrWriteConflict) {
			continue
		}
		return writeDate, err
	}
	return "", ErrWriteConflict
}

func decodeEnvironmentVariables(data []byte) ([]EnvironmentVariable, map[string]json.RawMessage, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, nil, fmt.Errorf("decode Psono secret JSON: %w", err)
	}
	raw, ok := document["environment_variables_variables"]
	if !ok {
		return nil, nil, errors.New("Psono secret is not an Environment Variables entry")
	}
	var variables []EnvironmentVariable
	if err := json.Unmarshal(raw, &variables); err != nil {
		return nil, nil, fmt.Errorf("decode environment variables: %w", err)
	}
	return variables, document, nil
}

func (c *Client) request(ctx context.Context, method string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Psono request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "terraform-provider-psono")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Psono: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read Psono response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if bytes.Contains(responseBody, []byte("WRITE_DATE_MISMATCH")) {
			return nil, ErrWriteConflict
		}
		return nil, fmt.Errorf("Psono returned HTTP %d", response.StatusCode)
	}
	return responseBody, nil
}

func (c *Client) secretLock(secretID string) *sync.Mutex {
	c.locksMu.Lock()
	defer c.locksMu.Unlock()
	if lock, ok := c.locks[secretID]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	c.locks[secretID] = lock
	return lock
}

func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	clear(c.apiSecretKey[:])
	return nil
}

func clearSecret(secret *Secret) {
	clear(secret.Data)
	clear(secret.Key[:])
}

func decodeKey(value string) ([32]byte, error) {
	var key [32]byte
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return key, err
	}
	if len(decoded) != len(key) {
		return key, fmt.Errorf("expected %d bytes, got %d", len(key), len(decoded))
	}
	copy(key[:], decoded)
	clear(decoded)
	return key, nil
}

func openHex(ciphertextHex, nonceHex string, key *[32]byte) ([]byte, error) {
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	if len(nonceBytes) != 24 {
		return nil, fmt.Errorf("expected 24-byte nonce, got %d", len(nonceBytes))
	}
	var nonce [24]byte
	copy(nonce[:], nonceBytes)
	clear(nonceBytes)
	plaintext, ok := secretbox.Open(nil, ciphertext, &nonce, key)
	clear(ciphertext)
	if !ok {
		return nil, errors.New("authentication failed")
	}
	return plaintext, nil
}
