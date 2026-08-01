package pacman

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProtocolSchemaIsValidAndEngineNeutral(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "protocol", "pacman", "v1", "protocol.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(contents) {
		t.Fatal("Pacman schema is invalid JSON")
	}
	text := strings.ToLower(string(contents))
	for _, forbidden := range []string{"gameobject", "uobject", "actor address", "container", "display transport"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Pacman schema contains engine/runtime-specific term %q", forbidden)
		}
	}
	for _, required := range []string{"scene", "object", "ui", "camera", "material", "animation", "timeline", "artifact", "event"} {
		if !strings.Contains(text, `"`+required+`"`) {
			t.Fatalf("Pacman schema omits resource kind %q", required)
		}
	}
}
