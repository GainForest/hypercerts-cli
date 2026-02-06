package atproto

import (
	"errors"
	"testing"

	"github.com/adrg/xdg"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

func setupTestXDG(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)
	xdg.Reload()
}

func TestPersistAndLoadAuthSession(t *testing.T) {
	setupTestXDG(t)

	sess := &AuthSession{
		DID:          syntax.DID("did:plc:test123"),
		PDS:          "https://pds.example.com",
		Handle:       "alice.example.com",
		Password:     "secret",
		AccessToken:  "at_token",
		RefreshToken: "rt_token",
	}

	if err := PersistAuthSession(sess); err != nil {
		t.Fatalf("PersistAuthSession: %v", err)
	}

	loaded, err := LoadAuthSessionFile()
	if err != nil {
		t.Fatalf("LoadAuthSessionFile: %v", err)
	}

	if loaded.DID != sess.DID {
		t.Errorf("DID: got %q, want %q", loaded.DID, sess.DID)
	}
	if loaded.PDS != sess.PDS {
		t.Errorf("PDS: got %q, want %q", loaded.PDS, sess.PDS)
	}
	if loaded.Handle != sess.Handle {
		t.Errorf("Handle: got %q, want %q", loaded.Handle, sess.Handle)
	}
	if loaded.Password != sess.Password {
		t.Errorf("Password: got %q, want %q", loaded.Password, sess.Password)
	}
	if loaded.AccessToken != sess.AccessToken {
		t.Errorf("AccessToken: got %q, want %q", loaded.AccessToken, sess.AccessToken)
	}
	if loaded.RefreshToken != sess.RefreshToken {
		t.Errorf("RefreshToken: got %q, want %q", loaded.RefreshToken, sess.RefreshToken)
	}
}

func TestWipeAuthSession(t *testing.T) {
	setupTestXDG(t)

	sess := &AuthSession{
		DID:      syntax.DID("did:plc:test123"),
		PDS:      "https://pds.example.com",
		Password: "secret",
	}
	if err := PersistAuthSession(sess); err != nil {
		t.Fatalf("PersistAuthSession: %v", err)
	}

	if err := WipeAuthSession(); err != nil {
		t.Fatalf("WipeAuthSession: %v", err)
	}

	_, err := LoadAuthSessionFile()
	if !errors.Is(err, ErrNoAuthSession) {
		t.Errorf("expected ErrNoAuthSession, got: %v", err)
	}
}

func TestLoadAuthSessionFile_missing(t *testing.T) {
	setupTestXDG(t)

	_, err := LoadAuthSessionFile()
	if !errors.Is(err, ErrNoAuthSession) {
		t.Errorf("expected ErrNoAuthSession, got: %v", err)
	}
}

func TestWipeAuthSession_noFile(t *testing.T) {
	setupTestXDG(t)

	// Should not error when file doesn't exist
	if err := WipeAuthSession(); err != nil {
		t.Errorf("WipeAuthSession with no file: %v", err)
	}
}
