package cmd

import (
	"testing"
)

func TestExtractRkey(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"full_aturi", "at://did:plc:abc123/org.hypercerts.claim.activity/3abc", "3abc"},
		{"short", "3abc", "3abc"},
		{"with_collection", "at://did:plc:xyz/com.example.record/rkey123", "rkey123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRkey(tt.uri)
			if got != tt.want {
				t.Errorf("extractRkey(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestResolveRecordURI(t *testing.T) {
	tests := []struct {
		name       string
		did        string
		collection string
		idOrURI    string
		want       string
	}{
		{"short_id", "did:plc:abc", "org.example.record", "rkey1", "at://did:plc:abc/org.example.record/rkey1"},
		{"full_aturi", "did:plc:abc", "org.example.record", "at://did:plc:xyz/org.other/rkey2", "at://did:plc:xyz/org.other/rkey2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRecordURI(tt.did, tt.collection, tt.idOrURI)
			if got != tt.want {
				t.Errorf("resolveRecordURI = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapStr(t *testing.T) {
	m := map[string]any{
		"name":  "Alice",
		"age":   42,
		"empty": "",
	}

	if got := mapStr(m, "name"); got != "Alice" {
		t.Errorf("mapStr(name) = %q, want %q", got, "Alice")
	}
	if got := mapStr(m, "missing"); got != "" {
		t.Errorf("mapStr(missing) = %q, want empty", got)
	}
	if got := mapStr(m, "age"); got != "" {
		t.Errorf("mapStr(age) = %q, want empty (not a string)", got)
	}
	if got := mapStr(m, "empty"); got != "" {
		t.Errorf("mapStr(empty) = %q, want empty", got)
	}
}

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"date_only", "2025-01-15", "2025-01-15T00:00:00Z"},
		{"full_rfc3339", "2025-01-15T10:30:00Z", "2025-01-15T10:30:00Z"},
		{"short_string", "today", "today"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDate(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPrettyJSON(t *testing.T) {
	m := map[string]any{"key": "value"}
	got := prettyJSON(m)
	if got == "" {
		t.Error("prettyJSON returned empty")
	}
	if got[0] != '{' {
		t.Errorf("prettyJSON should start with '{', got %q", got[:1])
	}
}

func TestBuildStrongRef(t *testing.T) {
	ref := buildStrongRef("at://did:plc:abc/org.example/rkey", "bafycid123")
	if ref["uri"] != "at://did:plc:abc/org.example/rkey" {
		t.Errorf("uri = %q, want at://...", ref["uri"])
	}
	if ref["cid"] != "bafycid123" {
		t.Errorf("cid = %q, want bafycid123", ref["cid"])
	}
}

func TestMapMap(t *testing.T) {
	m := map[string]any{
		"nested": map[string]any{"inner": "value"},
		"string": "not a map",
	}

	if got := mapMap(m, "nested"); got == nil {
		t.Error("mapMap(nested) returned nil")
	} else if got["inner"] != "value" {
		t.Errorf("mapMap(nested)[inner] = %v, want 'value'", got["inner"])
	}
	if got := mapMap(m, "string"); got != nil {
		t.Error("mapMap(string) should return nil for non-map")
	}
	if got := mapMap(m, "missing"); got != nil {
		t.Error("mapMap(missing) should return nil")
	}
}

func TestMapSlice(t *testing.T) {
	m := map[string]any{
		"items":  []any{"a", "b", "c"},
		"string": "not a slice",
	}

	if got := mapSlice(m, "items"); got == nil {
		t.Error("mapSlice(items) returned nil")
	} else if len(got) != 3 {
		t.Errorf("mapSlice(items) len = %d, want 3", len(got))
	}
	if got := mapSlice(m, "string"); got != nil {
		t.Error("mapSlice(string) should return nil for non-slice")
	}
	if got := mapSlice(m, "missing"); got != nil {
		t.Error("mapSlice(missing) should return nil")
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
		ok    bool
	}{
		{"integer", "42", 42.0, true},
		{"decimal", "3.14159", 3.14159, true},
		{"negative", "-12.5", -12.5, true},
		{"invalid", "abc", 0, false},
		{"empty", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseFloat(tt.input)
			if ok != tt.ok {
				t.Errorf("parseFloat(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("parseFloat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildLocationRecord(t *testing.T) {
	rec := buildLocationRecord(47.6062, -122.3321, "Seattle", "Pacific Northwest city")

	if rec["$type"] != "app.certified.location" {
		t.Errorf("$type = %v, want app.certified.location", rec["$type"])
	}
	if rec["lpVersion"] != "1.0" {
		t.Errorf("lpVersion = %v, want 1.0", rec["lpVersion"])
	}
	if rec["srs"] != "http://www.opengis.net/def/crs/OGC/1.3/CRS84" {
		t.Errorf("srs = %v, want OGC CRS84 URI", rec["srs"])
	}
	if rec["locationType"] != "coordinate-decimal" {
		t.Errorf("locationType = %v, want coordinate-decimal", rec["locationType"])
	}
	if rec["name"] != "Seattle" {
		t.Errorf("name = %v, want Seattle", rec["name"])
	}
	if rec["description"] != "Pacific Northwest city" {
		t.Errorf("description = %v, want 'Pacific Northwest city'", rec["description"])
	}

	loc, ok := rec["location"].(map[string]any)
	if !ok {
		t.Fatal("location is not a map")
	}
	if loc["$type"] != "app.certified.location#string" {
		t.Errorf("location.$type = %v, want app.certified.location#string", loc["$type"])
	}
	if loc["string"] != "47.6062, -122.3321" {
		t.Errorf("location.string = %v, want '47.6062, -122.3321'", loc["string"])
	}

	if rec["createdAt"] == nil || rec["createdAt"] == "" {
		t.Error("createdAt should be set")
	}
}

func TestParseLocationCoords(t *testing.T) {
	tests := []struct {
		name    string
		record  map[string]any
		wantLat float64
		wantLon float64
		wantOk  bool
	}{
		{
			name: "valid_coords",
			record: map[string]any{
				"location": map[string]any{
					"string": "47.6062, -122.3321",
				},
			},
			wantLat: 47.6062,
			wantLon: -122.3321,
			wantOk:  true,
		},
		{
			name: "no_location",
			record: map[string]any{
				"name": "test",
			},
			wantOk: false,
		},
		{
			name: "missing_string",
			record: map[string]any{
				"location": map[string]any{},
			},
			wantOk: false,
		},
		{
			name: "invalid_format",
			record: map[string]any{
				"location": map[string]any{
					"string": "not,valid,coords",
				},
			},
			wantOk: false,
		},
		{
			name: "non_numeric",
			record: map[string]any{
				"location": map[string]any{
					"string": "abc, def",
				},
			},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon, ok := parseLocationCoords(tt.record)
			if ok != tt.wantOk {
				t.Errorf("parseLocationCoords ok = %v, want %v", ok, tt.wantOk)
			}
			if ok {
				if lat != tt.wantLat {
					t.Errorf("lat = %v, want %v", lat, tt.wantLat)
				}
				if lon != tt.wantLon {
					t.Errorf("lon = %v, want %v", lon, tt.wantLon)
				}
			}
		})
	}
}
