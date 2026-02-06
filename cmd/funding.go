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
	"github.com/GainForest/hypercerts-cli/internal/prompt"
)

type fundingOption struct {
	URI        string
	CID        string
	Rkey       string
	From       string
	To         string
	Amount     string
	Currency   string
	ForRkey    string
	OccurredAt string
	Created    string
}

func fetchFundings(ctx context.Context, client *atclient.APIClient, did string) ([]fundingOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionFundingReceipt)
	if err != nil {
		return nil, fmt.Errorf("failed to list funding receipts: %w", err)
	}
	var result []fundingOption
	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}

		forRkey := ""
		if forURI := mapStr(e.Value, "for"); forURI != "" {
			forRkey = extractRkey(forURI)
		}

		occurred := ""
		if occurredAt := mapStr(e.Value, "occurredAt"); occurredAt != "" {
			if t, err := time.Parse(time.RFC3339, occurredAt); err == nil {
				occurred = t.Format("2006-01-02")
			}
		}

		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		from := mapStr(e.Value, "from")
		if len(from) > 20 {
			from = from[:17] + "..."
		}

		to := mapStr(e.Value, "to")
		if len(to) > 20 {
			to = to[:17] + "..."
		}

		result = append(result, fundingOption{
			URI:        e.URI,
			CID:        e.CID,
			Rkey:       string(aturi.RecordKey()),
			From:       from,
			To:         to,
			Amount:     mapStr(e.Value, "amount"),
			Currency:   mapStr(e.Value, "currency"),
			ForRkey:    forRkey,
			OccurredAt: occurred,
			Created:    created,
		})
	}
	return result, nil
}

func runFundingCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	record := map[string]any{
		"$type":     atproto.CollectionFundingReceipt,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	from := cmd.String("from")
	to := cmd.String("to")
	amount := cmd.String("amount")
	currency := cmd.String("currency")
	forURI := cmd.String("for")

	hasFlags := from != "" || to != "" || amount != "" || currency != ""

	if hasFlags {
		// Non-interactive: require fields via flags or prompt fallback
		if from == "" {
			defaultFrom := client.AccountDID.String()
			from, err = prompt.ReadRequiredWithDefault(w, os.Stdin, "From (sender DID)", "", defaultFrom)
			if err != nil {
				return err
			}
		}
		if to == "" {
			to, err = prompt.ReadRequired(w, os.Stdin, "To (recipient)", "DID or name")
			if err != nil {
				return err
			}
		}
		if amount == "" {
			amount, err = prompt.ReadRequired(w, os.Stdin, "Amount", "e.g. 1000.00")
			if err != nil {
				return err
			}
		}
		if currency == "" {
			currency, err = prompt.ReadRequired(w, os.Stdin, "Currency", "e.g. USD, EUR, ETH")
			if err != nil {
				return err
			}
		}

		record["from"] = from
		record["to"] = to
		record["amount"] = amount
		record["currency"] = currency

		// Handle --for flag
		if forURI != "" {
			did := client.AccountDID.String()
			record["for"] = resolveRecordURI(did, atproto.CollectionActivity, forURI)
		}
	} else {
		// Interactive: show all fields at once using huh form
		var paymentRail, paymentNetwork, transactionID, notes, occurredAt string
		var linkActivity bool

		// Pre-populate from with default
		from = client.AccountDID.String()

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("From").
					Description("Sender DID").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("from is required")
						}
						return nil
					}).
					Value(&from),

				huh.NewInput().
					Title("To").
					Description("Recipient DID or name").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("to is required")
						}
						return nil
					}).
					Value(&to),

				huh.NewInput().
					Title("Amount").
					Description("e.g. 1000.00").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("amount is required")
						}
						return nil
					}).
					Value(&amount),

				huh.NewInput().
					Title("Currency").
					Description("e.g. USD, EUR, ETH").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("currency is required")
						}
						return nil
					}).
					Value(&currency),
			).Title("Funding Details"),

			huh.NewGroup(
				huh.NewInput().
					Title("Payment rail").
					Description("bank_transfer, credit_card, onchain, cash (optional)").
					Value(&paymentRail),

				huh.NewInput().
					Title("Payment network").
					Description("arbitrum, ethereum, sepa, visa, paypal (optional)").
					Value(&paymentNetwork),

				huh.NewInput().
					Title("Transaction ID").
					Description("Payment reference (optional)").
					Value(&transactionID),

				huh.NewInput().
					Title("Notes").
					Description("Max 500 chars (optional)").
					CharLimit(500).
					Value(&notes),

				huh.NewInput().
					Title("Occurred at").
					Description("YYYY-MM-DD or RFC3339 (optional)").
					Value(&occurredAt),
			).Title("Payment Details"),

			huh.NewGroup(
				huh.NewConfirm().
					Title("Link to an activity?").
					Description("Select an existing activity to link this funding to").
					Value(&linkActivity),
			).Title("Linked Records"),
		).WithTheme(huh.ThemeBase16())

		err := form.Run()
		if err != nil {
			return err
		}

		record["from"] = from
		record["to"] = to
		record["amount"] = amount
		record["currency"] = currency

		if paymentRail != "" {
			record["paymentRail"] = paymentRail
		}
		if paymentNetwork != "" {
			record["paymentNetwork"] = paymentNetwork
		}
		if transactionID != "" {
			record["transactionId"] = transactionID
		}
		if notes != "" {
			record["notes"] = notes
		}
		if occurredAt != "" {
			record["occurredAt"] = normalizeDate(occurredAt)
		}

		if linkActivity {
			uri, _, err := selectActivity(ctx, client, w)
			if err != nil && err != menu.ErrCancelled {
				return err
			}
			if uri != "" {
				record["for"] = uri
			}
		}
	}

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionFundingReceipt, record)
	if err != nil {
		return fmt.Errorf("failed to create funding receipt: %w", err)
	}

	fmt.Fprintf(w, "\n\033[32m✓\033[0m Created funding receipt: %s\n", uri)
	return nil
}

func runFundingEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		fundings, err := fetchFundings(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, fundings, "funding receipt",
			func(f fundingOption) string {
				return fmt.Sprintf("%s %s to %s", f.Amount, f.Currency, f.To)
			},
			func(f fundingOption) string {
				if f.OccurredAt != "" {
					return f.OccurredAt
				}
				return f.Rkey
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionFundingReceipt, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("funding receipt not found: %s", extractRkey(uri))
	}

	// Get current values
	currentTo := mapStr(existing, "to")
	currentAmount := mapStr(existing, "amount")
	currentCurrency := mapStr(existing, "currency")
	currentNotes := mapStr(existing, "notes")

	changed := false

	// To
	newTo, err := prompt.ReadLineWithDefault(w, os.Stdin, "To (recipient)", "required", currentTo)
	if err != nil {
		return err
	}
	if newTo != currentTo {
		existing["to"] = newTo
		changed = true
	}

	// Amount
	newAmount, err := prompt.ReadLineWithDefault(w, os.Stdin, "Amount", "required", currentAmount)
	if err != nil {
		return err
	}
	if newAmount != currentAmount {
		existing["amount"] = newAmount
		changed = true
	}

	// Currency
	newCurrency, err := prompt.ReadLineWithDefault(w, os.Stdin, "Currency", "required", currentCurrency)
	if err != nil {
		return err
	}
	if newCurrency != currentCurrency {
		existing["currency"] = newCurrency
		changed = true
	}

	// Notes
	newNotes, err := prompt.ReadLineWithDefault(w, os.Stdin, "Notes", "optional", currentNotes)
	if err != nil {
		return err
	}
	if newNotes != currentNotes {
		if newNotes == "" {
			delete(existing, "notes")
		} else {
			existing["notes"] = newNotes
		}
		changed = true
	}

	if !changed {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update funding receipt: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated funding receipt: %s\n", resultURI)
	return nil
}

func runFundingDelete(ctx context.Context, cmd *cli.Command) error {
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
		fundings, err := fetchFundings(ctx, client, did)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, fundings, "funding receipt",
			func(f fundingOption) string {
				return fmt.Sprintf("%s %s to %s", f.Amount, f.Currency, f.To)
			},
			func(f fundingOption) string {
				if f.OccurredAt != "" {
					return f.OccurredAt
				}
				return f.Rkey
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "funding receipt") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, f := range selected {
			aturi, _ := syntax.ParseATURI(f.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted funding receipt: %s\n", f.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionFundingReceipt, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete funding receipt %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete funding receipt: %w", err)
	}
	fmt.Fprintf(w, "Deleted funding receipt: %s\n", extractRkey(uri))
	return nil
}

func runFundingList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionFundingReceipt)
	if err != nil {
		return fmt.Errorf("failed to list funding receipts: %w", err)
	}

	// Filter by activity if specified
	activityFilter := cmd.String("activity")
	if activityFilter != "" {
		activityURI := resolveRecordURI(did, atproto.CollectionActivity, activityFilter)
		var filtered []atproto.RecordEntry
		for _, e := range entries {
			if forURI := mapStr(e.Value, "for"); forURI == activityURI {
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

	fmt.Fprintf(w, "\033[1m%-15s %-12s %-8s %-20s %-10s %-10s %s\033[0m\n", "ID", "AMOUNT", "CURRENCY", "TO", "FOR", "OCCURRED", "CREATED")
	fmt.Fprintf(w, "%-15s %-12s %-8s %-20s %-10s %-10s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 10),
		strings.Repeat("-", 6), strings.Repeat("-", 18),
		strings.Repeat("-", 8), strings.Repeat("-", 10), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())

		amount := mapStr(e.Value, "amount")
		if len(amount) > 10 {
			amount = amount[:7] + "..."
		}

		currency := mapStr(e.Value, "currency")
		if len(currency) > 6 {
			currency = currency[:3] + "..."
		}

		to := mapStr(e.Value, "to")
		if len(to) > 18 {
			to = to[:15] + "..."
		}

		forRkey := "-"
		if forURI := mapStr(e.Value, "for"); forURI != "" {
			forRkey = extractRkey(forURI)
			if len(forRkey) > 8 {
				forRkey = forRkey[:5] + "..."
			}
		}

		occurred := "-"
		if occurredAt := mapStr(e.Value, "occurredAt"); occurredAt != "" {
			if t, err := time.Parse(time.RFC3339, occurredAt); err == nil {
				occurred = t.Format("2006-01-02")
			}
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-12s %-8s %-20s %-10s %-10s %s\n", id, amount, currency, to, forRkey, occurred, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no funding receipts found)\033[0m")
	}
	return nil
}

func runFundingGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionFundingReceipt, "funding")
}
