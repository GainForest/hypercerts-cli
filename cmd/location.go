package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
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

type locationOption struct {
	URI         string
	CID         string
	Rkey        string
	Name        string
	Coordinates string // "lat, lon" parsed from record
	Description string
	Created     string
}

func fetchLocations(ctx context.Context, client *atclient.APIClient) ([]locationOption, error) {
	entries, err := atproto.ListAllRecords(ctx, client, client.AccountDID.String(), atproto.CollectionLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to list locations: %w", err)
	}
	var result []locationOption
	for _, e := range entries {
		rkey := extractRkey(e.URI)
		coords := ""
		if lat, lon, ok := parseLocationCoords(e.Value); ok {
			coords = fmt.Sprintf("%s, %s",
				strconv.FormatFloat(lat, 'f', -1, 64),
				strconv.FormatFloat(lon, 'f', -1, 64))
		}
		created := ""
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}
		result = append(result, locationOption{
			URI:         e.URI,
			CID:         e.CID,
			Rkey:        rkey,
			Name:        mapStr(e.Value, "name"),
			Coordinates: coords,
			Description: mapStr(e.Value, "description"),
			Created:     created,
		})
	}
	return result, nil
}

// selectLocation shows a menu to select an existing location or create a new one.
// Used by measurement, activity, attachment, evaluation for linking locations.
func selectLocation(ctx context.Context, client *atclient.APIClient, w io.Writer) (*locationOption, error) {
	locations, err := fetchLocations(ctx, client)
	if err != nil {
		return nil, err
	}

	selected, isCreate, err := menu.SingleSelectWithCreate(w, locations, "location",
		func(l locationOption) string {
			if l.Name != "" {
				return l.Name
			}
			return l.Coordinates
		},
		func(l locationOption) string {
			if l.Name != "" {
				return l.Coordinates
			}
			return ""
		},
		"Create new location...",
	)
	if err != nil {
		return nil, err
	}

	if isCreate {
		return createLocationInline(ctx, client, w)
	}
	return selected, nil
}

// selectLocations allows selecting multiple locations (for activity.locations[], etc.).
// Returns slice of selected locations.
func selectLocations(ctx context.Context, client *atclient.APIClient, w io.Writer) ([]locationOption, error) {
	var result []locationOption
	for {
		loc, err := selectLocation(ctx, client, w)
		if err != nil {
			return nil, err
		}
		result = append(result, *loc)

		if !menu.Confirm(w, os.Stdin, "Add another location?") {
			break
		}
	}
	return result, nil
}

func createLocationInline(ctx context.Context, client *atclient.APIClient, w io.Writer) (*locationOption, error) {
	var latStr, lonStr, name, description string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Latitude").Description("-90 to 90").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("latitude is required")
					}
					return nil
				}).Value(&latStr),
			huh.NewInput().Title("Longitude").Description("-180 to 180").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("longitude is required")
					}
					return nil
				}).Value(&lonStr),
			huh.NewInput().Title("Name").Description("optional, max 100 graphemes").CharLimit(100).Value(&name),
			huh.NewInput().Title("Description").Description("optional, max 500 graphemes").CharLimit(500).Value(&description),
		).Title("New Location"),
	).WithTheme(style.Theme())

	if err := form.Run(); err != nil {
		return nil, err
	}

	lat, ok := parseFloat(latStr)
	if !ok || lat < -90 || lat > 90 {
		return nil, fmt.Errorf("invalid latitude: must be between -90 and 90")
	}
	lon, ok := parseFloat(lonStr)
	if !ok || lon < -180 || lon > 180 {
		return nil, fmt.Errorf("invalid longitude: must be between -180 and 180")
	}

	record := buildLocationRecord(lat, lon, name, description)

	uri, cid, err := atproto.CreateRecord(ctx, client, atproto.CollectionLocation, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create location: %w", err)
	}

	coords := fmt.Sprintf("%s, %s",
		strconv.FormatFloat(lat, 'f', -1, 64),
		strconv.FormatFloat(lon, 'f', -1, 64))

	fmt.Fprintf(w, "\n")
	return &locationOption{
		URI: uri, CID: cid, Rkey: extractRkey(uri),
		Name: name, Coordinates: coords, Description: description,
		Created: time.Now().Format("2006-01-02"),
	}, nil
}

