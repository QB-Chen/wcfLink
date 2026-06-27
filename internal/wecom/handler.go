package wecom

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type InboundHandler func(ctx context.Context, account AccountConfig, msg InboundMessage)

type CallbackHandler struct {
	accounts []AccountConfig
	aesKeys  map[string][]byte // corpID -> aesKey
	logger   *slog.Logger
	onMsg    InboundHandler
}

func NewCallbackHandler(accounts []AccountConfig, logger *slog.Logger, onMsg InboundHandler) (*CallbackHandler, error) {
	aesKeys := make(map[string][]byte, len(accounts))
	for _, acc := range accounts {
		if acc.CallbackAESKey == "" {
			continue
		}
		key, err := DecodeAESKey(acc.CallbackAESKey)
		if err != nil {
			return nil, fmt.Errorf("decode AES key for corp %s: %w", acc.CorpID, err)
		}
		aesKeys[acc.CorpID] = key
	}
	return &CallbackHandler{
		accounts: accounts,
		aesKeys:  aesKeys,
		logger:   logger,
		onMsg:    onMsg,
	}, nil
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSignature := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if r.Method == http.MethodGet {
		h.handleVerify(w, msgSignature, timestamp, nonce, echostr)
		return
	}
	if r.Method == http.MethodPost {
		h.handleMessage(w, r, msgSignature, timestamp, nonce)
		return
	}
	w.Header().Set("Allow", "GET, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (h *CallbackHandler) handleVerify(w http.ResponseWriter, msgSignature, timestamp, nonce, echostr string) {
	if echostr == "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("wecom callback ok"))
		return
	}

	account, aesKey := h.matchAccount(msgSignature, timestamp, nonce, echostr)
	if account == nil {
		h.logger.Warn("wecom callback: URL verification failed, signature mismatch")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	msg, _, err := DecryptMessage(aesKey, echostr)
	if err != nil {
		h.logger.Error("wecom callback: decrypt echostr failed", "err", err)
		http.Error(w, "decrypt failed", http.StatusInternalServerError)
		return
	}
	h.logger.Info("wecom callback: URL verified", "corp_id", account.CorpID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(msg))
}

func (h *CallbackHandler) handleMessage(w http.ResponseWriter, r *http.Request, msgSignature, timestamp, nonce string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.logger.Warn("wecom callback: read body failed", "err", err)
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	encBody, err := ParseEncryptedBody(body)
	if err != nil {
		h.logger.Warn("wecom callback: parse XML failed", "err", err)
		http.Error(w, "invalid xml", http.StatusBadRequest)
		return
	}

	account, aesKey := h.matchAccount(msgSignature, timestamp, nonce, encBody.Encrypt)
	if account == nil {
		h.logger.Warn("wecom callback: signature mismatch")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))

	xmlStr, corpID, err := DecryptMessage(aesKey, encBody.Encrypt)
	if err != nil {
		h.logger.Error("wecom callback: decrypt failed", "corp_id", account.CorpID, "err", err)
		return
	}
	_ = corpID

	xmlMsg, err := ParseDecryptedMessage(xmlStr)
	if err != nil {
		h.logger.Error("wecom callback: parse decrypted XML failed", "corp_id", account.CorpID, "err", err)
		return
	}

	inbound := XMLMessageToInbound(xmlMsg)
	h.logger.Info("wecom inbound",
		"corp_id", account.CorpID,
		"from", inbound.FromUserName,
		"msg_type", inbound.MsgType,
		"content", truncate(inbound.Content, 80),
	)

	if h.onMsg != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		h.onMsg(ctx, *account, inbound)
	}
}

func (h *CallbackHandler) matchAccount(msgSignature, timestamp, nonce, encrypt string) (*AccountConfig, []byte) {
	for i, acc := range h.accounts {
		if acc.CallbackToken == "" {
			continue
		}
		if VerifySignature(acc.CallbackToken, timestamp, nonce, encrypt, msgSignature) {
			if aesKey, ok := h.aesKeys[acc.CorpID]; ok {
				return &h.accounts[i], aesKey
			}
		}
	}
	return nil, nil
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
