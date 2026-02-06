package cmd

import (
	"testing"
)

func TestCoordinateValidation(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		lon      float64
		validLat bool
		validLon bool
	}{
		// Valid coordinates
		{"origin", 0, 0, true, true},
		{"seattle", 47.6062, -122.3321, true, true},
		{"amazon_basin", -3.4653, -62.2159, true, true},
		{"max_lat", 90.0, 0, true, true},
		{"min_lat", -90.0, 0, true, true},
		{"max_lon", 0, 180.0, true, true},
		{"min_lon", 0, -180.0, true, true},
		{"corners", 90, 180, true, true},

		// Invalid latitudes
		{"lat_too_high", 91.0, 0, false, true},
		{"lat_too_low", -91.0, 0, false, true},
		{"lat_way_off", 200.0, 0, false, true},

		// Invalid longitudes
		{"lon_too_high", 0, 181.0, true, false},
		{"lon_too_low", 0, -181.0, true, false},
		{"lon_way_off", 0, 400.0, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			latValid := tt.lat >= -90 && tt.lat <= 90
			lonValid := tt.lon >= -180 && tt.lon <= 180

			if latValid != tt.validLat {
				t.Errorf("lat %v valid = %v, want %v", tt.lat, latValid, tt.validLat)
			}
			if lonValid != tt.validLon {
				t.Errorf("lon %v valid = %v, want %v", tt.lon, lonValid, tt.validLon)
			}
		})
	}
}

func TestLocationRecordFormat(t *testing.T) {
	rec := buildLocationRecord(47.6062, -122.3321, "Seattle", "Test city")

	// Check required LP v1.0 fields
	if rec["lpVersion"] != "1.0" {
		t.Errorf("lpVersion = %v, want 1.0", rec["lpVersion"])
	}
	if rec["srs"] != "http://www.opengis.net/def/crs/OGC/1.3/CRS84" {
		t.Errorf("srs should be OGC CRS84 URI")
	}
	if rec["locationType"] != "coordinate-decimal" {
		t.Errorf("locationType = %v, want coordinate-decimal", rec["locationType"])
	}
	if rec["$type"] != "app.certified.location" {
		t.Errorf("$type = %v, want app.certified.location", rec["$type"])
	}

	// Check location.string format
	loc, ok := rec["location"].(map[string]any)
	if !ok {
		t.Fatal("location should be a map")
	}
	coordStr, ok := loc["string"].(string)
	if !ok {
		t.Fatal("location.string should be a string")
	}
	if coordStr != "47.6062, -122.3321" {
		t.Errorf("location.string = %q, want '47.6062, -122.3321'", coordStr)
	}
}
