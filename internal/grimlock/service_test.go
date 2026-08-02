package grimlock

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jangolova/internal/orchestrator"
	"jangolova/targetconn"
)

func testService(t *testing.T) *Service {
	t.Helper()
	registry, err := NewConnectorRegistry(&fakeConnector{protocol: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime(targetconn.ResolverFunc(func(context.Context, targetconn.Request) (targetconn.Material, error) {
		return targetconn.Material{Headers: map[string]string{"Authorization": "Bearer fixture-secret"}}, nil
	}), registry)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(runtime, orchestrator.NewRegistry(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(context.Background()) })
	return service
}

func TestServiceHealthAndAuthorization(t *testing.T) {
	service := testService(t)
	handler := service.Routes()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "fixture") {
		t.Fatalf("connector discovery status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestServiceRejectsInvalidSessionBeforeConnecting(t *testing.T) {
	service := testService(t)
	request := `{"apiVersion":"agent.grimlock/v1alpha1","userId":"app","agent":{"sessionId":"bad_id","model":{"apiVersion":"agent.model/v1alpha1","profileId":"profile","protocol":"fixture","endpoint":"https://gateway.example","model":"test","credentialRef":"credential"}},"bindings":[]}`
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(request))
	httpRequest.Header.Set("Authorization", "Bearer test-token")
	recorder := httptest.NewRecorder()
	service.Routes().ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var value ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(value.Message, "sessionId") {
		t.Fatalf("error = %#v", value)
	}
}

func TestGrimlockCursorRetentionValidation(t *testing.T) {
	if _, err := parseGrimlockCursor("later"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
	if _, err := parseGrimlockLimit("257"); err == nil {
		t.Fatal("oversized event limit accepted")
	}
}
