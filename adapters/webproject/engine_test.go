package webproject

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeOptionsRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := decodeOptions(json.RawMessage(`{"unknown":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("decodeOptions() error = %v, want unknown field error", err)
	}
}

func TestCleanRelativePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "file", value: "index.html", want: "index.html"},
		{name: "nested", value: "scene/index.html", want: "scene/index.html"},
		{name: "parent", value: "../secret", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := cleanRelativePath(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("cleanRelativePath(%q) = %q, want error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("cleanRelativePath(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("cleanRelativePath(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
