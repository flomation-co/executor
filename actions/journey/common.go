package journey_common

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	ProviderGoogle = "google"
	ProviderMapbox = "mapbox"

	ModeDriving = "driving"
	ModeWalking = "walking"
	ModeCycling = "cycling"
	ModeTransit = "transit"

	MetresPerMile = 1609.344
)

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type GeocodeResult struct {
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	FormattedAddress string  `json:"formatted_address"`
	Confidence       string  `json:"confidence"`
}

type ReverseGeocodeResult struct {
	Address  string `json:"address"`
	Street   string `json:"street"`
	City     string `json:"city"`
	Country  string `json:"country"`
	Postcode string `json:"postcode"`
}

type RouteParams struct {
	Origin      string
	Destination string
	Waypoints   []string
	Mode        string
	DepartAt    *time.Time
	Avoid       []string
}

type RouteStep struct {
	Instruction     string  `json:"instruction"`
	DistanceMetres  float64 `json:"distance_metres"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type RouteBounds struct {
	Northeast LatLng `json:"northeast"`
	Southwest LatLng `json:"southwest"`
}

type RouteResult struct {
	DistanceMetres   float64     `json:"distance_metres"`
	DistanceMiles    float64     `json:"distance_miles"`
	DurationSeconds  float64     `json:"duration_seconds"`
	DurationFriendly string      `json:"duration_friendly"`
	Polyline         string      `json:"polyline"`
	Steps            []RouteStep `json:"steps"`
	Bounds           RouteBounds `json:"bounds"`
}

type TravelTimeResult struct {
	DistanceMetres   float64 `json:"distance_metres"`
	DistanceMiles    float64 `json:"distance_miles"`
	DurationSeconds  float64 `json:"duration_seconds"`
	DurationFriendly string  `json:"duration_friendly"`
}

// OptimiseParams asks the provider to reorder Stops for shortest total path.
// Start and End are optional anchors — when both omitted the trip is treated
// as a closed cycle returning to the first stop. OptimiseFor accepts
// "duration" (default) or "distance"; not all providers honour "distance".
type OptimiseParams struct {
	Start       string
	End         string
	Stops       []string
	Mode        string
	OptimiseFor string
	DepartAt    *time.Time
}

type OptimiseLeg struct {
	From            string  `json:"from"`
	To              string  `json:"to"`
	DistanceMetres  float64 `json:"distance_metres"`
	DistanceMiles   float64 `json:"distance_miles"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type OptimiseResult struct {
	// OrderedStops is the original Stops slice rearranged into optimal order.
	// Start and End are NOT included here — they remain the trip's anchors.
	OrderedStops []string `json:"ordered_stops"`
	// WaypointOrder maps positions in OrderedStops back to the input Stops
	// slice. Useful when stops carry external IDs the caller wants to keep.
	WaypointOrder []int `json:"waypoint_order"`

	TotalDistanceMetres   float64 `json:"total_distance_metres"`
	TotalDistanceMiles    float64 `json:"total_distance_miles"`
	TotalDurationSeconds  float64 `json:"total_duration_seconds"`
	TotalDurationFriendly string  `json:"total_duration_friendly"`

	Legs []OptimiseLeg `json:"legs"`
}

// StaticMapMarker pins a labelled point on the rendered map.
type StaticMapMarker struct {
	Lat   float64
	Lng   float64
	Label string // single character; providers vary on multi-char support
	Color string // optional, provider-specific (e.g. "red", "0xFF0000")
}

type StaticMapParams struct {
	// Polyline is the encoded route (Google polyline algorithm) to overlay.
	// Required — drives both the path shape and the map's auto-bounds.
	Polyline string
	Width    int
	Height   int
	// Zoom 0 means "auto-fit to polyline bounds".
	Zoom    int
	Markers []StaticMapMarker
}

type Provider interface {
	Name() string
	Geocode(query, region string) (*GeocodeResult, error)
	ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error)
	CalculateRoute(params RouteParams) (*RouteResult, error)
	GetTravelTime(origin, destination, mode string, departAt *time.Time) (*TravelTimeResult, error)
	OptimiseRoute(params OptimiseParams) (*OptimiseResult, error)
	RenderStaticMap(params StaticMapParams) ([]byte, string, error)
}

