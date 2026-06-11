package journey_common

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// stubTransport returns the same canned response for any request, recording
// the inbound *http.Request so tests can assert on URL/params.
type stubTransport struct {
	status int
	body   string
	last   *http.Request
	err    error
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.last = req
	if s.err != nil {
		return nil, s.err
	}
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewBufferString(s.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func stubClient(status int, body string) (*http.Client, *stubTransport) {
	tr := &stubTransport{status: status, body: body}
	return &http.Client{Transport: tr, Timeout: time.Second}, tr
}

func TestIsLatLng(t *testing.T) {
	RegisterTestingT(t)

	ll, ok := IsLatLng("51.5034,-0.1276")
	Expect(ok).To(BeTrue())
	Expect(ll.Lat).To(BeNumerically("~", 51.5034, 1e-6))
	Expect(ll.Lng).To(BeNumerically("~", -0.1276, 1e-6))

	_, ok = IsLatLng("London")
	Expect(ok).To(BeFalse())

	_, ok = IsLatLng("NW1,6XE")
	Expect(ok).To(BeFalse())

	_, ok = IsLatLng("91,0")
	Expect(ok).To(BeFalse())

	_, ok = IsLatLng("0,181")
	Expect(ok).To(BeFalse())
}

func TestFriendlyDuration(t *testing.T) {
	RegisterTestingT(t)

	Expect(FriendlyDuration(0)).To(Equal("0 seconds"))
	Expect(FriendlyDuration(45)).To(Equal("45 seconds"))
	Expect(FriendlyDuration(60)).To(Equal("1 minute"))
	Expect(FriendlyDuration(3600)).To(Equal("1 hour"))
	Expect(FriendlyDuration(3660)).To(Equal("1 hour 1 minute"))
	Expect(FriendlyDuration(8100)).To(Equal("2 hours 15 minutes"))
	Expect(FriendlyDuration(86400)).To(Equal("1 day"))
	Expect(FriendlyDuration(90061)).To(Equal("1 day 1 hour 1 minute"))
}

func TestMetresToMiles(t *testing.T) {
	RegisterTestingT(t)

	Expect(MetresToMiles(1609.344)).To(BeNumerically("~", 1.0, 1e-9))
	Expect(MetresToMiles(0)).To(Equal(0.0))
}

func TestNewProviderUnknown(t *testing.T) {
	RegisterTestingT(t)

	_, err := NewProvider("not_a_provider", "key")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("unknown provider"))
}

func TestNewProviderRequiresKey(t *testing.T) {
	RegisterTestingT(t)

	_, err := NewProvider("google", "  ")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api key is required"))
}

const googleGeocodeOK = `{
  "status": "OK",
  "results": [{
    "formatted_address": "10 Downing St, London SW1A 2AA, UK",
    "address_components": [
      {"long_name": "10", "short_name": "10", "types": ["street_number"]},
      {"long_name": "Downing Street", "short_name": "Downing St", "types": ["route"]},
      {"long_name": "London", "short_name": "London", "types": ["postal_town"]},
      {"long_name": "United Kingdom", "short_name": "GB", "types": ["country", "political"]},
      {"long_name": "SW1A 2AA", "short_name": "SW1A 2AA", "types": ["postal_code"]}
    ],
    "geometry": {
      "location": {"lat": 51.5033635, "lng": -0.1276248},
      "location_type": "ROOFTOP"
    }
  }]
}`

func TestGoogleGeocode(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleGeocodeOK)
	p, err := NewProviderWithClient("google", "test-key", client)
	Expect(err).ToNot(HaveOccurred())

	res, err := p.Geocode("10 Downing Street", "gb")
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Latitude).To(BeNumerically("~", 51.5033635, 1e-6))
	Expect(res.Longitude).To(BeNumerically("~", -0.1276248, 1e-6))
	Expect(res.FormattedAddress).To(Equal("10 Downing St, London SW1A 2AA, UK"))
	Expect(res.Confidence).To(Equal("high"))

	Expect(tr.last.URL.Host).To(Equal("maps.googleapis.com"))
	Expect(tr.last.URL.Query().Get("key")).To(Equal("test-key"))
	Expect(tr.last.URL.Query().Get("region")).To(Equal("gb"))
}

