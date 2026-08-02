package cymonkey

import (
	"context"
	"encoding/json"
	"testing"

	"jangolova/internal/bridge"
)

type conformanceCaller map[string]json.RawMessage

func (caller conformanceCaller) Call(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
	return caller[method], nil
}

func TestSharedCymonkeyConformanceAcceptsBackendDescriptors(t *testing.T) {
	hello, _ := json.Marshal(Hello{ProtocolVersion: ProtocolVersion, Implementation: implementation{Name: "fixture"}, Backends: []BackendName{BackendCDP}})
	capabilities, _ := json.Marshal([]Capability{{
		Name: "dom.query", Backend: BackendCDP, Support: SupportMapped,
		Lifetime: LifetimeCall, Persistence: PersistenceEphemeral, Effect: "read", InputSchema: objectSchema("selector"),
	}})
	report, err := ValidateConformance(context.Background(), conformanceCaller{
		bridge.MethodHello: hello, bridge.MethodCapabilities: capabilities,
		bridge.MethodDescribe: json.RawMessage(`{"backend":"cdp"}`),
		bridge.MethodEvents:   json.RawMessage(`{"events":[],"cursor":"0"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Capabilities) != 1 || report.Capabilities[0].Name != "dom.query" {
		t.Fatalf("report = %#v", report)
	}
}

func TestSharedCymonkeyConformanceRejectsGenericBridgeProtocol(t *testing.T) {
	hello := json.RawMessage(`{"protocolVersion":"jangolova.bridge/v1alpha1","implementation":{"name":"wrong"},"backends":["cdp"]}`)
	_, err := ValidateConformance(context.Background(), conformanceCaller{bridge.MethodHello: hello})
	if err == nil {
		t.Fatal("ValidateConformance() error = nil")
	}
}
