package zdns

import (
	"testing"
)

func TestPairing(t *testing.T) {
	name := "TestDevice"
	_, code, err := CreateInvite(name)
	if err != nil {
		t.Fatalf("CreateInvite failed: %v", err)
	}

	invite, err := ParseInvite(code)
	if err != nil {
		t.Fatalf("ParseInvite failed: %v", err)
	}

	if invite.Name != name {
		t.Errorf("Expected name %s, got %s", name, invite.Name)
	}

	if len(invite.SharedSecret) != 32 {
		t.Errorf("Expected 32 byte secret, got %d", len(invite.SharedSecret))
	}
}
