package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atclient"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/urfave/cli/v3"

	"github.com/GainForest/hypercerts-cli/internal/atproto"
)

// optionalField represents an optional field that can be added to a record.
type optionalField struct {
	Name  string
	Label string
	Hint  string
}

// requireAuth authenticates and returns an API client, or a user-friendly error.
func requireAuth(ctx context.Context, cmd *cli.Command) (*atclient.APIClient, error) {
	client, err := atproto.LoginOrLoad(
		ctx,
		cmd.Root().String("username"),
		cmd.Root().String("password"),
		cmd.Root().String("plc-host"),
		Version,
	)
	if err == atproto.ErrNoAuthSession {
		return nil, fmt.Errorf("not logged in (run: hc account login)")
	}
	if err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}
	return client, nil
}

// configDirectory returns an identity directory for unauthenticated reads.
func configDirectory(cmd *cli.Command) identity.Directory {
	return atproto.ConfigDirectory(cmd.Root().String("plc-host"), Version)
}

// resolveIdent parses an AT identifier and looks it up via the directory.
func resolveIdent(ctx context.Context, cmd *cli.Command, arg string) (*identity.Identity, error) {
	id, err := syntax.ParseAtIdentifier(arg)
	if err != nil {
		return nil, err
	}
	dir := configDirectory(cmd)
	return dir.Lookup(ctx, id)
}

// userAgentString returns the user agent string for HTTP requests.
func userAgentString() string {
	return fmt.Sprintf("hc/%s", Version)
}

// extractRkey extracts the record key (last segment) from an AT-URI.
func extractRkey(uri string) string {
	return atproto.ExtractRkey(uri)
}

// resolveRecordURI resolves a short ID or full AT-URI to a full record AT-URI.
func resolveRecordURI(did, collection, idOrURI string) string {
	if strings.HasPrefix(idOrURI, "at://") {
		return idOrURI
	}
	return fmt.Sprintf("at://%s/%s/%s", did, collection, idOrURI)
}

// mapStr safely extracts a string from a map.
func mapStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// prettyJSON marshals a value with indentation.
func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// normalizeDate converts YYYY-MM-DD to RFC3339 or returns the input as-is.
func normalizeDate(s string) string {
	if s == "" {
		return ""
	}
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s + "T00:00:00Z"
	}
	return s
}

// buildStrongRef builds a strongRef object from URI and CID.
func buildStrongRef(uri, cid string) map[string]any {
	return map[string]any{
		"uri": uri,
		"cid": cid,
	}
}

// mapMap safely extracts a map[string]any from a map.
func mapMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

// mapSlice safely extracts a []any from a map.
func mapSlice(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

// parseFloat parses a string to float64.
func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// buildLocationRecord builds an app.certified.location record.
// Uses LP v1.0, OGC CRS84, coordinate-decimal format.
// Coordinates stored as "lat, lon" string matching audiogoat pattern.
func buildLocationRecord(lat, lon float64, name, description string) map[string]any {
	return map[string]any{
		"$type":        atproto.CollectionLocation,
		"lpVersion":    "1.0",
		"srs":          "http://www.opengis.net/def/crs/OGC/1.3/CRS84",
		"locationType": "coordinate-decimal",
		"location": map[string]any{
			"$type":  atproto.CollectionLocation + "#string",
			"string": strconv.FormatFloat(lat, 'f', -1, 64) + ", " + strconv.FormatFloat(lon, 'f', -1, 64),
		},
		"name":        name,
		"description": description,
		"createdAt":   time.Now().UTC().Format(time.RFC3339),
	}
}

// runSimpleGet fetches a single record by ID/AT-URI and prints it as JSON.
// Used by all subcommand `get` actions.
func runSimpleGet(ctx context.Context, cmd *cli.Command, collection, typeName string) error {
	arg := cmd.Args().First()
	if arg == "" {
		return fmt.Errorf("usage: hc %s get <id|at-uri>", typeName)
	}

	client, err := requireAuth(ctx, cmd)
	if err != nil {
		return err
	}

	did := client.AccountDID.String()
	uri := resolveRecordURI(did, collection, arg)

	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	record, _, err := atproto.GetRecord(ctx, client, did, aturi.Collection().String(), aturi.RecordKey().String())
	if err != nil {
		return fmt.Errorf("%s not found: %s", typeName, extractRkey(uri))
	}

	result := map[string]any{
		"uri":    uri,
		typeName: record,
	}

	fmt.Fprintln(cmd.Root().Writer, prettyJSON(result))
	return nil
}

// parseLocationCoords extracts lat/lon from a location record's location.string field.
func parseLocationCoords(m map[string]any) (lat, lon float64, ok bool) {
	loc := mapMap(m, "location")
	if loc == nil {
		return 0, 0, false
	}
	s := mapStr(loc, "string")
	if s == "" {
		return 0, 0, false
	}
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, latOk := parseFloat(strings.TrimSpace(parts[0]))
	lon, lonOk := parseFloat(strings.TrimSpace(parts[1]))
	if !latOk || !lonOk {
		return 0, 0, false
	}
	return lat, lon, true
}
