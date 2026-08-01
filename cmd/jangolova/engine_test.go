package main

import (
	"strings"
	"testing"
)

func TestDecodeEngineOptions(t *testing.T) {
	t.Parallel()

	raw, err := decodeEngineOptions(`{"headless":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"headless":true}` {
		t.Fatalf("options = %s", raw)
	}
	for _, invalid := range []string{"", "null", "[]", "not-json"} {
		if _, err := decodeEngineOptions(invalid); err == nil {
			t.Fatalf("decodeEngineOptions(%q) succeeded", invalid)
		}
	}
}

func TestEnvironmentFlags(t *testing.T) {
	t.Parallel()

	var values environmentFlags
	if err := values.Set("DISPLAY=:99"); err != nil {
		t.Fatal(err)
	}
	if err := values.Set("WAYLAND_DISPLAY=wayland-0"); err != nil {
		t.Fatal(err)
	}
	if values["DISPLAY"] != ":99" || values["WAYLAND_DISPLAY"] != "wayland-0" {
		t.Fatalf("environment = %#v", values)
	}
	for _, invalid := range []string{"DISPLAY", "=value", "BAD\x00KEY=value"} {
		if err := values.Set(invalid); err == nil {
			t.Fatalf("environment %q was accepted", invalid)
		}
	}
	if got := values.String(); !strings.Contains(got, "DISPLAY=:99") {
		t.Fatalf("String() = %q", got)
	}
}

func TestHandleFlags(t *testing.T) {
	t.Parallel()

	var values handleFlags
	if err := values.Set("native.window=window-1234"); err != nil {
		t.Fatal(err)
	}
	if values["native.window"] != "window-1234" {
		t.Fatalf("handles = %#v", values)
	}
	for _, invalid := range []string{
		"native.window",
		"native/window=value",
		"native.window=",
	} {
		if err := values.Set(invalid); err == nil {
			t.Fatalf("handle %q was accepted", invalid)
		}
	}
}
