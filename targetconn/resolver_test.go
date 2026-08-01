package targetconn

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jangolova/internal/orchestrator"
)

func TestPrepareResolvesRedactsAndReleasesCredential(t *testing.T) {
	released := 0
	resolver := ResolverFunc(func(_ context.Context, request Request) (Material, error) {
		if request.Reference != "browser-session" || request.TargetID != "browser-42" || request.Protocol != "cdp" {
			t.Fatalf("request = %#v", request)
		}
		return Material{
			Headers:   map[string]string{"authorization": "Bearer highly-sensitive-value"},
			ExpiresAt: time.Now().Add(time.Minute),
			Release:   func(context.Context) error { released++; return nil },
		}, nil
	})
	target, release, err := Prepare(context.Background(), resolver, orchestrator.EngineTarget{
		TargetID: "browser-42", Kind: "browser",
		Endpoints: []orchestrator.TargetEndpoint{{Name: "control", Protocol: "cdp", URL: "wss://browser.example/control", CredentialRef: "browser-session"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	connection := target.Endpoints[0].Connection
	if connection == nil || connection.Headers["Authorization"] != "Bearer highly-sensitive-value" {
		t.Fatalf("connection = %#v", connection)
	}
	redacted := RedactString("dial rejected Bearer highly-sensitive-value", target)
	if strings.Contains(redacted, "highly-sensitive") || !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("redacted = %q", redacted)
	}
	if err := release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if released != 1 || len(connection.Headers) != 0 {
		t.Fatalf("released = %d, headers = %#v", released, connection.Headers)
	}
}

func TestRedactJSONHandlesEscapedAndAuthorizationValues(t *testing.T) {
	target := orchestrator.EngineTarget{Endpoints: []orchestrator.TargetEndpoint{{
		Connection: &orchestrator.EndpointConnection{Headers: map[string]string{
			"Authorization": `Bearer token-with-"quote`,
		}},
	}}}
	document := RedactJSON(json.RawMessage(`{"whole":"Bearer token-with-\"quote","token":"token-with-\"quote"}`), target)
	if !json.Valid(document) || strings.Contains(string(document), "token-with") || strings.Count(string(document), "[REDACTED]") != 2 {
		t.Fatalf("redacted JSON = %s", document)
	}
}

func TestEnvironmentResolverRequiresExpiringCredential(t *testing.T) {
	resolver := EnvironmentResolver{Lookup: func(name string) (string, bool) {
		if name != "JANGOLOVA_CREDENTIAL_BROWSER_2DSESSION" {
			t.Fatalf("environment name = %q", name)
		}
		return `{"apiVersion":"interaction.connection/v1alpha1","kind":"credential","headers":{"Authorization":"Bearer token"}}`, true
	}}
	_, err := resolver.Resolve(context.Background(), Request{Kind: CredentialReference, Reference: "browser-session"})
	if err == nil || !strings.Contains(err.Error(), "expiresAt") {
		t.Fatalf("error = %v", err)
	}
}

func TestDirectoryResolverUsesReferenceAsIdentifier(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "credential")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	document := fmt.Sprintf(`{"apiVersion":"interaction.connection/v1alpha1","kind":"credential","headers":{"Authorization":"Bearer directory-token"},"expiresAt":%q}`, time.Now().Add(time.Minute).Format(time.RFC3339Nano))
	if err := os.WriteFile(filepath.Join(directory, "browser-session.json"), []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	material, err := (DirectoryResolver{Root: root}).Resolve(context.Background(), Request{Kind: CredentialReference, Reference: "browser-session"})
	if err != nil {
		t.Fatal(err)
	}
	if material.Headers["Authorization"] != "Bearer directory-token" {
		t.Fatalf("headers = %#v", material.Headers)
	}
	if _, err := (DirectoryResolver{Root: root}).Resolve(context.Background(), Request{Kind: CredentialReference, Reference: "../browser-session"}); err == nil {
		t.Fatal("path-like reference was accepted")
	}
}

func TestHTTPClientAppliesHeaderAndEnforcesExpiry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("Authorization") != "Bearer remote-session" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	connection := &orchestrator.EndpointConnection{
		Headers:   map[string]string{"Authorization": "Bearer remote-session"},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	endpoint := orchestrator.TargetEndpoint{URL: server.URL, Connection: connection}
	client, err := HTTPClient(endpoint, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	connection.ExpiresAt = time.Now().Add(-time.Second)
	_, err = client.Get(server.URL)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired request error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestHTTPClientUsesCallerResolvedCertificateAuthority(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	certificate := server.Certificate()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	endpoint := orchestrator.TargetEndpoint{
		URL:        server.URL,
		Connection: &orchestrator.EndpointConnection{TLS: &orchestrator.TLSConnection{CAFile: caPath}},
	}
	client, err := HTTPClient(endpoint, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
}

func TestPrepareRejectsCredentialThatExpiresTooSoon(t *testing.T) {
	released := 0
	resolver := ResolverFunc(func(context.Context, Request) (Material, error) {
		return Material{
			Headers:   map[string]string{"Authorization": "Bearer short-lived"},
			ExpiresAt: time.Now().Add(time.Second),
			Release:   func(context.Context) error { released++; return nil },
		}, nil
	})
	_, _, err := Prepare(context.Background(), resolver, orchestrator.EngineTarget{
		Endpoints: []orchestrator.TargetEndpoint{{Name: "cdp", Protocol: "cdp", CredentialRef: "short"}},
	})
	if err == nil || !strings.Contains(err.Error(), "expires too soon") {
		t.Fatalf("error = %v", err)
	}
	if released != 1 {
		t.Fatalf("release count = %d", released)
	}
}

func TestChainStopsOnlyForReferenceNotFound(t *testing.T) {
	want := errors.New("secret manager unavailable")
	chain := Chain{
		ResolverFunc(func(context.Context, Request) (Material, error) { return Material{}, ErrReferenceNotFound }),
		ResolverFunc(func(context.Context, Request) (Material, error) { return Material{}, want }),
		ResolverFunc(func(context.Context, Request) (Material, error) {
			t.Fatal("unexpected resolver")
			return Material{}, nil
		}),
	}
	_, err := chain.Resolve(context.Background(), Request{})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
