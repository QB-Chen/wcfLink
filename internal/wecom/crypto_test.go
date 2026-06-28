package wecom

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptMessageRoundTrip(t *testing.T) {
	t.Parallel()

	encodingKey := strings.TrimRight(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), "=")
	key, err := DecodeAESKey(encodingKey)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext, err := EncryptMessage(key, "<xml><Content>hello</Content></xml>", "corp-id")
	if err != nil {
		t.Fatal(err)
	}
	msg, corpID, err := DecryptMessage(key, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "<xml><Content>hello</Content></xml>" || corpID != "corp-id" {
		t.Fatalf("unexpected decrypt result msg=%q corpID=%q", msg, corpID)
	}
}

func TestDecryptMessageRejectsMalformedCiphertext(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	if _, _, err := DecryptMessage(key, base64.StdEncoding.EncodeToString([]byte("not-a-block"))); err == nil {
		t.Fatal("expected malformed ciphertext to be rejected")
	}
}
