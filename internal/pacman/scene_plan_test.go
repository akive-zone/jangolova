package pacman

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateScenePlanRequiresDeclaredStableResources(t *testing.T) {
	plan := houseScenePlan()
	if err := ValidateScenePlan(plan); err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].TargetID = "object:not-declared"
	if err := ValidateScenePlan(plan); err == nil || !strings.Contains(err.Error(), "undeclared") {
		t.Fatalf("ValidateScenePlan() error = %v", err)
	}
}

func TestValidateScenePlanAgainstCapabilitiesAndDescription(t *testing.T) {
	plan := houseScenePlan()
	description := Description{Revision: "4", Resources: []Resource{
		{ID: "object:hero", Kind: KindObject},
		{ID: "ui:status", Kind: KindUI},
	}}
	capabilities := []Capability{
		{Name: "object.transform.set", Effect: "write", TargetKinds: []ResourceKind{KindObject}, InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "ui.text.set", Effect: "write", TargetKinds: []ResourceKind{KindUI}, InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	if err := ValidateScenePlanAgainst(plan, description, capabilities); err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].Name = "camera.transform.set"
	if err := ValidateScenePlanAgainst(plan, description, capabilities); err == nil || !strings.Contains(err.Error(), "not advertised") {
		t.Fatalf("ValidateScenePlanAgainst() error = %v", err)
	}
}

func TestExecuteScenePlanNegotiatesAndPreservesActionOrder(t *testing.T) {
	caller := &scenePlanCaller{
		responses: map[string]json.RawMessage{
			MethodHello:        json.RawMessage(`{"protocolVersion":"jangolova.pacman/v1alpha1","implementation":{"engine":"godot","name":"fixture"}}`),
			MethodCapabilities: json.RawMessage(`[{"name":"object.transform.set","effect":"write","targetKinds":["object"],"inputSchema":{"type":"object"}},{"name":"ui.text.set","effect":"write","targetKinds":["ui"],"inputSchema":{"type":"object"}}]`),
			MethodDescribe:     json.RawMessage(`{"revision":"4","resources":[{"id":"object:hero","kind":"object"},{"id":"ui:status","kind":"ui"}]}`),
		},
	}
	plan := houseScenePlan()
	report, err := ExecuteScenePlan(context.Background(), caller, plan)
	if err != nil {
		t.Fatal(err)
	}
	if report.Completed != 2 || report.RevisionBefore != "4" || len(report.ActionResults) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if got, want := caller.actions, []string{"object.transform.set", "ui.text.set"}; !equalStrings(got, want) {
		t.Fatalf("actions = %#v, want %#v", got, want)
	}
}

func TestExecuteScenePlanStopsAtFirstActionFailure(t *testing.T) {
	caller := &scenePlanCaller{
		responses: map[string]json.RawMessage{
			MethodHello:        json.RawMessage(`{"protocolVersion":"jangolova.pacman/v1alpha1","implementation":{"engine":"godot","name":"fixture"}}`),
			MethodCapabilities: json.RawMessage(`[{"name":"object.transform.set","effect":"write","targetKinds":["object"],"inputSchema":{"type":"object"}},{"name":"ui.text.set","effect":"write","targetKinds":["ui"],"inputSchema":{"type":"object"}}]`),
			MethodDescribe:     json.RawMessage(`{"revision":"4","resources":[{"id":"object:hero","kind":"object"},{"id":"ui:status","kind":"ui"}]}`),
		},
		failAction: "ui.text.set",
	}
	report, err := ExecuteScenePlan(context.Background(), caller, houseScenePlan())
	if err == nil || !strings.Contains(err.Error(), "action 1") {
		t.Fatalf("ExecuteScenePlan() error = %v", err)
	}
	if report.Completed != 1 || len(report.ActionResults) != 1 {
		t.Fatalf("report = %#v", report)
	}
}

type scenePlanCaller struct {
	responses  map[string]json.RawMessage
	actions    []string
	failAction string
}

func (c *scenePlanCaller) Call(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	if method == MethodAct {
		var action ActionRequest
		if err := json.Unmarshal(params, &action); err != nil {
			return nil, err
		}
		c.actions = append(c.actions, action.Name)
		if action.Name == c.failAction {
			return nil, errors.New("fixture action failed")
		}
		return json.RawMessage(`{"ok":true}`), nil
	}
	return c.responses[method], nil
}

func houseScenePlan() ScenePlan {
	return ScenePlan{
		APIVersion: ScenePlanVersion,
		Name:       "house-choreography",
		Requires: []ScenePlanResource{
			{ID: "object:hero", Kind: KindObject},
			{ID: "ui:status", Kind: KindUI},
		},
		Actions: []ScenePlanAction{
			{Name: "object.transform.set", TargetID: "object:hero", Input: json.RawMessage(`{"position":{"x":132,"y":129}}`)},
			{Name: "ui.text.set", TargetID: "ui:status", Input: json.RawMessage(`{"text":"WELCOME HOME"}`)},
		},
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
