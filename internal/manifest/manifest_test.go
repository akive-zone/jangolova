package manifest

import (
	"strings"
	"testing"
)

func TestDecodeValidManifest(t *testing.T) {
	t.Parallel()

	input := `{
		"apiVersion": "jangolova.dev/v1alpha1",
		"kind": "Session",
		"metadata": {"name": "rotating-cube"},
		"spec": {
			"engine": {"adapter": "browser", "source": "./examples/threejs"},
			"surfaces": [{"name": "desktop", "adapter": "xvfb"}],
			"controllers": [{"name": "automation", "adapter": "puppeteer"}],
			"connectors": [{"name": "remote-view", "adapter": "vnc", "surface": "desktop"}]
		}
	}`

	value, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if value.Metadata.Name != "rotating-cube" {
		t.Fatalf("metadata.name = %q", value.Metadata.Name)
	}
	if value.Spec.Connectors[0].Surface != "desktop" {
		t.Fatalf("connector surface = %q", value.Spec.Connectors[0].Surface)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	input := `{
		"apiVersion": "jangolova.dev/v1alpha1",
		"kind": "Session",
		"metadata": {"name": "demo", "unknown": true},
		"spec": {"engine": {"adapter": "browser"}}
	}`

	_, err := Decode(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Decode() error = %v, want unknown field", err)
	}
}

func TestValidateReportsIndependentProblems(t *testing.T) {
	t.Parallel()

	value := Manifest{
		APIVersion: "wrong",
		Kind:       Kind,
		Metadata:   Metadata{Name: "Not Valid"},
		Spec: Spec{
			Engine: EngineSpec{},
			Surfaces: []SurfaceSpec{
				{Name: "desktop", Adapter: "xvfb"},
				{Name: "desktop", Adapter: ""},
			},
			Connectors: []ConnectorSpec{
				{Name: "viewer", Adapter: "vnc", Surface: "missing"},
			},
		},
	}

	err := value.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, expected := range []string{
		"apiVersion",
		"metadata.name",
		"spec.engine.adapter",
		"duplicated",
		"does not reference",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Validate() error %q does not contain %q", err, expected)
		}
	}
}

func TestValidateRequiresObjectOptions(t *testing.T) {
	t.Parallel()

	value := Manifest{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata:   Metadata{Name: "demo"},
		Spec: Spec{
			Engine: EngineSpec{Adapter: "browser", Options: []byte(`["not", "an", "object"]`)},
		},
	}

	err := value.Validate()
	if err == nil || !strings.Contains(err.Error(), "must be a JSON object") {
		t.Fatalf("Validate() error = %v", err)
	}
}
