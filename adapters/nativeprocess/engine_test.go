package nativeprocess

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

func TestAdapterLaunchesAndStopsProcess(t *testing.T) {
	output := filepath.Join(t.TempDir(), "helper.txt")
	options, err := json.Marshal(map[string]any{
		"args": []string{
			"-test.run=TestNativeProcessHelper",
			"--",
		},
		"environment": map[string]string{
			"JANGOLOVA_NATIVE_HELPER": "wait",
			"JANGOLOVA_HELPER_OUTPUT": output,
			"OVERRIDE_VALUE":          "configured",
		},
		"startupGrace": "100ms",
		"stopTimeout":  "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	handle, err := (Adapter{}).Start(
		context.Background(),
		manifest.EngineSpec{Source: os.Args[0], Options: options},
		orchestrator.EngineRuntime{
			Environment: orchestrator.EngineEnvironment{
				"SURFACE_VALUE":  "available",
				"OVERRIDE_VALUE": "surface",
			},
		},
	)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	instance := handle.(*instance)
	if instance.ProcessID() == 0 {
		t.Fatal("ProcessID() = 0 while process is running")
	}
	if health := instance.EngineHealth(context.Background()); health.Status != orchestrator.EngineHealthHealthy {
		t.Fatalf("EngineHealth() = %#v", health)
	}
	waitForFile(t, output, "started:available:configured:native-process")
	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if instance.ProcessID() != 0 {
		t.Fatalf("ProcessID() = %d after stop", instance.ProcessID())
	}
	if health := instance.EngineHealth(context.Background()); health.Status != orchestrator.EngineHealthStopped {
		t.Fatalf("EngineHealth() after stop = %#v", health)
	}
	waitForFile(t, output, "stopped")
	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestAdapterRejectsEarlyExitAndInvalidOptions(t *testing.T) {
	t.Parallel()

	options := json.RawMessage(`{
		"args":["-test.run=TestNativeProcessHelper","--"],
		"environment":{"JANGOLOVA_NATIVE_HELPER":"exit"},
		"startupGrace":"2s"
	}`)
	_, err := (Adapter{}).Start(
		context.Background(),
		manifest.EngineSpec{Source: os.Args[0], Options: options},
		orchestrator.EngineRuntime{},
	)
	if err == nil || !strings.Contains(err.Error(), "exited during startup") {
		t.Fatalf("early Start() error = %v", err)
	}
	for _, raw := range []string{
		`{"unknown":true}`,
		`{"startupGrace":"invalid"}`,
		`{"stopTimeout":"0s"}`,
		`{"environment":{"BAD=NAME":"value"}}`,
	} {
		_, err := (Adapter{}).Start(
			context.Background(),
			manifest.EngineSpec{Source: os.Args[0], Options: json.RawMessage(raw)},
			orchestrator.EngineRuntime{},
		)
		if err == nil {
			t.Errorf("Start(options=%s) succeeded, want error", raw)
		}
	}
}

func TestAdapterReportsUnexpectedExit(t *testing.T) {
	t.Parallel()

	options := json.RawMessage(`{
		"args":["-test.run=TestNativeProcessHelper","--"],
		"environment":{"JANGOLOVA_NATIVE_HELPER":"exit-later"},
		"startupGrace":"10ms"
	}`)
	handle, err := (Adapter{}).Start(
		context.Background(),
		manifest.EngineSpec{Source: os.Args[0], Options: options},
		orchestrator.EngineRuntime{},
	)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	instance := handle.(*instance)
	select {
	case event := <-instance.EngineEvents():
		if event.Type != "engine.exited" || event.Status != "exited" {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("engine exit event was not reported")
	}
	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestNativeProcessHelper(t *testing.T) {
	mode := os.Getenv("JANGOLOVA_NATIVE_HELPER")
	if mode == "" {
		return
	}
	if mode == "exit" {
		os.Exit(0)
	}
	output := os.Getenv("JANGOLOVA_HELPER_OUTPUT")
	initial := strings.Join([]string{
		"started",
		os.Getenv("SURFACE_VALUE"),
		os.Getenv("OVERRIDE_VALUE"),
		os.Getenv("JANGOLOVA_ENGINE_ADAPTER"),
	}, ":")
	if mode == "exit-later" {
		time.Sleep(100 * time.Millisecond)
		return
	}
	if err := os.WriteFile(output, []byte(initial), 0o600); err != nil {
		os.Exit(2)
	}
	signals := make(chan os.Signal, 1)
	signalNotifyInterrupt(signals)
	<-signals
	if err := os.WriteFile(output, []byte(initial+"\nstopped"), 0o600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func waitForFile(t *testing.T, path, expected string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), expected) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %q did not contain %q; data=%q error=%v", path, expected, data, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
