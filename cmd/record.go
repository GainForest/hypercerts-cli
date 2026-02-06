package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bluesky-social/indigo/api/agnostic"
	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/urfave/cli/v3"
)

func runRecordGet(ctx context.Context, cmd *cli.Command) error {
	uriArg := cmd.Args().First()
	if uriArg == "" {
		return fmt.Errorf("expected AT-URI argument")
	}

	aturi, err := syntax.ParseATURI(uriArg)
	if err != nil {
		return fmt.Errorf("not a valid AT-URI: %v", err)
	}

	dir := configDirectory(cmd)
	ident, err := dir.Lookup(ctx, aturi.Authority())
	if err != nil {
		return err
	}

	c := atclient.NewAPIClient(ident.PDSEndpoint())
	c.Headers.Set("User-Agent", userAgentString())

	resp, err := agnostic.RepoGetRecord(ctx, c, "", aturi.Collection().String(), ident.DID.String(), aturi.RecordKey().String())
	if err != nil {
		return err
	}
	if resp.Value == nil {
		return fmt.Errorf("empty record value")
	}

	var record map[string]any
	if err := json.Unmarshal(*resp.Value, &record); err != nil {
		return err
	}

	fmt.Fprintln(cmd.Root().Writer, prettyJSON(record))
	return nil
}

func runRecordList(ctx context.Context, cmd *cli.Command) error {
	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("provide an account identifier as argument")
	}

	ident, err := resolveIdent(ctx, cmd, username)
	if err != nil {
		return err
	}

	c := atclient.NewAPIClient(ident.PDSEndpoint())
	c.Headers.Set("User-Agent", userAgentString())
	if c.Host == "" {
		return fmt.Errorf("no PDS endpoint for identity")
	}

	desc, err := comatproto.RepoDescribeRepo(ctx, c, ident.DID.String())
	if err != nil {
		return err
	}

	w := cmd.Root().Writer

	if cmd.Bool("collections") {
		for _, nsid := range desc.Collections {
			fmt.Fprintln(w, nsid)
		}
		return nil
	}

	collections := desc.Collections
	filter := cmd.String("collection")
	if filter != "" {
		collections = []string{filter}
	}

	for _, nsid := range collections {
		cursor := ""
		for {
			resp, err := agnostic.RepoListRecords(ctx, c, nsid, cursor, 100, ident.DID.String(), false)
			if err != nil {
				return err
			}
			for _, rec := range resp.Records {
				aturi, err := syntax.ParseATURI(rec.Uri)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", aturi.Collection(), aturi.RecordKey(), rec.Cid)
			}
			if resp.Cursor != nil && *resp.Cursor != "" {
				cursor = *resp.Cursor
			} else {
				break
			}
		}
	}
	return nil
}

func runResolve(ctx context.Context, cmd *cli.Command) error {
	s := cmd.Args().First()
	if s == "" {
		return fmt.Errorf("provide an account identifier")
	}

	atid, err := syntax.ParseAtIdentifier(s)
	if err != nil {
		return err
	}

	bdir := identity.BaseDirectory{
		PLCURL:    cmd.Root().String("plc-host"),
		UserAgent: userAgentString(),
	}

	w := cmd.Root().Writer

	if atid.IsDID() {
		did, err := atid.AsDID()
		if err != nil {
			return err
		}
		if cmd.Bool("did") {
			fmt.Fprintln(w, did)
			return nil
		}
		raw, err := bdir.ResolveDIDRaw(ctx, did)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, prettyJSON(json.RawMessage(raw)))
		return nil
	}

	handle, err := atid.AsHandle()
	if err != nil {
		return err
	}
	did, err := bdir.ResolveHandle(ctx, handle)
	if err != nil {
		return err
	}
	if cmd.Bool("did") {
		fmt.Fprintln(w, did)
		return nil
	}
	raw, err := bdir.ResolveDIDRaw(ctx, did)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, prettyJSON(json.RawMessage(raw)))
	return nil
}
