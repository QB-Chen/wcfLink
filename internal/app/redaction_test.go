package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/QB-Chen/wcfLink/internal/ilink"
	"github.com/QB-Chen/wcfLink/internal/model"
)

func TestOutboundRawJSONOmitsContextToken(t *testing.T) {
	textRaw := outboundTextRawJSON("user1", "hello")
	if strings.Contains(textRaw, "context_token") || strings.Contains(textRaw, "secret-context") {
		t.Fatalf("outbound text raw json leaked context token: %s", textRaw)
	}
	if !strings.Contains(textRaw, "hello") || !strings.Contains(textRaw, "user1") {
		t.Fatalf("outbound text raw json omitted expected fields: %s", textRaw)
	}

	mediaRaw := outboundMediaRawJSON("user1", "/safe/file.txt", "file", "caption")
	if strings.Contains(mediaRaw, "context_token") || strings.Contains(mediaRaw, "secret-context") {
		t.Fatalf("outbound media raw json leaked context token: %s", mediaRaw)
	}
	if !strings.Contains(mediaRaw, "caption") || !strings.Contains(mediaRaw, "/safe/file.txt") {
		t.Fatalf("outbound media raw json omitted expected fields: %s", mediaRaw)
	}
}

func TestInboundLogPayloadRedactsContextToken(t *testing.T) {
	account := model.Account{AccountID: "acct1", BaseURL: "https://example.invalid"}
	msg := ilink.WeixinMessage{
		MessageID:    42,
		FromUserID:   "user1",
		ToUserID:     "bot1",
		ContextToken: "secret-context",
	}

	webhookPayload, err := json.Marshal(inboundPayload(account, msg, "", "", "", false))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(webhookPayload), "secret-context") {
		t.Fatalf("webhook payload should preserve context token for compatibility: %s", webhookPayload)
	}

	logPayload, err := json.Marshal(inboundPayload(account, msg, "", "", "", true))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logPayload), "secret-context") || strings.Contains(string(logPayload), "context_token") {
		t.Fatalf("log payload leaked context token: %s", logPayload)
	}
	if !strings.Contains(string(logPayload), "acct1") || !strings.Contains(string(logPayload), "user1") {
		t.Fatalf("log payload omitted expected routing fields: %s", logPayload)
	}
}
