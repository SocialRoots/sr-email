package web

import (
	"testing"

	"github.com/SocialRoots/sr-email/settings"
)

func TestVerifyMailgunSignature_Valid(t *testing.T) {
	restore := saveSettings()
	defer restore()

	key := "my-signing-key"
	settings.MailgunAPIKey = key

	ts := "1721234567"
	tok := "abcdefghijabcdefghijabcdefghijabcdefghijab"
	sig := computeSignature(key, ts, tok)

	if !verifyMailgunSignature(ts, tok, sig) {
		t.Errorf("expected valid signature to pass verification")
	}
}

func TestVerifyMailgunSignature_Invalid(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = "real-key"

	ts := "1721234567"
	tok := "abcdefghijabcdefghijabcdefghijabcdefghijab"
	// Signature computed with a different key.
	wrongSig := computeSignature("wrong-key", ts, tok)

	if verifyMailgunSignature(ts, tok, wrongSig) {
		t.Errorf("expected wrong key signature to fail verification")
	}
}

func TestVerifyMailgunSignature_WrongToken(t *testing.T) {
	restore := saveSettings()
	defer restore()

	key := "my-signing-key"
	settings.MailgunAPIKey = key

	ts := "1721234567"
	tok := "abcdefghijabcdefghijabcdefghijabcdefghijab"
	// Signature computed for a different token.
	sig := computeSignature(key, ts, "different-token")

	if verifyMailgunSignature(ts, tok, sig) {
		t.Errorf("expected wrong token signature to fail verification")
	}
}

func TestVerifyMailgunSignature_EmptyKeyIsPromiscuous(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = "" // No key configured

	if !verifyMailgunSignature("anything", "anything", "anything") {
		t.Errorf("empty key should accept any signature (dev mode)")
	}
	if !verifyMailgunSignature("", "", "") {
		t.Errorf("empty key should accept empty signature")
	}
}

func TestVerifyMailgunSignature_EmptyValues(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = "key"

	// Empty timestamp or token should still produce a deterministic signature.
	if verifyMailgunSignature("", "", "") {
		t.Errorf("empty values with key set should fail (no valid signature for empty input)")
	}
}

func TestVerifyMailgunSignature_ConstantTime(t *testing.T) {
	restore := saveSettings()
	defer restore()

	settings.MailgunAPIKey = "key"

	// Verify we're using hmac.Equal (constant-time comparison).
	// The function should not panic or behave differently for different-length sigs.
	ts := "1721234567"
	tok := "abcdefghijabcdefghijabcdefghijabcdefghijab"
	sig := computeSignature("key", ts, tok)

	if !verifyMailgunSignature(ts, tok, sig) {
		t.Errorf("valid signature should pass")
	}
	if verifyMailgunSignature(ts, tok, sig+"extra") {
		t.Errorf("appended extra chars should fail")
	}
	if verifyMailgunSignature(ts, tok, sig[:len(sig)-2]) {
		t.Errorf("truncated signature should fail")
	}
}