package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
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

// keyPattern validates lowercase-hyphenated keys
var keyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type workScopeOption struct {
	URI        string
	CID        string
	Rkey       string
	Key        string
	Label      string
	Kind       string
	ParentRkey string
	Created    string
}

func fetchWorkScopes(ctx context.Context, client *atclient.APIClient, did string) ([]workScopeOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionWorkScopeTag)
	if err != nil {
		return nil, fmt.Errorf("failed to list work scope tags: %w", err)
	}
	var result []workScopeOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}

		parentRkey := ""
		if parent := mapMap(e.Value, "parent"); parent != nil {
			parentRkey = extractRkey(mapStr(parent, "uri"))
		}

		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		key := mapStr(e.Value, "key")
		if len(key) > 25 {
			key = key[:22] + "..."
		}

		label := mapStr(e.Value, "label")
		if len(label) > 30 {
			label = label[:27] + "..."
		}

		result = append(result, workScopeOption{
			URI:        e.URI,
			CID:        e.CID,
			Rkey:       string(aturi.RecordKey()),
			Key:        key,
			Label:      label,
			Kind:       mapStr(e.Value, "kind"),
			ParentRkey: parentRkey,
			Created:    created,
		})
	}
	return result, nil
}

// selectWorkScope allows selecting an existing work scope tag.
func selectWorkScope(ctx context.Context, client *atclient.APIClient, w io.Writer) (*workScopeOption, error) {
	did := client.AccountDID.String()
	scopes, err := fetchWorkScopes(ctx, client, did)
	if err != nil {
		return nil, err
	}

	if len(scopes) == 0 {
		return nil, fmt.Errorf("no work scope tags found - create one first")
	}

	selected, err := menu.SingleSelect(w, scopes, "work scope",
		func(s workScopeOption) string { return s.Label },
		func(s workScopeOption) string {
			if s.Kind != "" {
				return s.Kind
			}
			return s.Key
		},
	)
	if err != nil {
		return nil, err
	}

	return selected, nil
}

func runWorkScopeCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionWorkScopeTag,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	key := cmd.String("key")
	label := cmd.String("label")
	kind := cmd.String("kind")
	description := cmd.String("description")

	if key != "" && label != "" {
		// Non-interactive flag mode
		if !keyPattern.MatchString(key) {
			return fmt.Errorf("key must be lowercase letters and numbers separated by hyphens")
		}
		record["key"] = key
		record["label"] = label
		if kind != "" {
			record["kind"] = kind
		}
		if description != "" {
			record["description"] = description
		}
	} else {
		// Interactive: show all fields at once using huh form
		var addParent, addAliases bool

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Key").
					Description("Lowercase-hyphenated, e.g. climate-action").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("key is required")
						}
						return nil
					}).
					Value(&key),

				huh.NewInput().
					Title("Label").
					Description("Human-readable name").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("label is required")
						}
						return nil
					}).
					Value(&label),

				huh.NewSelect[string]().
					Title("Kind").
					Description("Category for this tag (optional)").
					Options(
						huh.NewOption("(skip)", ""),
						huh.NewOption("topic - subject area", "topic"),
						huh.NewOption("language - programming/natural", "language"),
						huh.NewOption("domain - field of study", "domain"),
						huh.NewOption("method - approach/technique", "method"),
						huh.NewOption("tag - general label", "tag"),
					).
					Value(&kind),

				huh.NewInput().
					Title("Description").
					Description("Max 1000 graphemes (optional)").
					CharLimit(1000).
					Value(&description),
			).Title("Work Scope Tag"),

			huh.NewGroup(
				huh.NewConfirm().
					Title("Add parent tag?").
					Description("Link to a broader scope tag").
					Value(&addParent),

				huh.NewConfirm().
					Title("Add aliases?").
					Description("Alternative names for this tag").
					Value(&addAliases),
			).Title("Hierarchy"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			return err
		}

		if !keyPattern.MatchString(key) {
			return fmt.Errorf("key must be lowercase letters and numbers separated by hyphens")
		}
		record["key"] = key
		record["label"] = label
		if kind != "" {
			record["kind"] = kind
		}
		if description != "" {
			record["description"] = description
		}

		// Handle parent selection (needs API call + menu)
		if addParent {
			parent, err := selectWorkScope(ctx, client, w)
			if err != nil && err != menu.ErrCancelled {
				return err
			}
			if parent != nil {
				record["parent"] = buildStrongRef(parent.URI, parent.CID)
			}
		}

		// Handle aliases (loop until blank)
		if addAliases {
			var aliases []string
			for {
				var alias string
				var addMore bool

				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().Title("Alias").Description("alternative name, or leave blank to finish").Value(&alias),
						huh.NewConfirm().Title("Add another alias?").Inline(true).Value(&addMore),
					),
				).WithTheme(style.Theme())

				if err := form.Run(); err != nil {
					if err == huh.ErrUserAborted {
						break
					}
					return err
				}

				if strings.TrimSpace(alias) == "" {
					break
				}
				aliases = append(aliases, alias)

				if !addMore {
					break
				}
			}
			if len(aliases) > 0 {
				record["aliases"] = aliases
			}
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionWorkScopeTag, record)
	if err != nil {
		return fmt.Errorf("failed to create work scope tag: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created work scope tag: %s\n", uri)
	return nil
}

func runWorkScopeEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		scopes, err := fetchWorkScopes(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, scopes, "work scope",
			func(s workScopeOption) string { return s.Label },
			func(s workScopeOption) string {
				if s.Kind != "" {
					return s.Kind
				}
				return s.Key
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionWorkScopeTag, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("work scope tag not found: %s", extractRkey(uri))
	}

	// Get current values
	currentKey := mapStr(existing, "key")
	currentLabel := mapStr(existing, "label")
	currentKind := mapStr(existing, "kind")
	currentDesc := mapStr(existing, "description")

	changed := false
	isInteractive := cmd.String("key") == "" && cmd.String("label") == "" && cmd.String("kind") == ""

	if isInteractive {
		newKey := currentKey
		newLabel := currentLabel
		newKind := currentKind
		newDesc := currentDesc
		var editParent bool

		existingParent := mapMap(existing, "parent")
		parentTitle := "Link parent tag?"
		if existingParent != nil {
			parentTitle = "Replace parent tag?"
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Key").
					Description("Required, lowercase-hyphenated").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("key is required")
						}
						if !keyPattern.MatchString(s) {
							return errors.New("must be lowercase letters/numbers separated by hyphens")
						}
						return nil
					}).
					Value(&newKey),

				huh.NewInput().
					Title("Label").
					Description("Required").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("label is required")
						}
						return nil
					}).
					Value(&newLabel),

				huh.NewSelect[string]().
					Title("Kind").
					Options(
						huh.NewOption("(keep current)", currentKind),
						huh.NewOption("topic - subject area", "topic"),
						huh.NewOption("language - programming/natural", "language"),
						huh.NewOption("domain - field of study", "domain"),
						huh.NewOption("method - approach/technique", "method"),
						huh.NewOption("tag - general label", "tag"),
					).
					Value(&newKind),

				huh.NewInput().
					Title("Description").
					Description("Optional").
					Value(&newDesc),
			).Title("Edit Work Scope Tag"),

			huh.NewGroup(
				huh.NewConfirm().
					Title(parentTitle).
					Description("Link to a broader scope tag").
					Value(&editParent),
			).Title("Linked Records"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			return err
		}

		if newKey != currentKey {
			existing["key"] = newKey
			changed = true
		}
		if newLabel != currentLabel {
			existing["label"] = newLabel
			changed = true
		}
		if newKind != currentKind {
			if newKind == "" {
				delete(existing, "kind")
			} else {
				existing["kind"] = newKind
			}
			changed = true
		}
		if newDesc != currentDesc {
			if newDesc == "" {
				delete(existing, "description")
			} else {
				existing["description"] = newDesc
			}
			changed = true
		}

		if editParent {
			parent, err := selectWorkScope(ctx, client, w)
			if err != nil && err != menu.ErrCancelled {
				return err
			}
			if parent != nil {
				existing["parent"] = buildStrongRef(parent.URI, parent.CID)
				changed = true
			}
		}
	} else {
		// Non-interactive mode
		newKey := cmd.String("key")
		if newKey != "" && newKey != currentKey {
			if !keyPattern.MatchString(newKey) {
				return fmt.Errorf("key must be lowercase letters and numbers separated by hyphens")
			}
			existing["key"] = newKey
			changed = true
		}

		newLabel := cmd.String("label")
		if newLabel != "" && newLabel != currentLabel {
			existing["label"] = newLabel
			changed = true
		}

		newKind := cmd.String("kind")
		if newKind != "" && newKind != currentKind {
			existing["kind"] = newKind
			changed = true
		}

		newDesc := cmd.String("description")
		if newDesc != "" && newDesc != currentDesc {
			existing["description"] = newDesc
			changed = true
		}
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update work scope tag: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated work scope tag: %s\n", resultURI)
	return nil
}

func runWorkScopeDelete(ctx context.Context, cmd *cli.Command) error {
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
		scopes, err := fetchWorkScopes(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, scopes, "work scope",
			func(s workScopeOption) string { return s.Label },
			func(s workScopeOption) string {
				if s.Kind != "" {
					return s.Kind
				}
				return s.Key
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "work scope tag") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, s := range selected {
			aturi, _ := syntax.ParseATURI(s.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted work scope tag: %s\n", s.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionWorkScopeTag, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete work scope tag %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete work scope tag: %w", err)
	}
	fmt.Fprintf(w, "Deleted work scope tag: %s\n", extractRkey(uri))
	return nil
}

func runWorkScopeList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionWorkScopeTag)
	if err != nil {
		return fmt.Errorf("failed to list work scope tags: %w", err)
	}

	// Filter by kind if specified
	kindFilter := cmd.String("kind")
	if kindFilter != "" {
		var filtered []atproto.RecordEntry
		for _, e := range entries {
			if k := mapStr(e.Value, "kind"); k == kindFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-25s %-30s %-12s %-10s %s\033[0m\n", "ID", "KEY", "LABEL", "KIND", "PARENT", "CREATED")
	fmt.Fprintf(w, "%-15s %-25s %-30s %-12s %-10s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 23),
		strings.Repeat("-", 28), strings.Repeat("-", 10),
		strings.Repeat("-", 8), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		key := mapStr(e.Value, "key")
		if len(key) > 23 {
			key = key[:20] + "..."
		}

		label := mapStr(e.Value, "label")
		if len(label) > 28 {
			label = label[:25] + "..."
		}

		kind := mapStr(e.Value, "kind")
		if kind == "" {
			kind = "-"
		}

		parentRkey := "-"
		if parent := mapMap(e.Value, "parent"); parent != nil {
			parentRkey = extractRkey(mapStr(parent, "uri"))
			if len(parentRkey) > 8 {
				parentRkey = parentRkey[:5] + "..."
			}
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-25s %-30s %-12s %-10s %s\n", id, key, label, kind, parentRkey, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no work scope tags found)\033[0m")
	}
	return nil
}

func runWorkScopeGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionWorkScopeTag, "workscope")
}
