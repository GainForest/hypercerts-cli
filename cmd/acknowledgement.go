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

type acknowledgementOption struct {
	URI          string
	Rkey         string
	Subject      string
	Acknowledged bool
	Context      string
	Created      string
}

func fetchAcknowledgements(ctx context.Context, client *atclient.APIClient, did string) ([]acknowledgementOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionAcknowledgement)
	if err != nil {
		return nil, fmt.Errorf("failed to list acknowledgements: %w", err)
	}
	var result []acknowledgementOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		subject := ""
		if subjectRef := mapMap(e.Value, "subject"); subjectRef != nil {
			subject = mapStr(subjectRef, "uri")
		}
		context := ""
		if contextRef := mapMap(e.Value, "context"); contextRef != nil {
			if uri := mapStr(contextRef, "uri"); uri != "" {
				context = uri
			}
		}
		acknowledged := false
		if ack, ok := e.Value["acknowledged"].(bool); ok {
			acknowledged = ack
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, acknowledgementOption{
			URI:          e.URI,
			Rkey:         string(aturi.RecordKey()),
			Subject:      subject,
			Acknowledged: acknowledged,
			Context:      context,
			Created:      created,
		})
	}
	return result, nil
}

func runAcknowledgementCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionAcknowledgement,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	subjectFlag := cmd.String("subject")
	contextFlag := cmd.String("context")
	acknowledgedFlag := cmd.Bool("acknowledged")
	rejectedFlag := cmd.Bool("rejected")
	commentFlag := cmd.String("comment")

	hasFlags := subjectFlag != "" || contextFlag != "" || rejectedFlag || commentFlag != ""

	if hasFlags {
		// Non-interactive mode
		if subjectFlag == "" {
			return fmt.Errorf("--subject is required")
		}

		// Parse subject URI and get CID
		subjectURI, err := syntax.ParseATURI(subjectFlag)
		if err != nil {
			return fmt.Errorf("invalid subject URI: %w", err)
		}
		_, subjectCID, err := atproto.GetRecord(ctx, client, subjectURI.Authority().String(), subjectURI.Collection().String(), subjectURI.RecordKey().String())
		if err != nil {
			return fmt.Errorf("subject record not found: %w", err)
		}
		record["subject"] = buildStrongRef(subjectFlag, subjectCID)

		// Set acknowledged (default true unless --rejected)
		if rejectedFlag {
			record["acknowledged"] = false
		} else {
			record["acknowledged"] = acknowledgedFlag || !rejectedFlag
		}

		// Optional context
		if contextFlag != "" {
			contextURI, err := syntax.ParseATURI(contextFlag)
			if err != nil {
				return fmt.Errorf("invalid context URI: %w", err)
			}
			_, contextCID, err := atproto.GetRecord(ctx, client, contextURI.Authority().String(), contextURI.Collection().String(), contextURI.RecordKey().String())
			if err != nil {
				return fmt.Errorf("context record not found: %w", err)
			}
			record["context"] = buildStrongRef(contextFlag, contextCID)
		}

		// Optional comment
		if commentFlag != "" {
			record["comment"] = commentFlag
		}
	} else {
		// Interactive mode
		var subjectURI string
		var acknowledged bool = true
		var addContext bool
		var comment string

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Subject AT-URI").
					Description("The record being acknowledged (required)").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("subject URI is required")
						}
						_, err := syntax.ParseATURI(s)
						if err != nil {
							return fmt.Errorf("invalid AT-URI: %w", err)
						}
						return nil
					}).
					Value(&subjectURI),

				huh.NewConfirm().
					Title("Acknowledged?").
					Description("True to acknowledge, false to reject").
					Value(&acknowledged),
			).Title("Acknowledgement Details"),

			huh.NewGroup(
				huh.NewConfirm().
					Title("Add context?").
					Description("Link to a context record (optional)").
					Value(&addContext),

				huh.NewInput().
					Title("Comment").
					Description("Optional comment (max 10000 chars)").
					CharLimit(10000).
					Value(&comment),
			).Title("Optional Fields"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return fmt.Errorf("cancelled")
			}
			return err
		}

		// Get subject CID
		parsedSubject, err := syntax.ParseATURI(subjectURI)
		if err != nil {
			return fmt.Errorf("invalid subject URI: %w", err)
		}
		_, subjectCID, err := atproto.GetRecord(ctx, client, parsedSubject.Authority().String(), parsedSubject.Collection().String(), parsedSubject.RecordKey().String())
		if err != nil {
			return fmt.Errorf("subject record not found: %w", err)
		}
		record["subject"] = buildStrongRef(subjectURI, subjectCID)
		record["acknowledged"] = acknowledged

		// Optional context
		if addContext {
			var contextURI string
			contextForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Context AT-URI").
						Description("The context record").
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return errors.New("context URI is required")
							}
							_, err := syntax.ParseATURI(s)
							if err != nil {
								return fmt.Errorf("invalid AT-URI: %w", err)
							}
							return nil
						}).
						Value(&contextURI),
				),
			).WithTheme(style.Theme())

			if err := contextForm.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					return fmt.Errorf("cancelled")
				}
				return err
			}

			parsedContext, err := syntax.ParseATURI(contextURI)
			if err != nil {
				return fmt.Errorf("invalid context URI: %w", err)
			}
			_, contextCID, err := atproto.GetRecord(ctx, client, parsedContext.Authority().String(), parsedContext.Collection().String(), parsedContext.RecordKey().String())
			if err != nil {
				return fmt.Errorf("context record not found: %w", err)
			}
			record["context"] = buildStrongRef(contextURI, contextCID)
		}

		// Optional comment
		if comment != "" {
			record["comment"] = comment
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionAcknowledgement, record)
	if err != nil {
		return fmt.Errorf("failed to create acknowledgement: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created acknowledgement: %s\n", uri)
	return nil
}