var DefaultClient = &http.Client{Timeout: 60 * time.Second}

func NewProvider(name, apiKey string) (Provider, error) {
	return NewProviderWithClient(name, apiKey, DefaultClient)
}

// NewProviderWithClient lets tests inject a stub *http.Client whose Transport
// returns canned responses without making real network calls.
func NewProviderWithClient(name, apiKey string, client *http.Client) (Provider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("journey: api key is required for provider %q", name)
	}
	if client == nil {
		client = DefaultClient
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ProviderGoogle, "":
		return &googleProvider{apiKey: apiKey, client: client}, nil
	case ProviderMapbox:
		return &mapboxProvider{apiKey: apiKey, client: client}, nil
	}
	return nil, fmt.Errorf("journey: unknown provider %q", name)
}

func MetresToMiles(metres float64) float64 {
	return metres / MetresPerMile
}

// FriendlyDuration renders seconds as e.g. "2 hours 15 minutes" or "45 seconds".
// Mirrors the editor's friendlyDuration() output so AI replies match the UI.
func FriendlyDuration(seconds float64) string {
	if seconds <= 0 {
		return "0 seconds"
	}
	total := int64(math.Round(seconds))
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60
	secs := total % 60

	var parts []string
	if days > 0 {
		parts = append(parts, plural(days, "day"))
	}
	if hours > 0 {
		parts = append(parts, plural(hours, "hour"))
	}
	if minutes > 0 {
		parts = append(parts, plural(minutes, "minute"))
	}
	if secs > 0 && days == 0 && hours == 0 {
		parts = append(parts, plural(secs, "second"))
	}
	if len(parts) == 0 {
		return "less than a minute"
	}
	return strings.Join(parts, " ")
}

func plural(n int64, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// DecodePolyline implements Google's encoded polyline algorithm format. The
// same encoding is used by Google Directions, Mapbox Directions (when
// geometries=polyline), Mapbox Optimization, and most major mapping APIs.
//
// Reference: https://developers.google.com/maps/documentation/utilities/polylinealgorithm
//
// Returns the decoded sequence of (lat, lng) pairs as []LatLng. Each chunk
// of the input string encodes a delta from the previous coordinate; the
// loop accumulates absolute lat/lng as it walks.
func DecodePolyline(encoded string) []LatLng {
	if encoded == "" {
		return nil
	}
	var coords []LatLng
	index := 0
	lat, lng := 0, 0
	for index < len(encoded) {
		dlat, n := decodePolylineValue(encoded, index)
		if n == 0 {
			return coords
		}
		index += n
		dlng, n := decodePolylineValue(encoded, index)
		if n == 0 {
			return coords
		}
		index += n
		lat += dlat
		lng += dlng
		coords = append(coords, LatLng{
			Lat: float64(lat) * 1e-5,
			Lng: float64(lng) * 1e-5,
		})
	}
	return coords
}

func decodePolylineValue(s string, index int) (int, int) {
	result := 0
	shift := uint(0)
	consumed := 0
	for {
		if index+consumed >= len(s) {
			return 0, 0
		}
		b := int(s[index+consumed]) - 63
		consumed++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		result = ^(result >> 1)
	} else {
		result >>= 1
	}
	return result, consumed
}

// IsLatLng returns parsed coordinates if the input looks like "lat,lng".
// Pure numeric pairs go through routing APIs as coordinates; anything else
// is treated as a textual address.
func IsLatLng(value string) (LatLng, bool) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) != 2 {
		return LatLng{}, false
	}
	var lat, lng float64
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &lat); err != nil {
		return LatLng{}, false
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &lng); err != nil {
		return LatLng{}, false
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return LatLng{}, false
	}
	return LatLng{Lat: lat, Lng: lng}, true
}
