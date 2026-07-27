package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	APIVersion = "jangolova.dev/v1alpha1"
	Kind       = "Session"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Manifest struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`
}

type Metadata struct {
	Name string `json:"name"`
}

type Spec struct {
	Engine      EngineSpec       `json:"engine"`
	Surfaces    []SurfaceSpec    `json:"surfaces,omitempty"`
	Controllers []ControllerSpec `json:"controllers,omitempty"`
	Connectors  []ConnectorSpec  `json:"connectors,omitempty"`
}

type EngineSpec struct {
	Adapter string          `json:"adapter"`
	Source  string          `json:"source,omitempty"`
	Options json.RawMessage `json:"options,omitempty"`
}

type SurfaceSpec struct {
	Name    string          `json:"name"`
	Adapter string          `json:"adapter"`
	Options json.RawMessage `json:"options,omitempty"`
}

type ControllerSpec struct {
	Name    string          `json:"name"`
	Adapter string          `json:"adapter"`
	Options json.RawMessage `json:"options,omitempty"`
}

type ConnectorSpec struct {
	Name    string          `json:"name"`
	Adapter string          `json:"adapter"`
	Surface string          `json:"surface"`
	Options json.RawMessage `json:"options,omitempty"`
}

func Decode(reader io.Reader) (Manifest, error) {
	var value Manifest
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&value); err != nil {
		return Manifest{}, fmt.Errorf("decode session manifest: %w", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, errors.New("decode session manifest: multiple JSON values")
		}
		return Manifest{}, fmt.Errorf("decode session manifest: %w", err)
	}

	if err := value.Validate(); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func (m Manifest) Validate() error {
	var problems []error

	if m.APIVersion != APIVersion {
		problems = append(problems, fmt.Errorf("apiVersion must be %q", APIVersion))
	}
	if m.Kind != Kind {
		problems = append(problems, fmt.Errorf("kind must be %q", Kind))
	}
	if err := validateName("metadata.name", m.Metadata.Name); err != nil {
		problems = append(problems, err)
	}
	if strings.TrimSpace(m.Spec.Engine.Adapter) == "" {
		problems = append(problems, errors.New("spec.engine.adapter is required"))
	}
	if err := validateOptions("spec.engine.options", m.Spec.Engine.Options); err != nil {
		problems = append(problems, err)
	}

	surfaces := make(map[string]struct{}, len(m.Spec.Surfaces))
	for index, surface := range m.Spec.Surfaces {
		path := fmt.Sprintf("spec.surfaces[%d]", index)
		if err := validateName(path+".name", surface.Name); err != nil {
			problems = append(problems, err)
		} else if _, exists := surfaces[surface.Name]; exists {
			problems = append(problems, fmt.Errorf("%s.name %q is duplicated", path, surface.Name))
		} else {
			surfaces[surface.Name] = struct{}{}
		}
		if strings.TrimSpace(surface.Adapter) == "" {
			problems = append(problems, fmt.Errorf("%s.adapter is required", path))
		}
		if err := validateOptions(path+".options", surface.Options); err != nil {
			problems = append(problems, err)
		}
	}

	controllerNames := make(map[string]struct{}, len(m.Spec.Controllers))
	for index, controller := range m.Spec.Controllers {
		path := fmt.Sprintf("spec.controllers[%d]", index)
		if err := validateName(path+".name", controller.Name); err != nil {
			problems = append(problems, err)
		} else if _, exists := controllerNames[controller.Name]; exists {
			problems = append(problems, fmt.Errorf("%s.name %q is duplicated", path, controller.Name))
		} else {
			controllerNames[controller.Name] = struct{}{}
		}
		if strings.TrimSpace(controller.Adapter) == "" {
			problems = append(problems, fmt.Errorf("%s.adapter is required", path))
		}
		if err := validateOptions(path+".options", controller.Options); err != nil {
			problems = append(problems, err)
		}
	}

	connectorNames := make(map[string]struct{}, len(m.Spec.Connectors))
	for index, connector := range m.Spec.Connectors {
		path := fmt.Sprintf("spec.connectors[%d]", index)
		if err := validateName(path+".name", connector.Name); err != nil {
			problems = append(problems, err)
		} else if _, exists := connectorNames[connector.Name]; exists {
			problems = append(problems, fmt.Errorf("%s.name %q is duplicated", path, connector.Name))
		} else {
			connectorNames[connector.Name] = struct{}{}
		}
		if strings.TrimSpace(connector.Adapter) == "" {
			problems = append(problems, fmt.Errorf("%s.adapter is required", path))
		}
		if connector.Surface == "" {
			problems = append(problems, fmt.Errorf("%s.surface is required", path))
		} else if _, exists := surfaces[connector.Surface]; !exists {
			problems = append(problems, fmt.Errorf(
				"%s.surface %q does not reference a declared surface",
				path,
				connector.Surface,
			))
		}
		if err := validateOptions(path+".options", connector.Options); err != nil {
			problems = append(problems, err)
		}
	}

	return errors.Join(problems...)
}

func validateName(path, value string) error {
	if !namePattern.MatchString(value) {
		return fmt.Errorf(
			"%s must start with a lowercase letter and contain at most 63 lowercase letters, digits, or hyphens",
			path,
		)
	}
	return nil
}

func validateOptions(path string, raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s must be a JSON object: %w", path, err)
	}
	if value == nil {
		return fmt.Errorf("%s must be a JSON object", path)
	}
	return nil
}
