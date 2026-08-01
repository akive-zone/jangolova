package chromium

import "testing"

func TestValidateLaunchHostRequiresExplicitRemoteOptIn(t *testing.T) {
	t.Parallel()

	if err := validateLaunchHost("127.0.0.1", false); err != nil {
		t.Fatal(err)
	}
	if err := validateLaunchHost("0.0.0.0", false); err == nil {
		t.Fatal("non-loopback debugging was accepted without opt-in")
	}
	if err := validateLaunchHost("0.0.0.0", true); err != nil {
		t.Fatalf("explicit container debugging opt-in was rejected: %v", err)
	}
}
