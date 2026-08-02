package pacman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"jangolova/internal/bridge"
)

var stableIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}:[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

type ConformanceReport struct {
	Implementation string
	Capabilities   []Capability
	Description    Description
	Health         Health
}

func ValidateConformance(ctx context.Context, caller bridge.Caller) (ConformanceReport, error) {
	if caller == nil {
		return ConformanceReport{}, errors.New("Pacman caller is required")
	}
	var hello Hello
	if err := callDecode(ctx, caller, MethodHello, `{}`, &hello); err != nil {
		return ConformanceReport{}, err
	}
	if hello.ProtocolVersion != ProtocolVersion {
		return ConformanceReport{}, fmt.Errorf("Pacman protocol %q is incompatible; expected %q", hello.ProtocolVersion, ProtocolVersion)
	}
	if strings.TrimSpace(hello.Implementation.Name) == "" || !supportedEngine(hello.Implementation.Engine) {
		return ConformanceReport{}, errors.New("Pacman implementation name and supported engine are required")
	}
	var capabilities []Capability
	if err := callDecode(ctx, caller, MethodCapabilities, `{}`, &capabilities); err != nil {
		return ConformanceReport{}, err
	}
	if err := ValidateCapabilities(capabilities); err != nil {
		return ConformanceReport{}, err
	}
	var description Description
	if err := callDecode(ctx, caller, MethodDescribe, `{}`, &description); err != nil {
		return ConformanceReport{}, err
	}
	if err := ValidateDescription(description); err != nil {
		return ConformanceReport{}, err
	}
	var batch EventBatch
	if err := callDecode(ctx, caller, MethodEvents, `{"limit":10}`, &batch); err != nil {
		return ConformanceReport{}, err
	}
	if strings.TrimSpace(batch.Cursor) == "" {
		return ConformanceReport{}, errors.New("Pacman event cursor is required")
	}
	for _, event := range batch.Events {
		if event.ID == "" || event.Type == "" || event.OccurredAt.IsZero() {
			return ConformanceReport{}, errors.New("Pacman events require id, type, and occurredAt")
		}
		if !stableIDPattern.MatchString(event.Type) || !strings.HasPrefix(event.Type, "event:") {
			return ConformanceReport{}, fmt.Errorf("Pacman event type %q is not a stable event ID", event.Type)
		}
		if event.SourceID != "" && !stableIDPattern.MatchString(event.SourceID) {
			return ConformanceReport{}, fmt.Errorf("Pacman event sourceId %q is not stable", event.SourceID)
		}
	}
	var health Health
	if err := callDecode(ctx, caller, MethodHealth, `{}`, &health); err != nil {
		return ConformanceReport{}, err
	}
	if health.Status != "ready" && health.Status != "degraded" && health.Status != "unavailable" || health.ObservedAt.IsZero() {
		return ConformanceReport{}, errors.New("Pacman health requires a valid status and observedAt")
	}
	return ConformanceReport{Implementation: hello.Implementation.Name, Capabilities: capabilities, Description: description, Health: health}, nil
}

func supportedEngine(engine string) bool {
	switch engine {
	case "godot", "unity", "unreal", "threejs":
		return true
	default:
		return false
	}
}

func ValidateActionRequest(value ActionRequest, capabilities map[string]struct{}) error {
	if strings.TrimSpace(value.Name) == "" {
		return errors.New("Pacman action name is required")
	}
	if _, ok := capabilities[value.Name]; !ok {
		return fmt.Errorf("Pacman action %q was not advertised", value.Name)
	}
	if value.TargetID != "" && !stableIDPattern.MatchString(value.TargetID) {
		return fmt.Errorf("Pacman action targetId %q is not stable", value.TargetID)
	}
	if len(bytes.TrimSpace(value.Input)) > 0 && !json.Valid(value.Input) {
		return errors.New("Pacman action input is invalid JSON")
	}
	return nil
}

func ValidateCapabilities(values []Capability) error {
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value.Name) == "" || seen[value.Name] {
			return fmt.Errorf("Pacman capability name %q is empty or duplicated", value.Name)
		}
		seen[value.Name] = true
		if value.Effect != "read" && value.Effect != "write" && value.Effect != "external" {
			return fmt.Errorf("Pacman capability %q has invalid effect", value.Name)
		}
		var schema map[string]any
		if json.Unmarshal(value.InputSchema, &schema) != nil || schema == nil {
			return fmt.Errorf("Pacman capability %q requires an input schema object", value.Name)
		}
		for _, kind := range value.TargetKinds {
			if !ValidKind(kind) {
				return fmt.Errorf("Pacman capability %q has invalid target kind %q", value.Name, kind)
			}
		}
	}
	return nil
}

func ValidateDescription(value Description) error {
	if strings.TrimSpace(value.Revision) == "" {
		return errors.New("Pacman description revision is required")
	}
	seen := map[string]bool{}
	for _, resource := range value.Resources {
		if !ValidKind(resource.Kind) || !stableIDPattern.MatchString(resource.ID) || !strings.HasPrefix(resource.ID, string(resource.Kind)+":") {
			return fmt.Errorf("Pacman resource %q has an invalid kind or stable ID", resource.ID)
		}
		if seen[resource.ID] {
			return fmt.Errorf("Pacman resource %q is duplicated", resource.ID)
		}
		seen[resource.ID] = true
		if len(bytes.TrimSpace(resource.Properties)) > 0 && !json.Valid(resource.Properties) {
			return fmt.Errorf("Pacman resource %q properties are invalid JSON", resource.ID)
		}
	}
	return nil
}

func ValidKind(kind ResourceKind) bool {
	switch kind {
	case KindScene, KindObject, KindUI, KindCamera, KindMaterial, KindAnimation, KindTimeline, KindArtifact, KindEvent:
		return true
	default:
		return false
	}
}

func callDecode(ctx context.Context, caller bridge.Caller, method, params string, target any) error {
	raw, err := caller.Call(ctx, method, json.RawMessage(params))
	if err != nil {
		return fmt.Errorf("Pacman %s: %w", method, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode Pacman %s: %w", method, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		// json.Decoder reports io.EOF after exactly one value.
		return fmt.Errorf("decode Pacman %s: response must contain one JSON value", method)
	}
	return nil
}
