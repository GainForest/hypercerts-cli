package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
	"github.com/GainForest/hypercerts-cli/internal/menu"
	"github.com/GainForest/hypercerts-cli/internal/style"
)

// --- Badge Definition ---

type badgeDefinitionOption struct {
	URI         string
	CID         string
	Rkey        string
	Title       string
	BadgeType   string
	Description string
	Created     string
}

func fetchBadgeDefinitions(ctx context.Context, client *atclient.APIClient, did string) ([]badgeDefinitionOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeDefinition)
	if err != nil {
		return nil, fmt.Errorf("failed to list badge definitions: %w", err)
	}
	var result []badgeDefinitionOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, badgeDefinitionOption{
			URI:         e.URI,
			CID:         e.CID,
			Rkey:        string(aturi.RecordKey()),
			Title:       mapStr(e.Value, "title"),
			BadgeType:   mapStr(e.Value, "badgeType"),
			Description: mapStr(e.Value, "description"),
			Created:     created,
		})
	}
	return result, nil
}

func runBadgeDefinitionCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	title := cmd.String("title")
	badgeType := cmd.String("type")
	description := cmd.String("description")

	if title == "" && badgeType == "" {
		// Interactive mode
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Title").
					Description("Badge title (max 256 chars)").
					CharLimit(256).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("title is required")
						}
						return nil
					}).
					Value(&title),

				huh.NewInput().
					Title("Badge Type").
					Description("Type identifier (max 100 chars)").
					CharLimit(100).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("badge type is required")
						}
						return nil
					}).
					Value(&badgeType),

				huh.NewInput().
					Title("Description").
					Description("Optional, max 5000 chars").
					CharLimit(5000).
					Value(&description),
			).Title("Badge Definition"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	} else {
		// Non-interactive: prompt for missing required fields
		if title == "" {
			err = huh.NewInput().Title("Title").Description("Badge title (max 256 chars)").CharLimit(256).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("title is required")
					}
					return nil
				}).Value(&title).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
		if badgeType == "" {
			err = huh.NewInput().Title("Badge Type").Description("Type identifier (max 100 chars)").CharLimit(100).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("badge type is required")
					}
					return nil
				}).Value(&badgeType).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
	}

	record := map[string]any{
		"$type":     atproto.CollectionBadgeDefinition,
		"title":     title,
		"badgeType": badgeType,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	if description != "" {
		record["description"] = description
	}

	// Note: icon field is required by lexicon but CLI doesn't support blob upload yet
	fmt.Fprintf(w, "\033[33m⚠\033[0m  Warning: Badge icon field is required by lexicon but not yet supported by CLI.\n")
	fmt.Fprintf(w, "   This record may not validate on the server without an icon.\n\n")

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionBadgeDefinition, record)
	if err != nil {
		return fmt.Errorf("failed to create badge definition: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Created badge definition: %s\n", uri)
	return nil
}

func runBadgeDefinitionList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeDefinition)
	if err != nil {
		return fmt.Errorf("failed to list badge definitions: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-30s %-20s %s\033[0m\n", "ID", "TITLE", "TYPE", "CREATED")
	fmt.Fprintf(w, "%-15s %-30s %-20s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 28),
		strings.Repeat("-", 18), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		title := mapStr(e.Value, "title")
		badgeType := mapStr(e.Value, "badgeType")

		if len(title) > 28 {
			title = title[:25] + "..."
		}
		if len(badgeType) > 18 {
			badgeType = badgeType[:15] + "..."
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-30s %-20s %s\n", id, title, badgeType, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no badge definitions found)\033[0m")
	}
	return nil
}

func runBadgeDefinitionGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionBadgeDefinition, "badge definition")
}

func runBadgeDefinitionDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	id := cmd.String("id")
	if id == "" {
		id = cmd.Args().First()
	}

	if id == "" {
		badges, err := fetchBadgeDefinitions(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, badges, "badge definition",
			func(b badgeDefinitionOption) string { return b.Title },
			func(b badgeDefinitionOption) string { return b.BadgeType },
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "badge definition") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, b := range selected {
			aturi, _ := syntax.ParseATURI(b.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted badge definition: %s\n", b.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionBadgeDefinition, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete badge definition %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete badge definition: %w", err)
	}
	fmt.Fprintf(w, "Deleted badge definition: %s\n", extractRkey(uri))
	return nil
}

// --- Badge Award ---

type badgeAwardOption struct {
	URI     string
	CID     string
	Rkey    string
	Badge   string
	Subject string
	Note    string
	Created string
}

