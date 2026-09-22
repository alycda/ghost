package server

import (
	"regexp"
	"testing"
)

func TestScramVerifierShape(t *testing.T) {
	v, err := scramVerifier("secret")
	if err != nil {
		t.Fatal(err)
	}
	// iterations:salt$StoredKey:ServerKey, salt 16 bytes, keys 32 bytes.
	re := regexp.MustCompile(`^SCRAM-SHA-256\$4096:[A-Za-z0-9+/]{22}==\$[A-Za-z0-9+/]{43}=:[A-Za-z0-9+/]{43}=$`)
	if !re.MatchString(v) {
		t.Fatalf("unexpected verifier shape: %s", v)
	}
	again, _ := scramVerifier("secret")
	if again == v {
		t.Fatal("two verifiers for one password share a salt")
	}
}
