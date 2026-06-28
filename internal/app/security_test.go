package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateListenSecurityRequiresTokenForPublicBind(t *testing.T) {
	t.Parallel()

	if err := validateListenSecurity("0.0.0.0:17890", ""); err == nil {
		t.Fatal("expected public bind without token to be rejected")
	}
	if err := validateListenSecurity("0.0.0.0:17890", "secret"); err != nil {
		t.Fatalf("expected public bind with token to be allowed: %v", err)
	}
	if err := validateListenSecurity("127.0.0.1:17890", ""); err != nil {
		t.Fatalf("expected loopback bind without token to be allowed: %v", err)
	}
}

func TestValidateLocalMediaPathRestrictsRootAndSize(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	allowed := filepath.Join(root, "allowed.txt")
	if err := os.WriteFile(allowed, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := validateLocalMediaPath("allowed.txt", root, 10)
	if err != nil {
		t.Fatalf("expected file under root to be allowed: %v", err)
	}
	allowed, err = filepath.EvalSymlinks(allowed)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != allowed {
		t.Fatalf("expected resolved path %q, got %q", allowed, resolved)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateLocalMediaPath(outside, root, 10); err == nil {
		t.Fatal("expected file outside root to be rejected")
	}
	if _, err := validateLocalMediaPath("allowed.txt", root, 1); err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
}
