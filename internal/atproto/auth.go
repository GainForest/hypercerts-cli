package atproto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"github.com/adrg/xdg"
)

// ErrNoAuthSession is returned when no auth session file is found.
var ErrNoAuthSession = errors.New("no auth session found")

// AuthSession represents a persisted authentication session.
type AuthSession struct {
	DID          syntax.DID `json:"did"`
	PDS          string     `json:"pds"`
	Handle       string     `json:"handle"`
	Password     string     `json:"password"`
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
}

// PersistAuthSession saves the auth session to XDG state directory.
func PersistAuthSession(sess *AuthSession) error {
	fPath, err := xdg.StateFile("hc/auth-session.json")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	authBytes, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	_, err = f.Write(authBytes)
	return err
}

// LoadAuthSessionFile loads the auth session from XDG state directory.
func LoadAuthSessionFile() (*AuthSession, error) {
	fPath, err := xdg.SearchStateFile("hc/auth-session.json")
	if err != nil {
		return nil, ErrNoAuthSession
	}
	fBytes, err := os.ReadFile(fPath)
	if err != nil {
		return nil, err
	}
	var sess AuthSession
	if err := json.Unmarshal(fBytes, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// WipeAuthSession deletes the auth session file.
func WipeAuthSession() error {
	fPath, err := xdg.SearchStateFile("hc/auth-session.json")
	if err != nil {
		return nil // file doesn't exist
	}
	return os.Remove(fPath)
}

// authRefreshCallback is called when tokens are refreshed.
func authRefreshCallback(_ context.Context, data atclient.PasswordSessionData) {
	sess, _ := LoadAuthSessionFile()
	if sess == nil {
		sess = &AuthSession{}
	}
	sess.DID = data.AccountDID
	sess.AccessToken = data.AccessToken
	sess.RefreshToken = data.RefreshToken
	sess.PDS = data.Host
	if err := PersistAuthSession(sess); err != nil {
		slog.Warn("failed to save refreshed auth session data", "err", err)
	}
}

// ConfigDirectory returns an identity directory configured with PLC host and user agent.
func ConfigDirectory(plcHost, version string) identity.Directory {
	dir := identity.DefaultDirectory()
	cdir, ok := dir.(*identity.CacheDirectory)
	if ok {
		bdir, ok := cdir.Inner.(*identity.BaseDirectory)
		if ok {
			bdir.UserAgent = fmt.Sprintf("hc/%s", version)
			if plcHost != "" {
				bdir.PLCURL = plcHost
			}
		}
	}
	return dir
}

// Login authenticates with a PDS using username and password.
func Login(ctx context.Context, username, password, pdsHost, plcHost, version string) (*atclient.APIClient, error) {
	if pdsHost != "" {
		return atclient.LoginWithPasswordHost(ctx, pdsHost, username, password, "", authRefreshCallback)
	}
	atid, err := syntax.ParseAtIdentifier(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}
	dir := ConfigDirectory(plcHost, version)
	return atclient.LoginWithPassword(ctx, dir, atid, password, "", authRefreshCallback)
}

// LoadAuthClient loads an auth client from the saved session.
func LoadAuthClient(ctx context.Context, plcHost, version string) (*atclient.APIClient, error) {
	sess, err := LoadAuthSessionFile()
	if err != nil {
		return nil, err
	}
	client := atclient.ResumePasswordSession(atclient.PasswordSessionData{
		AccessToken:  sess.AccessToken,
		RefreshToken: sess.RefreshToken,
		AccountDID:   sess.DID,
		Host:         sess.PDS,
	}, authRefreshCallback)
	_, err = comatproto.ServerGetSession(ctx, client)
	if err == nil {
		return client, nil
	}
	dir := ConfigDirectory(plcHost, version)
	return atclient.LoginWithPassword(ctx, dir, sess.DID.AtIdentifier(), sess.Password, "", authRefreshCallback)
}

// LoginOrLoad checks for username/password first, then falls back to loading a saved session.
func LoginOrLoad(ctx context.Context, username, password, plcHost, version string) (*atclient.APIClient, error) {
	if username != "" && password != "" {
		dir := ConfigDirectory(plcHost, version)
		atid, err := syntax.ParseAtIdentifier(username)
		if err != nil {
			return nil, err
		}
		return atclient.LoginWithPassword(ctx, dir, atid, password, "", nil)
	}
	return LoadAuthClient(ctx, plcHost, version)
}
