package pacman

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fixtureCaller map[string]json.RawMessage

func (f fixtureCaller) Call(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
	return f[method], nil
}

func TestConformanceAcceptsExplicitAllowlist(t *testing.T) {
	caller := validFixture()
	report, err := ValidateConformance(context.Background(), caller)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Description.Resources) != 2 || report.Implementation != "unity-fixture" {
		t.Fatalf("report = %#v", report)
	}
}

func TestConformanceAcceptsSupportedBrowserAndNativeEngines(t *testing.T) {
	for _, engine := range []string{"godot", "unity", "unreal", "threejs"} {
		caller := validFixture()
		caller[MethodHello] = json.RawMessage(`{"protocolVersion":"jangolova.pacman/v1alpha1","implementation":{"engine":"` + engine + `","name":"fixture"}}`)
		if _, err := ValidateConformance(context.Background(), caller); err != nil {
			t.Fatalf("engine %s: %v", engine, err)
		}
	}
}

func TestConformanceRejectsUnstableOrMismatchedIDs(t *testing.T) {
	for _, description := range []string{
		`{"revision":"1","resources":[{"id":"Hero (Clone)","kind":"object"}]}`,
		`{"revision":"1","resources":[{"id":"camera:main","kind":"object"}]}`,
		`{"revision":"1","resources":[{"id":"object:hero","kind":"object"},{"id":"object:hero","kind":"object"}]}`,
	} {
		caller := validFixture()
		caller[MethodDescribe] = json.RawMessage(description)
		if _, err := ValidateConformance(context.Background(), caller); err == nil || !strings.Contains(err.Error(), "resource") {
			t.Fatalf("description %s error = %v", description, err)
		}
	}
}

func TestActionRequiresAdvertisedCapabilityAndStableTarget(t *testing.T) {
	allowed := map[string]struct{}{"camera.activate": {}}
	for _, request := range []ActionRequest{
		{Name: "camera.delete", TargetID: "camera:main"},
		{Name: "camera.activate", TargetID: "Main Camera"},
	} {
		if err := ValidateActionRequest(request, allowed); err == nil {
			t.Fatalf("ValidateActionRequest(%#v) error = nil", request)
		}
	}
}

func validFixture() fixtureCaller {
	return fixtureCaller{
		MethodHello:        json.RawMessage(`{"protocolVersion":"jangolova.pacman/v1alpha1","implementation":{"engine":"unity","name":"unity-fixture"}}`),
		MethodCapabilities: json.RawMessage(`[{"name":"camera.activate","effect":"write","targetKinds":["camera"],"inputSchema":{"type":"object"}}]`),
		MethodDescribe:     json.RawMessage(`{"revision":"1","resources":[{"id":"scene:main","kind":"scene"},{"id":"camera:main","kind":"camera"}]}`),
		MethodEvents:       json.RawMessage(`{"events":[],"cursor":"0"}`),
		MethodHealth:       json.RawMessage(`{"status":"ready","observedAt":"2026-08-01T00:00:00Z"}`),
	}
}
