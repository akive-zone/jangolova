package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jangolova/targetconn"
)

func TestModelsCommand(t *testing.T) {
	t.Parallel()

	if err := modelsCommand(nil); err != nil {
		t.Fatalf("modelsCommand(nil) failed: %v", err)
	}

	if err := modelsCommand([]string{"--json"}); err != nil {
		t.Fatalf("modelsCommand(['--json']) failed: %v", err)
	}

	if err := modelsCommand([]string{"unexpected-arg"}); err == nil {
		t.Fatal("modelsCommand with positional arg should fail")
	}
}

func TestConnectModelCommandValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing endpoint", []string{"--model", "gpt-4o"}},
		{"missing model", []string{"--endpoint", "https://api.openai.com/v1"}},
		{"non-loopback http", []string{"--endpoint", "http://api.openai.com/v1", "--model", "gpt-4o"}},
		{"positional arg rejected", []string{"--endpoint", "https://api.openai.com/v1", "--model", "gpt-4o", "extra"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := connectModelCommand(tc.args); err == nil {
				t.Fatalf("connectModelCommand(%v) expected error, got nil", tc.args)
			}
		})
	}
}

func TestConnectModelCommandSuccess(t *testing.T) {
	t.Parallel()

	// Mock server mimicking an OpenAI-compatible model gateway.
	// The openaimodel client does not make a call during construction, so a
	// minimal HTTP server is enough to satisfy the HTTP transport round-trip
	// that targetconn prepares.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1000000000,"choices":[]}`))
	}))
	defer server.Close()

	args := []string{
		"--protocol", "openai-compatible",
		"--endpoint", server.URL, // httptest.NewServer uses http://127.0.0.1:PORT (loopback)
		"--model", "test-model",
		"--token", "test-token-for-cli-probe",
		"--json",
	}

	if err := connectModelCommand(args); err != nil {
		t.Fatalf("connectModelCommand failed against mock server: %v", err)
	}
}

func TestInlineOrDefaultResolver(t *testing.T) {
	t.Parallel()

	t.Run("empty token uses DefaultResolver", func(t *testing.T) {
		t.Parallel()
		resolver := inlineOrDefaultResolver("my-ref", "")
		// DefaultResolver returns a Chain — just verify it's non-nil
		if resolver == nil {
			t.Fatal("expected non-nil resolver")
		}
	})

	t.Run("inline token resolves named credentialRef", func(t *testing.T) {
		t.Parallel()
		resolver := inlineOrDefaultResolver("test-cred", "my-secret-key")
		material, err := resolver.Resolve(t.Context(), targetconn.Request{
			Kind:      targetconn.CredentialReference,
			Reference: "test-cred",
		})
		if err != nil {
			t.Fatalf("Resolve(test-cred) failed: %v", err)
		}
		if material.Headers["Authorization"] != "Bearer my-secret-key" {
			t.Fatalf("unexpected Authorization header: %q", material.Headers["Authorization"])
		}
		if material.ExpiresAt.IsZero() {
			t.Fatal("ExpiresAt must be set for inline credential material")
		}
	})

	t.Run("already-prefixed bearer token is not double-prefixed", func(t *testing.T) {
		t.Parallel()
		resolver := inlineOrDefaultResolver("ref", "Bearer already-prefixed-token")
		material, err := resolver.Resolve(t.Context(), targetconn.Request{
			Kind:      targetconn.CredentialReference,
			Reference: "ref",
		})
		if err != nil {
			t.Fatal(err)
		}
		if material.Headers["Authorization"] != "Bearer already-prefixed-token" {
			t.Fatalf("double-prefixed: %q", material.Headers["Authorization"])
		}
	})

	t.Run("unmatched reference falls through", func(t *testing.T) {
		t.Parallel()
		resolver := inlineOrDefaultResolver("my-ref", "some-token")
		_, err := resolver.Resolve(t.Context(), targetconn.Request{
			Kind:      targetconn.CredentialReference,
			Reference: "other-ref",
		})
		if err == nil {
			t.Fatal("expected error for unmatched reference, got nil")
		}
	})
}