func fetchBadgeAwards(ctx context.Context, client *atclient.APIClient, did string) ([]badgeAwardOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeAward)
	if err != nil {
		return nil, fmt.Errorf("failed to list badge awards: %w", err)
	}
	var result []badgeAwardOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		badgeURI := ""
		if badge := mapMap(e.Value, "badge"); badge != nil {
			badgeURI = mapStr(badge, "uri")
		}

		subjectStr := ""
		if subject := mapMap(e.Value, "subject"); subject != nil {
			if subjectType := mapStr(subject, "$type"); subjectType == "app.certified.defs#did" {
				subjectStr = mapStr(subject, "did")
			} else {
				subjectStr = mapStr(subject, "uri")
			}
		}

		result = append(result, badgeAwardOption{
			URI:     e.URI,
			CID:     e.CID,
			Rkey:    string(aturi.RecordKey()),
			Badge:   badgeURI,
			Subject: subjectStr,
			Note:    mapStr(e.Value, "note"),
			Created: created,
		})
	}
	return result, nil
}

func runBadgeAwardCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	badgeURI := cmd.String("badge")
	subjectStr := cmd.String("subject")
	note := cmd.String("note")
	url := cmd.String("url")

	if badgeURI == "" || subjectStr == "" {
		// Interactive mode
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Badge AT-URI").
					Description("AT-URI of badge definition").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("badge URI is required")
						}
						return nil
					}).
					Value(&badgeURI),

				huh.NewInput().
					Title("Subject").
					Description("DID or AT-URI of recipient").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("subject is required")
						}
						return nil
					}).
					Value(&subjectStr),

				huh.NewInput().
					Title("Note").
					Description("Optional note (max 500 chars)").
					CharLimit(500).
					Value(&note),

				huh.NewInput().
					Title("URL").
					Description("Optional URL (max 2048 chars)").
					CharLimit(2048).
					Value(&url),
			).Title("Badge Award"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	} else {
		// Non-interactive: prompt for missing required fields
		if badgeURI == "" {
			err = huh.NewInput().Title("Badge AT-URI").Description("AT-URI of badge definition").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("badge URI is required")
					}
					return nil
				}).Value(&badgeURI).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
		if subjectStr == "" {
			err = huh.NewInput().Title("Subject").Description("DID or AT-URI of recipient").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("subject is required")
					}
					return nil
				}).Value(&subjectStr).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
	}

	// Get badge CID for strongRef
	badgeAturi, err := syntax.ParseATURI(badgeURI)
	if err != nil {
		return fmt.Errorf("invalid badge URI: %w", err)
	}
	_, badgeCID, err := atproto.GetRecord(ctx, client, badgeAturi.Authority().String(), badgeAturi.Collection().String(), badgeAturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("failed to fetch badge: %w", err)
	}

	// Build subject union: either DID object or strongRef
	var subject map[string]any
	if strings.HasPrefix(subjectStr, "did:") {
		subject = map[string]any{
			"$type": "app.certified.defs#did",
			"did":   subjectStr,
		}
	} else {
		// Assume it's an AT-URI, build strongRef
		subjectAturi, err := syntax.ParseATURI(subjectStr)
		if err != nil {
			return fmt.Errorf("invalid subject URI: %w", err)
		}
		_, subjectCID, err := atproto.GetRecord(ctx, client, subjectAturi.Authority().String(), subjectAturi.Collection().String(), subjectAturi.RecordKey().String())
		if err != nil {
			return fmt.Errorf("failed to fetch subject: %w", err)
		}
		subject = buildStrongRef(subjectStr, subjectCID)
	}

	record := map[string]any{
		"$type":     atproto.CollectionBadgeAward,
		"badge":     buildStrongRef(badgeURI, badgeCID),
		"subject":   subject,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	if note != "" {
		record["note"] = note
	}
	if url != "" {
		record["url"] = url
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionBadgeAward, record)
	if err != nil {
		return fmt.Errorf("failed to create badge award: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Created badge award: %s\n", uri)
	return nil
}

func runBadgeAwardList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeAward)
	if err != nil {
		return fmt.Errorf("failed to list badge awards: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-35s %-30s %s\033[0m\n", "ID", "BADGE", "SUBJECT", "CREATED")
	fmt.Fprintf(w, "%-15s %-35s %-30s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 33),
		strings.Repeat("-", 28), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		badgeURI := ""
		if badge := mapMap(e.Value, "badge"); badge != nil {
			badgeURI = mapStr(badge, "uri")
		}
		if len(badgeURI) > 33 {
			badgeURI = "..." + badgeURI[len(badgeURI)-30:]
		}

		subjectStr := ""
		if subject := mapMap(e.Value, "subject"); subject != nil {
			if subjectType := mapStr(subject, "$type"); subjectType == "app.certified.defs#did" {
				subjectStr = mapStr(subject, "did")
			} else {
				subjectStr = mapStr(subject, "uri")
			}
		}
		if len(subjectStr) > 28 {
			subjectStr = "..." + subjectStr[len(subjectStr)-25:]
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-35s %-30s %s\n", id, badgeURI, subjectStr, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no badge awards found)\033[0m")
	}
	return nil
}

func runBadgeAwardGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionBadgeAward, "badge award")
}

func runBadgeAwardDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	id := cmd.String("id")
	if id == "" {
		id = cmd.Args().First()
	}

	if id == "" {
		awards, err := fetchBadgeAwards(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, awards, "badge award",
			func(a badgeAwardOption) string { return a.Rkey },
			func(a badgeAwardOption) string { return a.Subject },
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "badge award") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, a := range selected {
			aturi, _ := syntax.ParseATURI(a.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted badge award: %s\n", a.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionBadgeAward, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete badge award %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete badge award: %w", err)
	}
	fmt.Fprintf(w, "Deleted badge award: %s\n", extractRkey(uri))
	return nil
}

// --- Badge Response ---

type badgeResponseOption struct {
	URI        string
	CID        string
	Rkey       string
	BadgeAward string
	Response   string
	Weight     string
	Created    string
}

func fetchBadgeResponses(ctx context.Context, client *atclient.APIClient, did string) ([]badgeResponseOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to list badge responses: %w", err)
	}
	var result []badgeResponseOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		awardURI := ""
		if award := mapMap(e.Value, "badgeAward"); award != nil {
			awardURI = mapStr(award, "uri")
		}

		result = append(result, badgeResponseOption{
			URI:        e.URI,
			CID:        e.CID,
			Rkey:       string(aturi.RecordKey()),
			BadgeAward: awardURI,
			Response:   mapStr(e.Value, "response"),
			Weight:     mapStr(e.Value, "weight"),
			Created:    created,
		})
	}
	return result, nil
}

func runBadgeResponseCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	awardURI := cmd.String("badge-award")
	response := cmd.String("response")
	weight := cmd.String("weight")

	if awardURI == "" || response == "" {
		// Interactive mode
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Badge Award AT-URI").
					Description("AT-URI of badge award").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("badge award URI is required")
						}
						return nil
					}).
					Value(&awardURI),

				huh.NewSelect[string]().
					Title("Response").
					Description("Accept or reject the badge").
					Options(
						huh.NewOption("Accepted", "accepted"),
						huh.NewOption("Rejected", "rejected"),
					).
					Value(&response),

				huh.NewInput().
					Title("Weight").
					Description("Optional weight (max 50 chars)").
					CharLimit(50).
					Value(&weight),
			).Title("Badge Response"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}
	} else {
		// Non-interactive: prompt for missing required fields
		if awardURI == "" {
			err = huh.NewInput().Title("Badge Award AT-URI").Description("AT-URI of badge award").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("badge award URI is required")
					}
					return nil
				}).Value(&awardURI).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
		if response == "" {
			err = huh.NewSelect[string]().Title("Response").
				Options(
					huh.NewOption("Accepted", "accepted"),
					huh.NewOption("Rejected", "rejected"),
				).Value(&response).WithTheme(style.Theme()).Run()
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}
		}
	}

	// Validate response value
	if response != "accepted" && response != "rejected" {
		return fmt.Errorf("response must be 'accepted' or 'rejected', got: %s", response)
	}

	// Get award CID for strongRef
	awardAturi, err := syntax.ParseATURI(awardURI)
	if err != nil {
		return fmt.Errorf("invalid badge award URI: %w", err)
	}
	_, awardCID, err := atproto.GetRecord(ctx, client, awardAturi.Authority().String(), awardAturi.Collection().String(), awardAturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("failed to fetch badge award: %w", err)
	}

	record := map[string]any{
		"$type":      atproto.CollectionBadgeResponse,
		"badgeAward": buildStrongRef(awardURI, awardCID),
		"response":   response,
		"createdAt":  time.Now().UTC().Format(time.RFC3339),
	}

	if weight != "" {
		record["weight"] = weight
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionBadgeResponse, record)
	if err != nil {
		return fmt.Errorf("failed to create badge response: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Created badge response: %s\n", uri)
	return nil
}

func runBadgeResponseList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionBadgeResponse)
	if err != nil {
		return fmt.Errorf("failed to list badge responses: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-40s %-12s %s\033[0m\n", "ID", "BADGE AWARD", "RESPONSE", "CREATED")
	fmt.Fprintf(w, "%-15s %-40s %-12s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 38),
		strings.Repeat("-", 10), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		awardURI := ""
		if award := mapMap(e.Value, "badgeAward"); award != nil {
			awardURI = mapStr(award, "uri")
		}
		if len(awardURI) > 38 {
			awardURI = "..." + awardURI[len(awardURI)-35:]
		}

		response := mapStr(e.Value, "response")

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-40s %-12s %s\n", id, awardURI, response, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no badge responses found)\033[0m")
	}
	return nil
}

func runBadgeResponseGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionBadgeResponse, "badge response")
}

func runBadgeResponseDelete(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	id := cmd.String("id")
	if id == "" {
		id = cmd.Args().First()
	}

	if id == "" {
		responses, err := fetchBadgeResponses(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, responses, "badge response",
			func(r badgeResponseOption) string { return r.Rkey },
			func(r badgeResponseOption) string { return r.Response },
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "badge response") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, r := range selected {
			aturi, _ := syntax.ParseATURI(r.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted badge response: %s\n", r.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionBadgeResponse, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete badge response %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete badge response: %w", err)
	}
	fmt.Fprintf(w, "Deleted badge response: %s\n", extractRkey(uri))
	return nil
}
