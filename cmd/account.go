package cmd

import (
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
)

func runAccountLogin(ctx context.Context, cmd *cli.Command) error {
	username := cmd.String("username")
	password := cmd.String("password")
	pdsHost := cmd.String("pds-host")
	plcHost := cmd.Root().String("plc-host")

	client, err := atproto.Login(ctx, username, password, pdsHost, plcHost, Version)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	sessResp, err := comatproto.ServerGetSession(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get session info: %w", err)
	}

	passAuth, ok := client.Auth.(*atclient.PasswordAuth)
	if !ok {
		return fmt.Errorf("unexpected auth type")
	}

	sess := atproto.AuthSession{
		DID:          passAuth.Session.AccountDID,
		PDS:          passAuth.Session.Host,
		Handle:       sessResp.Handle,
		Password:     password,
		AccessToken:  passAuth.Session.AccessToken,
		RefreshToken: passAuth.Session.RefreshToken,
	}
	if err := atproto.PersistAuthSession(&sess); err != nil {
		return fmt.Errorf("failed to persist session: %w", err)
	}

	w := cmd.Root().Writer
	fmt.Fprintf(w, "Logged in as %s (%s)\n", sessResp.Handle, sessResp.Did)
	return nil
}

func runAccountLogout(_ context.Context, cmd *cli.Command) error {
	if err := atproto.WipeAuthSession(); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Root().Writer, "Logged out")
	return nil
}

func runAccountStatus(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	sessResp, err := comatproto.ServerGetSession(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	w := cmd.Root().Writer
	fmt.Fprintf(w, "DID:    %s\n", sessResp.Did)
	fmt.Fprintf(w, "Handle: %s\n", sessResp.Handle)
	fmt.Fprintf(w, "PDS:    %s\n", client.Host)
	return nil
}
