package netguard

import (
	"context"
	"testing"
)

func TestValidateOutboundURLRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()

	cases := []string{
		"file:///etc/passwd",
		"http://localhost:8080",
		"http://127.0.0.1",
		"http://10.0.0.1",
		"http://172.16.0.1",
		"http://192.168.1.1",
		"http://169.254.169.254",
		"http://[::1]/",
		"http://user@example.com/",
	}
	for _, rawURL := range cases {
		rawURL := rawURL
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()
			if err := ValidateOutboundURL(context.Background(), rawURL); err == nil {
				t.Fatalf("expected %q to be rejected", rawURL)
			}
		})
	}
}

func TestValidateOutboundURLAllowsPublicAddress(t *testing.T) {
	t.Parallel()

	if err := ValidateOutboundURL(context.Background(), "http://93.184.216.34/path"); err != nil {
		t.Fatalf("expected public URL to be allowed: %v", err)
	}
}
