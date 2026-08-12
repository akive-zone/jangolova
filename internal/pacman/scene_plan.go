package pacman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"jangolova/internal/bridge"
)

const ScenePlanVersion = "jangolova.pacman.scene/v1alpha1"

const (
	maximumScenePlanRequirements = 256
	maximumScenePlanActions      = 256
	maximumScenePlanInputBytes   = 64 * 1024
)

// ScenePlan is the bounded model-facing layer above Pacman's transport
// protocol. It does not create arbitrary engine objects; it names resources
// that the target has explicitly registered and sequences advertised actions.
type ScenePlan struct {
	APIVersion  string              `json:"apiVersion"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Requires    []ScenePlanResource `json:"requires"`
	Actions     []ScenePlanAction   `json:"actions"`
}

type ScenePlanResource struct {
	ID   string       `json:"id"`
	Kind ResourceKind `json:"kind"`
}

type ScenePlanAction struct {
	Name     string          `json:"name"`
	TargetID string          `json:"targetId"`
	Input    json.RawMessage `json:"input"`
}

type ScenePlanReport struct {
	PlanName       string            `json:"planName"`
	RevisionBefore string            `json:"revisionBefore"`
	Completed      int               `json:"completed"`
	ActionResults  []json.RawMessage `json:"actionResults"`
}

// ValidateScenePlan validates model output before it is matched to an engine.
// It intentionally rejects target-less actions and undeclared targets so a
// plan cannot smuggle a new engine surface past the explicit allowlist.
func ValidateScenePlan(plan ScenePlan) error {
	if plan.APIVersion != ScenePlanVersion {
		return fmt.Errorf("scene plan apiVersion %q is incompatible; expected %q", plan.APIVersion, ScenePlanVersion)
	}
	if !validScenePlanName(plan.Name) {
		return fmt.Errorf("scene plan name %q is invalid", plan.Name)
	}
	if len([]byte(plan.Description)) > 2048 {
		return errors.New("scene plan description is too large")
	}
	if len(plan.Requires) == 0 || len(plan.Requires) > maximumScenePlanRequirements {
		return fmt.Errorf("scene plan requires between 1 and %d resources", maximumScenePlanRequirements)
	}
	if len(plan.Actions) == 0 || len(plan.Actions) > maximumScenePlanActions {
		return fmt.Errorf("scene plan requires between 1 and %d actions", maximumScenePlanActions)
	}
	required := make(map[string]ResourceKind, len(plan.Requires))
	for _, resource := range plan.Requires {
		if !ValidKind(resource.Kind) || !stableIDPattern.MatchString(resource.ID) || !strings.HasPrefix(resource.ID, string(resource.Kind)+":") {
			return fmt.Errorf("scene plan resource %q has an invalid kind or stable ID", resource.ID)
		}
		if _, exists := required[resource.ID]; exists {
			return fmt.Errorf("scene plan resource %q is duplicated", resource.ID)
		}
		required[resource.ID] = resource.Kind
	}
	for index, action := range plan.Actions {
		if strings.TrimSpace(action.Name) == "" || len(action.Name) > 128 {
			return fmt.Errorf("scene plan action %d has an invalid name", index)
		}
		if !stableIDPattern.MatchString(action.TargetID) {
			return fmt.Errorf("scene plan action %d targetId %q is not stable", index, action.TargetID)
		}
		if _, exists := required[action.TargetID]; !exists {
			return fmt.Errorf("scene plan action %d targets undeclared resource %q", index, action.TargetID)
		}
		if len(bytes.TrimSpace(action.Input)) == 0 || len(action.Input) > maximumScenePlanInputBytes {
			return fmt.Errorf("scene plan action %d input is missing or too large", index)
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal(action.Input, &input); err != nil || input == nil {
			return fmt.Errorf("scene plan action %d input must be a JSON object", index)
		}
	}
	return nil
}

// ValidateScenePlanAgainst checks the plan against the target's current
// description and advertised capabilities. Callers should do this immediately
// before execution because descriptions and capabilities are target-owned.
func ValidateScenePlanAgainst(plan ScenePlan, description Description, capabilities []Capability) error {
	if err := ValidateScenePlan(plan); err != nil {
		return err
	}
	resources := make(map[string]ResourceKind, len(description.Resources))
	for _, resource := range description.Resources {
		resources[resource.ID] = resource.Kind
	}
	capabilityMap := make(map[string]Capability, len(capabilities))
	for _, capability := range capabilities {
		capabilityMap[capability.Name] = capability
	}
	for _, requirement := range plan.Requires {
		actual, exists := resources[requirement.ID]
		if !exists {
			return fmt.Errorf("scene plan resource %q is not present in the target description", requirement.ID)
		}
		if actual != requirement.Kind {
			return fmt.Errorf("scene plan resource %q is %q, not %q", requirement.ID, actual, requirement.Kind)
		}
	}
	for index, action := range plan.Actions {
		capability, exists := capabilityMap[action.Name]
		if !exists {
			return fmt.Errorf("scene plan action %q was not advertised", action.Name)
		}
		kind := resources[action.TargetID]
		if len(capability.TargetKinds) > 0 {
			allowed := false
			for _, targetKind := range capability.TargetKinds {
				if targetKind == kind {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("scene plan action %d %q cannot target %q resources", index, action.Name, kind)
			}
		}
	}
	return nil
}

// ExecuteScenePlan negotiates the target, validates the plan against its
// current allowlist, and invokes actions in order. Execution stops at the
// first failed action; already completed actions are reported for replay or
// compensation decisions by the caller.
func ExecuteScenePlan(ctx context.Context, caller bridge.Caller, plan ScenePlan) (ScenePlanReport, error) {
	if caller == nil {
		return ScenePlanReport{}, errors.New("scene plan caller is required")
	}
	if err := ValidateScenePlan(plan); err != nil {
		return ScenePlanReport{}, err
	}
	var hello Hello
	if err := callDecode(ctx, caller, MethodHello, `{}`, &hello); err != nil {
		return ScenePlanReport{}, err
	}
	if hello.ProtocolVersion != ProtocolVersion {
		return ScenePlanReport{}, fmt.Errorf("Pacman protocol %q is incompatible; expected %q", hello.ProtocolVersion, ProtocolVersion)
	}
	var capabilities []Capability
	if err := callDecode(ctx, caller, MethodCapabilities, `{}`, &capabilities); err != nil {
		return ScenePlanReport{}, err
	}
	var description Description
	if err := callDecode(ctx, caller, MethodDescribe, `{}`, &description); err != nil {
		return ScenePlanReport{}, err
	}
	if err := ValidateScenePlanAgainst(plan, description, capabilities); err != nil {
		return ScenePlanReport{}, err
	}
	report := ScenePlanReport{PlanName: plan.Name, RevisionBefore: description.Revision, ActionResults: make([]json.RawMessage, 0, len(plan.Actions))}
	for index, action := range plan.Actions {
		params, err := json.Marshal(ActionRequest{Name: action.Name, TargetID: action.TargetID, Input: action.Input})
		if err != nil {
			return report, fmt.Errorf("encode scene plan action %d: %w", index, err)
		}
		result, err := caller.Call(ctx, MethodAct, params)
		if err != nil {
			return report, fmt.Errorf("execute scene plan action %d %q: %w", index, action.Name, err)
		}
		if !json.Valid(result) {
			return report, fmt.Errorf("execute scene plan action %d: target returned invalid JSON", index)
		}
		report.ActionResults = append(report.ActionResults, json.RawMessage(append([]byte(nil), result...)))
		report.Completed++
	}
	return report, nil
}

func validScenePlanName(value string) bool {
	if value == "" || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, char := range value[1:] {
		if char != '-' && (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}
