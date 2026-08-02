package grimlock

import "testing"

func TestModelProfileValidatesCallerOwnedGateway(t *testing.T) {
	t.Parallel()
	valid := ModelProfile{
		APIVersion: ModelAPIVersion, ProfileID: "application-model",
		Protocol: OpenAICompatibleProtocol, Endpoint: "https://gateway.example/v1",
		Model: "approved-model", CredentialRef: "application-credential", TLSRef: "private-trust",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	local := valid
	local.Endpoint = "http://127.0.0.1:8080/v1"
	if err := local.Validate(); err != nil {
		t.Fatalf("loopback Validate() error = %v", err)
	}
}

func TestModelProfileRejectsUnsafeOrInlineConnectionMaterial(t *testing.T) {
	t.Parallel()
	base := ModelProfile{
		APIVersion: ModelAPIVersion, ProfileID: "application-model",
		Protocol: OpenAICompatibleProtocol, Endpoint: "https://gateway.example/v1",
		Model: "approved-model", CredentialRef: "application-credential",
	}
	tests := map[string]ModelProfile{
		"remote plaintext":             func() ModelProfile { value := base; value.Endpoint = "http://gateway.example/v1"; return value }(),
		"inline user info":             func() ModelProfile { value := base; value.Endpoint = "https://token@gateway.example/v1"; return value }(),
		"fragment":                     func() ModelProfile { value := base; value.Endpoint += "#secret"; return value }(),
		"query credential":             func() ModelProfile { value := base; value.Endpoint += "?api_key=secret"; return value }(),
		"missing credential reference": func() ModelProfile { value := base; value.CredentialRef = ""; return value }(),
		"path credential reference":    func() ModelProfile { value := base; value.CredentialRef = "../credential"; return value }(),
	}
	for name, profile := range tests {
		t.Run(name, func(t *testing.T) {
			if err := profile.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}