func runAcknowledgementDelete(ctx context.Context, cmd *cli.Command) error {
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
		acknowledgements, err := fetchAcknowledgements(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, acknowledgements, "acknowledgement",
			func(a acknowledgementOption) string {
				status := "✓ acknowledged"
				if !a.Acknowledged {
					status = "✗ rejected"
				}
				return fmt.Sprintf("%s - %s", extractRkey(a.Subject), status)
			},
			func(a acknowledgementOption) string {
				return a.Rkey
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "acknowledgement") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, a := range selected {
			aturi, _ := syntax.ParseATURI(a.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted acknowledgement: %s\n", a.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionAcknowledgement, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete acknowledgement %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete acknowledgement: %w", err)
	}
	fmt.Fprintf(w, "Deleted acknowledgement: %s\n", extractRkey(uri))
	return nil
}

func runAcknowledgementList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := fetchAcknowledgements(ctx, client, did)
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		// Re-fetch full records for JSON output
		rawEntries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionAcknowledgement)
		if err != nil {
			return fmt.Errorf("failed to list acknowledgements: %w", err)
		}
		var records []map[string]any
		for _, e := range rawEntries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-40s %-15s %-40s %s\033[0m\n", "ID", "SUBJECT", "ACKNOWLEDGED", "CONTEXT", "CREATED")
	fmt.Fprintf(w, "%-15s %-40s %-15s %-40s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 38),
		strings.Repeat("-", 13), strings.Repeat("-", 38), strings.Repeat("-", 10))

	for _, a := range entries {
		subject := extractRkey(a.Subject)
		if subject == "" {
			subject = "-"
		}
		if len(subject) > 38 {
			subject = subject[:35] + "..."
		}

		ackStatus := "✓ yes"
		if !a.Acknowledged {
			ackStatus = "✗ no"
		}

		context := extractRkey(a.Context)
		if context == "" {
			context = "-"
		}
		if len(context) > 38 {
			context = context[:35] + "..."
		}

		fmt.Fprintf(w, "%-15s %-40s %-15s %-40s %s\n",
			a.Rkey, subject, ackStatus, context, a.Created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no acknowledgements found)\033[0m")
	}
	return nil
}

func runAcknowledgementGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionAcknowledgement, "acknowledgement")
}
