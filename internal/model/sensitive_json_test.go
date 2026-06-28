package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSensitiveFieldsOmittedFromJSON(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{
			name: "peer context",
			value: PeerContext{
				AccountID:    "acct1",
				PeerUserID:   "user1",
				ContextToken: "secret-context",
			},
		},
		{
			name: "ilink event",
			value: Event{
				ID:           1,
				AccountID:    "acct1",
				Direction:    "inbound",
				ContextToken: "secret-context",
				RawJSON:      `{"context_token":"secret-context"}`,
			},
		},
		{
			name: "wecom event",
			value: WeComEvent{
				ID:      1,
				CorpID:  "corp1",
				RawJSON: `{"secret":"hidden"}`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			body := string(data)
			for _, forbidden := range []string{"secret-context", "context_token", "raw_json", "hidden"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("JSON leaked %q: %s", forbidden, body)
				}
			}
		})
	}
}
