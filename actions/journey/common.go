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

type Provider interface {
	Name() string
	Geocode(query, region string) (*GeocodeResult, error)
	ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error)
	CalculateRoute(params RouteParams) (*RouteResult, error)
	GetTravelTime(origin, destination, mode string, departAt *time.Time) (*TravelTimeResult, error)
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
