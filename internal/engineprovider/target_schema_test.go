package engineprovider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTargetSchemaIsProviderAndLocationNeutral(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "protocol", "target", "v1", "target.schema.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(contents) {
		t.Fatal("target schema is invalid JSON")
	}
	lower := strings.ToLower(string(contents))
	for _, forbidden := range []string{"jangolova", "xallet", "localhost", "container", "virtual-machine"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("provider-neutral target schema contains %q", forbidden)
		}
	}
}
