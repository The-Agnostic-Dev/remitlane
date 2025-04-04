package partners

import (
	"strings"
	"testing"
)

func TestHashKeyTrims(t *testing.T) {
	if HashKey(" abc ") != HashKey("abc") {
		t.Error("hash should ignore surrounding space")
	}
	if len(HashKey("abc")) != 64 {
		t.Error("want hex sha256")
	}
}

func TestNewKey(t *testing.T) {
	k := NewKey("live")
	if !strings.HasPrefix(k, "rl_live_") || len(k) != len("rl_live_")+32 {
		t.Errorf("bad key %q", k)
	}
	if Prefix(k) != k[:16] {
		t.Error("prefix")
	}
}