func TestGoogleGeocodeZeroResults(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, `{"status":"ZERO_RESULTS","results":[]}`)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.Geocode("kjlsdjflkjsdf", "")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("no geocode results"))
}

func TestGoogleGeocodeErrorStatus(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, `{"status":"REQUEST_DENIED","error_message":"API key invalid"}`)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.Geocode("anything", "")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("REQUEST_DENIED"))
	Expect(err.Error()).To(ContainSubstring("API key invalid"))
}

const googleReverseGeocodeOK = `{
  "status": "OK",
  "results": [{
    "formatted_address": "10 Downing St, London SW1A 2AA, UK",
    "address_components": [
      {"long_name": "Downing Street", "types": ["route"]},
      {"long_name": "London", "types": ["postal_town"]},
      {"long_name": "United Kingdom", "types": ["country"]},
      {"long_name": "SW1A 2AA", "types": ["postal_code"]}
    ],
    "geometry": {"location": {"lat": 51.5, "lng": -0.12}, "location_type": "ROOFTOP"}
  }]
}`

func TestGoogleReverseGeocode(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, googleReverseGeocodeOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	res, err := p.ReverseGeocode(51.5, -0.12)
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Street).To(Equal("Downing Street"))
	Expect(res.City).To(Equal("London"))
	Expect(res.Country).To(Equal("United Kingdom"))
	Expect(res.Postcode).To(Equal("SW1A 2AA"))
}

const googleDirectionsOK = `{
  "status": "OK",
  "routes": [{
    "overview_polyline": {"points": "abc123xyz"},
    "bounds": {"northeast": {"lat": 53.5, "lng": -2.2}, "southwest": {"lat": 51.5, "lng": -2.3}},
    "legs": [{
      "distance": {"value": 322000, "text": "322 km"},
      "duration": {"value": 13500, "text": "3 hours 45 mins"},
      "steps": [
        {"html_instructions": "Head <b>north</b> on A40", "distance": {"value": 500}, "duration": {"value": 60}},
        {"html_instructions": "Continue onto M40", "distance": {"value": 321500}, "duration": {"value": 13440}}
      ]
    }]
  }]
}`

func TestGoogleCalculateRoute(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleDirectionsOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	depart := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)
	r, err := p.CalculateRoute(RouteParams{
		Origin:      "London",
		Destination: "Manchester",
		Waypoints:   []string{"Birmingham"},
		Mode:        ModeDriving,
		DepartAt:    &depart,
		Avoid:       []string{"tolls"},
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(r.DistanceMetres).To(Equal(322000.0))
	Expect(r.DistanceMiles).To(BeNumerically("~", 200.08, 0.1))
	Expect(r.DurationSeconds).To(Equal(13500.0))
	Expect(r.DurationFriendly).To(Equal("3 hours 45 minutes"))
	Expect(r.Polyline).To(Equal("abc123xyz"))
	Expect(r.Steps).To(HaveLen(2))
	Expect(r.Steps[0].Instruction).To(Equal("Head north on A40"))
	Expect(r.Bounds.Northeast.Lat).To(Equal(53.5))

	Expect(tr.last.URL.Query().Get("waypoints")).To(Equal("Birmingham"))
	Expect(tr.last.URL.Query().Get("avoid")).To(Equal("tolls"))
	Expect(tr.last.URL.Query().Get("departure_time")).ToNot(BeEmpty())
}

const googleDistanceMatrixOK = `{
  "status": "OK",
  "rows": [{"elements": [{"status": "OK", "distance": {"value": 200000}, "duration": {"value": 7200}}]}]
}`

func TestGoogleGetTravelTime(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, googleDistanceMatrixOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	tt, err := p.GetTravelTime("London", "Manchester", ModeDriving, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(tt.DistanceMiles).To(BeNumerically("~", 124.27, 0.1))
	Expect(tt.DurationFriendly).To(Equal("2 hours"))
}

func TestGoogleModeMapping(t *testing.T) {
	RegisterTestingT(t)

	Expect(googleMode(ModeWalking)).To(Equal("walking"))
	Expect(googleMode(ModeCycling)).To(Equal("bicycling"))
	Expect(googleMode(ModeTransit)).To(Equal("transit"))
	Expect(googleMode(ModeDriving)).To(Equal("driving"))
	Expect(googleMode("")).To(Equal("driving"))
}

func TestGoogleConfidence(t *testing.T) {
	RegisterTestingT(t)

	Expect(googleConfidence("ROOFTOP", false)).To(Equal("high"))
	Expect(googleConfidence("GEOMETRIC_CENTER", false)).To(Equal("medium"))
	Expect(googleConfidence("APPROXIMATE", false)).To(Equal("low"))
	Expect(googleConfidence("ROOFTOP", true)).To(Equal("low"))
}

const mapboxGeocodeOK = `{
  "features": [{
    "place_name": "10 Downing Street, London SW1A 2AA, United Kingdom",
    "center": [-0.1276, 51.5034],
    "relevance": 0.95,
    "text": "Downing Street",
    "place_type": ["address"],
    "context": [
      {"id": "postcode.1", "text": "SW1A 2AA"},
      {"id": "place.1", "text": "London"},
      {"id": "country.1", "text": "United Kingdom"}
    ]
  }]
}`

func TestMapboxGeocode(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, mapboxGeocodeOK)
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	res, err := p.Geocode("10 Downing Street", "gb")
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Latitude).To(Equal(51.5034))
	Expect(res.Longitude).To(Equal(-0.1276))
	Expect(res.Confidence).To(Equal("high"))
}