func runLocationCreate(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer

	latStr := cmd.String("lat")
	lonStr := cmd.String("lon")
	name := cmd.String("name")
	description := cmd.String("description")

	var lat, lon float64
	if latStr == "" || lonStr == "" {
		// Interactive mode
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Latitude").
					Description("-90 to 90").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("latitude is required")
						}
						return nil
					}).
					Value(&latStr),

				huh.NewInput().
					Title("Longitude").
					Description("-180 to 180").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return errors.New("longitude is required")
						}
						return nil
					}).
					Value(&lonStr),

				huh.NewInput().
					Title("Name").
					Description("optional, max 100 graphemes").
					CharLimit(100).
					Value(&name),

				huh.NewInput().
					Title("Description").
					Description("optional, max 500 graphemes").
					CharLimit(500).
					Value(&description),
			).Title("Location"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			return err
		}

		var ok bool
		lat, ok = parseFloat(latStr)
		if !ok || lat < -90 || lat > 90 {
			return fmt.Errorf("invalid latitude: must be between -90 and 90")
		}
		lon, ok = parseFloat(lonStr)
		if !ok || lon < -180 || lon > 180 {
			return fmt.Errorf("invalid longitude: must be between -180 and 180")
		}
	} else {
		// Non-interactive mode
		var ok bool
		lat, ok = parseFloat(latStr)
		if !ok || lat < -90 || lat > 90 {
			return fmt.Errorf("invalid latitude: must be between -90 and 90")
		}
		lon, ok = parseFloat(lonStr)
		if !ok || lon < -180 || lon > 180 {
			return fmt.Errorf("invalid longitude: must be between -180 and 180")
		}
	}

	record := buildLocationRecord(lat, lon, name, description)

	uri, _, err := atproto.CreateRecord(ctx, client, atproto.CollectionLocation, record)
	if err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Created location: %s\n", uri)
	return nil
}

func runLocationEdit(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	did := client.AccountDID.String()

	arg := cmd.Args().First()
	var uri string
	if arg == "" {
		locations, err := fetchLocations(ctx, client)
		if err != nil {
			return err
		}
		selected, err := menu.SingleSelect(w, locations, "location",
			func(l locationOption) string {
				if l.Name != "" {
					return l.Name
				}
				return l.Coordinates
			},
			func(l locationOption) string {
				if l.Name != "" {
					return l.Coordinates
				}
				return ""
			},
		)
		if err != nil {
			return err
		}
		uri = selected.URI
	} else {
		uri = resolveRecordURI(did, atproto.CollectionLocation, arg)
	}

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	existing, cid, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("location not found: %s", extractRkey(uri))
	}

	// Parse existing coords
	currentLat, currentLon, _ := parseLocationCoords(existing)
	currentLatStr := strconv.FormatFloat(currentLat, 'f', -1, 64)
	currentLonStr := strconv.FormatFloat(currentLon, 'f', -1, 64)
	currentName := mapStr(existing, "name")
	currentDesc := mapStr(existing, "description")

	// Get new values from flags or prompts
	newLatStr := cmd.String("lat")
	newLonStr := cmd.String("lon")
	newName := cmd.String("name")
	newDesc := cmd.String("description")

	if newLatStr == "" && newLonStr == "" && newName == "" && newDesc == "" {
		// Interactive mode
		newLatStr = currentLatStr
		newLonStr = currentLonStr
		newName = currentName
		newDesc = currentDesc

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Latitude").
					Description("-90 to 90").
					Value(&newLatStr),

				huh.NewInput().
					Title("Longitude").
					Description("-180 to 180").
					Value(&newLonStr),

				huh.NewInput().
					Title("Name").
					Description("Optional").
					Value(&newName),

				huh.NewInput().
					Title("Description").
					Description("Optional").
					Value(&newDesc),
			).Title("Edit Location"),
		).WithTheme(style.Theme())

		if err := form.Run(); err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			return err
		}
	}

	// Apply defaults
	if newLatStr == "" {
		newLatStr = currentLatStr
	}
	if newLonStr == "" {
		newLonStr = currentLonStr
	}
	if newName == "" {
		newName = currentName
	}
	if newDesc == "" {
		newDesc = currentDesc
	}

	// Validate coordinates
	newLat, ok := parseFloat(newLatStr)
	if !ok || newLat < -90 || newLat > 90 {
		return fmt.Errorf("invalid latitude: must be between -90 and 90")
	}
	newLon, ok := parseFloat(newLonStr)
	if !ok || newLon < -180 || newLon > 180 {
		return fmt.Errorf("invalid longitude: must be between -180 and 180")
	}

	// Check if anything changed
	if newLat == currentLat && newLon == currentLon && newName == currentName && newDesc == currentDesc {
		fmt.Fprintln(w, "No changes.")
		return nil
	}

	// Build updated record
	existing["location"] = map[string]any{
		"$type":  atproto.CollectionLocation + "#string",
		"string": strconv.FormatFloat(newLat, 'f', -1, 64) + ", " + strconv.FormatFloat(newLon, 'f', -1, 64),
	}
	existing["name"] = newName
	existing["description"] = newDesc

	resultURI, err := atproto.PutRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String(), existing, &cid)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}

	fmt.Fprintf(w, "\033[32m✓\033[0m Updated location: %s\n", resultURI)
	return nil
}

