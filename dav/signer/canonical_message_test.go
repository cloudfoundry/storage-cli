package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestSignatureBindsEachFieldIndependently(t *testing.T) {
	signer := signer{secret: "fixture-signing-secret"}
	timestamp := int64(1700000000)
	original := signer.generateSignature("09/foo", "GET", time.Unix(timestamp, 0), 3600)
	shiftedTime := signer.generateSignature("09/foo", "GET", time.Unix(17000000003, 0), 600)
	shiftedPath := signer.generateSignature("09/foo1", "GET", time.Unix(700000000, 0), 3600)
	if original == shiftedTime || original == shiftedPath {
		t.Fatal("different signed field tuples have identical MAC")
	}
	mac := hmac.New(sha256.New, []byte("fixture-signing-secret"))
	if _, err := mac.Write([]byte("GET\n09/foo\n1700000000\n3600")); err != nil {
		t.Fatal(err)
	}
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if original != want {
		t.Fatal("canonical encoding differs from newline-delimited verifier protocol")
	}
}