func TestMapboxReverseGeocode(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, mapboxGeocodeOK)
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	res, err := p.ReverseGeocode(51.5034, -0.1276)
	Expect(err).ToNot(HaveOccurred())
	Expect(res.City).To(Equal("London"))
	Expect(res.Postcode).To(Equal("SW1A 2AA"))
	Expect(res.Country).To(Equal("United Kingdom"))
	Expect(res.Street).To(Equal("Downing Street"))
}

const mapboxDirectionsOK = `{
  "code": "Ok",
  "routes": [{
    "distance": 322000,
    "duration": 13500,
    "geometry": "polylinedata",
    "legs": [{"steps": [
      {"maneuver": {"instruction": "Head north on A40"}, "distance": 500, "duration": 60}
    ]}]
  }],
  "waypoints": [
    {"location": [-0.12, 51.5]},
    {"location": [-2.24, 53.48]}
  ]
}`

func TestMapboxCalculateRoute(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, mapboxDirectionsOK)
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	r, err := p.CalculateRoute(RouteParams{
		Origin:      "51.5,-0.12",
		Destination: "53.48,-2.24",
		Mode:        ModeDriving,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(r.DistanceMetres).To(Equal(322000.0))
	Expect(r.DurationFriendly).To(Equal("3 hours 45 minutes"))
	Expect(r.Polyline).To(Equal("polylinedata"))
	Expect(r.Steps).To(HaveLen(1))
	Expect(r.Bounds.Northeast.Lat).To(BeNumerically("~", 53.48, 0.01))

	Expect(strings.Contains(tr.last.URL.Path, "/directions/v5/mapbox/driving-traffic/")).To(BeTrue())
}

func TestMapboxExcludeMapping(t *testing.T) {
	RegisterTestingT(t)

	Expect(mapboxExclude([]string{"tolls", "ferries"})).To(ConsistOf("toll", "ferry"))
	Expect(mapboxExclude(nil)).To(BeEmpty())
}

func TestMapboxConfidence(t *testing.T) {
	RegisterTestingT(t)

	Expect(mapboxConfidence(0.95)).To(Equal("high"))
	Expect(mapboxConfidence(0.7)).To(Equal("medium"))
	Expect(mapboxConfidence(0.3)).To(Equal("low"))
}

func TestStripHTML(t *testing.T) {
	RegisterTestingT(t)

	Expect(stripHTML("Head <b>north</b> on A40")).To(Equal("Head north on A40"))
	Expect(stripHTML("Plain text")).To(Equal("Plain text"))
	Expect(stripHTML("<wbr/>Onto <i>M40</i>")).To(Equal("Onto M40"))
}