func runLocationDelete(ctx context.Context, cmd *cli.Command) error {
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
		locations, err := fetchLocations(ctx, client)
		if err != nil {
			return err
		}
		selected, err := menu.MultiSelect(w, locations, "location",
			func(l locationOption) string {
				if l.Name != "" {
					return l.Name
				}
				return l.Coordinates
			},
			func(l locationOption) string {
				if l.Name != "" {
					return l.Coordinates
				}
				return l.Rkey
			},
		)
		if err != nil {
			return err
		}
		if !menu.ConfirmBulkDelete(w, os.Stdin, len(selected), "location") {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
		for _, loc := range selected {
			aturi, _ := syntax.ParseATURI(loc.URI)
			if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
				fmt.Fprintf(w, "  Warning: %v\n", err)
			} else {
				fmt.Fprintf(w, "Deleted location: %s\n", loc.Rkey)
			}
		}
		return nil
	}

	uri := resolveRecordURI(did, atproto.CollectionLocation, id)
	if !cmd.Bool("force") {
		if !menu.Confirm(w, os.Stdin, fmt.Sprintf("Delete location %s?", extractRkey(uri))) {
			fmt.Fprintln(w, "Aborted.")
			return nil
		}
	}
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	if err := atproto.DeleteRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String()); err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	fmt.Fprintf(w, "Deleted location: %s\n", extractRkey(uri))
	return nil
}

func runLocationList(ctx context.Context, cmd *cli.Command) error {
	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	did := client.AccountDID.String()

	entries, err := atproto.ListAllRecords(ctx, client, did, atproto.CollectionLocation)
	if err != nil {
		return fmt.Errorf("failed to list locations: %w", err)
	}

	if cmd.Bool("json") {
		var records []map[string]any
		for _, e := range entries {
			records = append(records, map[string]any{"uri": e.URI, "record": e.Value})
		}
		fmt.Fprintln(w, prettyJSON(records))
		return nil
	}

	fmt.Fprintf(w, "\033[1m%-15s %-25s %-25s %-30s %s\033[0m\n", "ID", "NAME", "COORDINATES", "DESCRIPTION", "CREATED")
	fmt.Fprintf(w, "%-15s %-25s %-25s %-30s %s\n",
		strings.Repeat("-", 13), strings.Repeat("-", 23),
		strings.Repeat("-", 23), strings.Repeat("-", 28), strings.Repeat("-", 10))

	for _, e := range entries {
		aturi, err := syntax.ParseATURI(e.URI)
		if err != nil {
			continue
		}
		id := string(aturi.RecordKey())
		name := mapStr(e.Value, "name")
		coords := ""
		if lat, lon, ok := parseLocationCoords(e.Value); ok {
			coords = fmt.Sprintf("%s, %s",
				strconv.FormatFloat(lat, 'f', -1, 64),
				strconv.FormatFloat(lon, 'f', -1, 64))
		}
		description := mapStr(e.Value, "description")

		if len(name) > 23 {
			name = name[:20] + "..."
		}
		if len(coords) > 23 {
			coords = coords[:20] + "..."
		}
		if len(description) > 28 {
			description = description[:25] + "..."
		}

		created := "-"
		if createdAt := mapStr(e.Value, "createdAt"); createdAt != "" {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				created = t.Format("2006-01-02")
			}
		}

		fmt.Fprintf(w, "%-15s %-25s %-25s %-30s %s\n", id, name, coords, description, created)
	}

	if len(entries) == 0 {
		fmt.Fprintln(w, "\033[90m(no locations found)\033[0m")
	}
	return nil
}

func runLocationGet(ctx context.Context, cmd *cli.Command) error {
	return runSimpleGet(ctx, cmd, atproto.CollectionLocation, "location")
}
