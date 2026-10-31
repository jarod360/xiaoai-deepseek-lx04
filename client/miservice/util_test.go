package miservice

import (
	"encoding/base64"
	"testing"
)

func TestMiotNonceAndDeviceId(t *testing.T) {
	first, second := getRandom(16), getRandom(16)
	if len(first) != 16 || len(second) != 16 || first == second {
		t.Fatalf("invalid device identifiers")
	}
	a, b := genNonce(), genNonce()
	decoded, err := base64.StdEncoding.DecodeString(a)
	if err != nil || len(decoded) != 12 || a == b {
		t.Fatalf("invalid or repeated MIoT nonce")
	}
}
