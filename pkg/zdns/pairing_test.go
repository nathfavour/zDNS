package zdns

import (
	"testing"
)

func TestPairing(t *testing.T) {
	name := "TestDevice"
	_, pub, _ := GenerateLongTermKey()
	code, err := CreateInvite(name, pub)
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
}
