package grimlock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestModelProfileSchemaTracksGoContract(t *testing.T) {
	t.Parallel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve schema test path")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "protocol", "grimlock", "v1", "model-profile.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(contents, &schema); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apiVersion", "profileId", "protocol", "endpoint", "model", "credentialRef", "tlsRef", "metadata"} {
		if _, ok := schema.Properties[name]; !ok {
			t.Fatalf("schema property %q is missing", name)
		}
	}
	if len(schema.Required) != 6 {
		t.Fatalf("schema required = %#v", schema.Required)
	}
}
