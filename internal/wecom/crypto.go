package wecom

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

func VerifySignature(token, timestamp, nonce, encrypt string, msgSignature string) bool {
	return ComputeSignature(token, timestamp, nonce, encrypt) == msgSignature
}

func ComputeSignature(token, timestamp, nonce, encrypt string) string {
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	joined := strings.Join(parts, "")
	h := sha1.New()
	h.Write([]byte(joined))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func DecodeAESKey(encodingAESKey string) ([]byte, error) {
	raw := encodingAESKey + "="
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode AES key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid AES key length: expected 32, got %d", len(key))
	}
	return key, nil
}

func DecryptMessage(aesKey []byte, cipherTextBase64 string) (string, string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return "", "", fmt.Errorf("decode ciphertext: %w", err)
	}
	if len(ciphertext) < aes.BlockSize {
		return "", "", fmt.Errorf("ciphertext too short")
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", "", fmt.Errorf("ciphertext size is not a multiple of block size")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", "", fmt.Errorf("create cipher: %w", err)
	}
	iv := aesKey[:16]
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", "", err
	}

	if len(plaintext) < 20 {
		return "", "", fmt.Errorf("decrypted data too short")
	}

	msgLen := binary.BigEndian.Uint32(plaintext[16:20])
	msgEnd := 20 + msgLen
	if int(msgEnd) > len(plaintext) {
		return "", "", fmt.Errorf("message length exceeds data")
	}
	msg := string(plaintext[20:msgEnd])
	corpID := string(plaintext[msgEnd:])
	return msg, corpID, nil
}

func EncryptMessage(aesKey []byte, plainText, corpID string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	msgBytes := []byte(plainText)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msgBytes)))
	corpBytes := []byte(corpID)

	raw := make([]byte, 0, 16+4+len(msgBytes)+len(corpBytes))
	raw = append(raw, random...)
	raw = append(raw, lenBuf...)
	raw = append(raw, msgBytes...)
	raw = append(raw, corpBytes...)

	padded := pkcs7Pad(raw, 32)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	iv := aesKey[:16]
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding == 0 {
		padding = blockSize
	}
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > 32 || pad > len(data) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for i := len(data) - pad; i < len(data); i++ {
		if data[i] != byte(pad) {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return data[:len(data)-pad], nil
}
