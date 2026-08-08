package pacmanprotocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestPacmanV1GeneratedBindingsRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	original := Hello{
		ProtocolVersion: ProtocolVersion,
		Implementation:  Implementation{Engine: "godot", Name: "pacman-fixture", Version: "1.0"},
		Features:        []string{"scene", "object"},
	}
	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal hello: %v", err)
	}
	var decoded Hello
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal hello: %v", err)
	}
	if diff := cmp.Diff(original, decoded); diff != "" {
		t.Fatalf("hello round-trip mismatch: %s", diff)
	}

	description := Description{
		Revision: "rev-1",
		Resources: []Resource{
			{ID: "scene:main", Kind: KindScene, Label: "Main Scene", Properties: json.RawMessage(`{"backgroundColor":"#000"}`)},
		},
	}
	payload, err = json.Marshal(description)
	if err != nil {
		t.Fatalf("marshal description: %v", err)
	}
	var decodedDesc Description
	if err := json.Unmarshal(payload, &decodedDesc); err != nil {
		t.Fatalf("unmarshal description: %v", err)
	}
	if diff := cmp.Diff(description, decodedDesc); diff != "" {
		t.Fatalf("description round-trip mismatch: %s", diff)
	}

	eventBatch := EventBatch{
		Cursor: "1",
		Events: []Event{
			{
				ID:         "evt-1",
				Type:       "resource.updated",
				SourceID:   "scene:main",
				OccurredAt: now,
				Data:       json.RawMessage(`{"revision":"rev-2"}`),
			},
		},
	}
	payload, err = json.Marshal(eventBatch)
	if err != nil {
		t.Fatalf("marshal eventBatch: %v", err)
	}
	var decodedBatch EventBatch
	if err := json.Unmarshal(payload, &decodedBatch); err != nil {
		t.Fatalf("unmarshal eventBatch: %v", err)
	}
	if diff := cmp.Diff(eventBatch, decodedBatch); diff != "" {
		t.Fatalf("eventBatch round-trip mismatch: %s", diff)
	}
}

func TestPacmanV1ClientCall(t *testing.T) {
	t.Parallel()
	var captured CallRequest
	transport := &fakeTransport{
		response: CallResponse{ProtocolVersion: ProtocolVersion, Result: json.RawMessage(`{"revision":"rev-2"}`)},
		onCall: func(req CallRequest) {
			captured = req
		},
	}
	client := Client{Transport: transport}
	result, err := client.Call(context.Background(), "act", json.RawMessage(`{"name":"scene.describe"}`))
	if err != nil {
		t.Fatalf("client call: %v", err)
	}
	if string(result) != `{"revision":"rev-2"}` {
		t.Fatalf("result = %s, want {\"revision\":\"rev-2\"}", result)
	}
	if captured.Method != "act" {
		t.Fatalf("method = %q, want act", captured.Method)
	}
}

type fakeTransport struct {
	response CallResponse
	onCall   func(CallRequest)
}

func (f *fakeTransport) Call(_ context.Context, req CallRequest) (CallResponse, error) {
	if f.onCall != nil {
		f.onCall(req)
	}
	return f.response, nil
}
