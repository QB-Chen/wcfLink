package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/QB-Chen/wcfLink/internal/ilink"
)

func TestSaveInboundMessageKeepsPeerContextButRedactsRawEventJSON(t *testing.T) {
	ctx := context.Background()
	st, err := New(ctx, t.TempDir()+"/wcfLink.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	msg := ilink.WeixinMessage{
		MessageID:    123,
		FromUserID:   "user1",
		ToUserID:     "bot1",
		ContextToken: "secret-context",
		ItemList: []ilink.MessageItem{{
			Type: ilink.MessageItemTypeText,
			TextItem: &ilink.TextItem{
				Text: "hello",
			},
		}},
	}
	if err := st.SaveInboundMessage(ctx, "acct1", msg, "", "", ""); err != nil {
		t.Fatal(err)
	}

	peerCtx, err := st.GetPeerContext(ctx, "acct1", "user1")
	if err != nil {
		t.Fatal(err)
	}
	if peerCtx.ContextToken != "secret-context" {
		t.Fatalf("peer context token not preserved for replies: %q", peerCtx.ContextToken)
	}

	var rawJSON string
	if err := st.DB().QueryRowContext(ctx, `SELECT raw_json FROM events WHERE account_id = ?`, "acct1").Scan(&rawJSON); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rawJSON, "secret-context") || strings.Contains(rawJSON, "context_token") {
		t.Fatalf("stored raw event JSON leaked context token: %s", rawJSON)
	}

	events, err := st.ListEvents(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-context") || strings.Contains(string(data), "context_token") || strings.Contains(string(data), "raw_json") {
		t.Fatalf("event API JSON leaked sensitive fields: %s", data)
	}
}
