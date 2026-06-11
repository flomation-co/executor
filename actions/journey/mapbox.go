package journey_common

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type mapboxProvider struct {
	apiKey string
	client *http.Client
}

const mapboxBaseURL = "https://api.mapbox.com"

func (m *mapboxProvider) Name() string { return ProviderMapbox }

type mapboxGeocodeResponse struct {
	Features []struct {
		PlaceName   string             `json:"place_name"`
		Center      []float64          `json:"center"`
		Relevance   float64            `json:"relevance"`
		Text        string             `json:"text"`
		Context     []mapboxContextItm `json:"context"`
		Address     string             `json:"address"`
		Properties  map[string]any     `json:"properties"`
		PlaceTypes  []string           `json:"place_type"`
	} `json:"features"`
	Message string `json:"message"`
}

type mapboxContextItm struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (m *mapboxProvider) Geocode(query, region string) (*GeocodeResult, error) {
	q := url.Values{"access_token": {m.apiKey}, "limit": {"1"}}
	if region != "" {
		q.Set("country", region)
	}
	path := "/geocoding/v5/mapbox.places/" + url.PathEscape(query) + ".json"
	body, err := m.get(path, q)
	if err != nil {
		return nil, err
	}
	var resp mapboxGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/mapbox: invalid geocode response: %w", err)
	}
	if resp.Message != "" {
		return nil, fmt.Errorf("journey/mapbox: %s", resp.Message)
	}
	if len(resp.Features) == 0 {
		return nil, fmt.Errorf("journey/mapbox: no geocode results for %q", query)
	}
	f := resp.Features[0]
	if len(f.Center) < 2 {
		return nil, fmt.Errorf("journey/mapbox: malformed feature center")
	}
	return &GeocodeResult{
		Latitude:         f.Center[1],
		Longitude:        f.Center[0],
		FormattedAddress: f.PlaceName,
		Confidence:       mapboxConfidence(f.Relevance),
	}, nil
}

func (m *mapboxProvider) ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error) {
	q := url.Values{"access_token": {m.apiKey}, "limit": {"1"}}
	path := fmt.Sprintf("/geocoding/v5/mapbox.places/%f,%f.json", lng, lat)
	body, err := m.get(path, q)
	if err != nil {
		return nil, err
	}
	var resp mapboxGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/mapbox: invalid reverse geocode response: %w", err)
	}
	if resp.Message != "" {
		return nil, fmt.Errorf("journey/mapbox: %s", resp.Message)
	}
	if len(resp.Features) == 0 {
		return nil, fmt.Errorf("journey/mapbox: no reverse geocode results")
	}
	f := resp.Features[0]
	out := &ReverseGeocodeResult{Address: f.PlaceName, Street: f.Text}
	for _, c := range f.Context {
		switch {
		case strings.HasPrefix(c.ID, "postcode"):
			out.Postcode = c.Text
		case strings.HasPrefix(c.ID, "place"):
			if out.City == "" {
				out.City = c.Text
			}
		case strings.HasPrefix(c.ID, "country"):
			out.Country = c.Text
		}
	}
	return out, nil
}

type mapboxDirectionsResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Routes  []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry string  `json:"geometry"`
		Legs     []struct {
			Steps []struct {
				Maneuver struct {
					Instruction string `json:"instruction"`
				} `json:"maneuver"`
				Distance float64 `json:"distance"`
				Duration float64 `json:"duration"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
	Waypoints []struct {
		Location []float64 `json:"location"`
	} `json:"waypoints"`
}

func (m *mapboxProvider) CalculateRoute(p RouteParams) (*RouteResult, error) {
	coords, err := m.coordsString(p.Origin, p.Waypoints, p.Destination)
	if err != nil {
		return nil, err
	}
	path := "/directions/v5/mapbox/" + mapboxProfile(p.Mode) + "/" + coords
	q := url.Values{
		"access_token": {m.apiKey},
		"geometries":   {"polyline"},
		"overview":     {"full"},
		"steps":        {"true"},
	}
	if p.DepartAt != nil {
		q.Set("depart_at", p.DepartAt.UTC().Format(time.RFC3339))
	}
	if len(p.Avoid) > 0 {
		q.Set("exclude", strings.Join(mapboxExclude(p.Avoid), ","))
	}
	body, err := m.get(path, q)
	if err != nil {
		return nil, err
	}
	var resp mapboxDirectionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/mapbox: invalid directions response: %w", err)
	}
	if resp.Code != "Ok" {
		msg := resp.Message
		if msg == "" {
			msg = resp.Code
		}
		return nil, fmt.Errorf("journey/mapbox: directions failed: %s", msg)
	}
	if len(resp.Routes) == 0 {
		return nil, fmt.Errorf("journey/mapbox: no routes returned")
	}
	r := resp.Routes[0]
	var steps []RouteStep
	for _, leg := range r.Legs {
		for _, s := range leg.Steps {
			steps = append(steps, RouteStep{
				Instruction:     s.Maneuver.Instruction,
				DistanceMetres:  s.Distance,
				DurationSeconds: s.Duration,
			})
		}
	}
	return &RouteResult{
		DistanceMetres:   r.Distance,
		DistanceMiles:    MetresToMiles(r.Distance),
		DurationSeconds:  r.Duration,
		DurationFriendly: FriendlyDuration(r.Duration),
		Polyline:         r.Geometry,
		Steps:            steps,
		Bounds:           mapboxBoundsFromWaypoints(resp.Waypoints),
	}, nil
}

func (m *mapboxProvider) GetTravelTime(origin, destination, mode string, departAt *time.Time) (*TravelTimeResult, error) {
	route, err := m.CalculateRoute(RouteParams{
		Origin:      origin,
		Destination: destination,
		Mode:        mode,
		DepartAt:    departAt,
	})
	if err != nil {
		return nil, err
	}
	return &TravelTimeResult{
		DistanceMetres:   route.DistanceMetres,
		DistanceMiles:    route.DistanceMiles,
		DurationSeconds:  route.DurationSeconds,
		DurationFriendly: route.DurationFriendly,
	}, nil
}

func (m *mapboxProvider) coordsString(origin string, waypoints []string, destination string) (string, error) {
	all := append([]string{origin}, waypoints...)
	all = append(all, destination)
	parts := make([]string, 0, len(all))
	for _, s := range all {
		ll, err := m.toLngLat(s)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%f,%f", ll.Lng, ll.Lat))
	}
	return strings.Join(parts, ";"), nil
}

// toLngLat resolves textual input via Mapbox geocoding so the Directions API
// always receives coordinates. Google's Directions API accepts addresses
// directly; Mapbox doesn't, which is why this hop only exists here.
func (m *mapboxProvider) toLngLat(value string) (LatLng, error) {
	if ll, ok := IsLatLng(value); ok {
		return ll, nil
	}
	geo, err := m.Geocode(value, "")
	if err != nil {
		return LatLng{}, err
	}
	return LatLng{Lat: geo.Latitude, Lng: geo.Longitude}, nil
}

func (m *mapboxProvider) get(path string, query url.Values) ([]byte, error) {
	u := mapboxBaseURL + path + "?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("journey/mapbox: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("journey/mapbox: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("journey/mapbox: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func mapboxProfile(mode string) string {
	switch strings.ToLower(mode) {
	case ModeWalking:
		return "walking"
	case ModeCycling:
		return "cycling"
	case ModeTransit:
		return "driving"
	default:
		return "driving-traffic"
	}
}

func mapboxExclude(avoid []string) []string {
	out := make([]string, 0, len(avoid))
	for _, a := range avoid {
		switch strings.ToLower(a) {
		case "tolls":
			out = append(out, "toll")
		case "motorways":
			out = append(out, "motorway")
		case "ferries":
			out = append(out, "ferry")
		}
	}
	return out
}

func mapboxConfidence(relevance float64) string {
	switch {
	case relevance >= 0.9:
		return "high"
	case relevance >= 0.6:
		return "medium"
	default:
		return "low"
	}
}

func mapboxBoundsFromWaypoints(wps []struct {
	Location []float64 `json:"location"`
}) RouteBounds {
	if len(wps) == 0 {
		return RouteBounds{}
	}
	minLat, maxLat := 90.0, -90.0
	minLng, maxLng := 180.0, -180.0
	for _, w := range wps {
		if len(w.Location) < 2 {
			continue
		}
		lng, lat := w.Location[0], w.Location[1]
		if lat < minLat {
			minLat = lat
		}
		if lat > maxLat {
			maxLat = lat
		}
		if lng < minLng {
			minLng = lng
		}
		if lng > maxLng {
			maxLng = lng
		}
	}
	return RouteBounds{
		Northeast: LatLng{Lat: maxLat, Lng: maxLng},
		Southwest: LatLng{Lat: minLat, Lng: minLng},
	}
}

// RenderStaticMap builds a Mapbox Static Images URL. Unlike Google's
// query-param style, Mapbox encodes overlays into the URL PATH itself —
// `path-{width}+{colour}-{opacity}({encoded_polyline})` for routes and
// `pin-{size}-{label}+{colour}({lng},{lat})` for markers. The route's
// encoded polyline must be URL-path-escaped because it contains characters
// the path parser would otherwise interpret (e.g. `?` `#`).
//
// The `auto` viewport token is the equivalent of Google's "no zoom/center"
// — Mapbox computes bounds to fit all overlays.
func (m *mapboxProvider) RenderStaticMap(p StaticMapParams) ([]byte, string, error) {
	if strings.TrimSpace(p.Polyline) == "" {
		return nil, "", fmt.Errorf("journey/mapbox: render_static_map requires a polyline")
	}
	width, height := p.Width, p.Height
	if width <= 0 {
		width = 600
	}
	if height <= 0 {
		height = 400
	}

	overlays := []string{
		"path-4+0000ff-0.9(" + url.PathEscape(p.Polyline) + ")",
	}
	for _, marker := range p.Markers {
		label := marker.Label
		if label == "" {
			label = "m"
		}
		colour := strings.TrimPrefix(marker.Color, "0x")
		if colour == "" {
			colour = "000000"
		}
		overlays = append(overlays, fmt.Sprintf("pin-s-%s+%s(%f,%f)",
			url.PathEscape(label), colour, marker.Lng, marker.Lat))
	}

	viewport := "auto"
	if p.Zoom > 0 {
		viewport = fmt.Sprintf("%d", p.Zoom)
	}

	path := fmt.Sprintf("/styles/v1/mapbox/streets-v12/static/%s/%s/%dx%d",
		strings.Join(overlays, ","), viewport, width, height)
	q := url.Values{"access_token": {m.apiKey}}

	u := mapboxBaseURL + path + "?" + q.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("journey/mapbox: static map request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("journey/mapbox: read static map body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("journey/mapbox: static map http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	return body, mime, nil
}

type mapboxOptimisationResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Trips     []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Legs     []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
		} `json:"legs"`
	} `json:"trips"`
	Waypoints []struct {
		WaypointIndex int       `json:"waypoint_index"`
		TripsIndex    int       `json:"trips_index"`
		Location      []float64 `json:"location"`
		Name          string    `json:"name"`
	} `json:"waypoints"`
}

// OptimiseRoute uses Mapbox's Optimization v1 API. Unlike Google's "add
// optimize:true to Directions", Mapbox treats it as a separate endpoint that
// always returns a reordered waypoint list. Source and destination handling
// is controlled by `source` and `destination` query params: "first"/"last"
// pin them, "any" lets the optimiser pick. We translate our Start/End/Stops
// semantics onto those params.
func (m *mapboxProvider) OptimiseRoute(p OptimiseParams) (*OptimiseResult, error) {
	if len(p.Stops) == 0 {
		return nil, fmt.Errorf("journey/mapbox: optimise_route requires at least one stop")
	}

	// Build the input list in input-order: [start?, ...stops, end?]. Mapbox
	// indexes by this order; the response gives the optimal visit order via
	// each waypoint's waypoint_index.
	inputs := make([]string, 0, len(p.Stops)+2)
	startOffset := 0
	if p.Start != "" {
		inputs = append(inputs, p.Start)
		startOffset = 1
	}
	inputs = append(inputs, p.Stops...)
	if p.End != "" {
		inputs = append(inputs, p.End)
	}

	coords, err := m.coordsString(inputs[0], inputs[1:len(inputs)-1], inputs[len(inputs)-1])
	if err != nil {
		// Single-input edge case (stops==1, no anchors): coordsString needs
		// at least origin+destination. Skip optimisation, just return the
		// stop unchanged.
		if len(inputs) == 1 {
			return &OptimiseResult{
				OrderedStops:          []string{p.Stops[0]},
				WaypointOrder:         []int{0},
				TotalDurationFriendly: "0 seconds",
			}, nil
		}
		return nil, err
	}

	q := url.Values{
		"access_token":  {m.apiKey},
		"geometries":    {"polyline"},
		"roundtrip":     {"false"},
	}
	if p.Start == "" && p.End == "" {
		q.Set("roundtrip", "true")
		q.Set("source", "first")
		q.Set("destination", "last")
	} else {
		if p.Start != "" {
			q.Set("source", "first")
		} else {
			q.Set("source", "any")
		}
		if p.End != "" {
			q.Set("destination", "last")
		} else {
			q.Set("destination", "any")
		}
	}

	path := "/optimized-trips/v1/mapbox/" + mapboxProfile(p.Mode) + "/" + coords
	body, err := m.get(path, q)
	if err != nil {
		return nil, err
	}
	var resp mapboxOptimisationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/mapbox: invalid optimisation response: %w", err)
	}
	if resp.Code != "Ok" {
		msg := resp.Message
		if msg == "" {
			msg = resp.Code
		}
		return nil, fmt.Errorf("journey/mapbox: optimisation failed: %s", msg)
	}
	if len(resp.Trips) == 0 {
		return nil, fmt.Errorf("journey/mapbox: no trips returned for optimisation")
	}

	// Build waypoint_index → input_position map. Each response waypoint
	// corresponds to one input by position; waypoint_index is the visit
	// order. Reverse-index: for each visit position, find the input it came
	// from.
	visitOrder := make([]int, len(resp.Waypoints))
	for inputPos, wp := range resp.Waypoints {
		if wp.WaypointIndex >= 0 && wp.WaypointIndex < len(visitOrder) {
			visitOrder[wp.WaypointIndex] = inputPos
		}
	}

	// Strip Start/End anchors from the visit order and remap remaining
	// indices back to the original p.Stops slice.
	ordered := make([]string, 0, len(p.Stops))
	waypointOrder := make([]int, 0, len(p.Stops))
	for _, inputPos := range visitOrder {
		stopIdx := inputPos - startOffset
		if stopIdx < 0 || stopIdx >= len(p.Stops) {
			continue
		}
		ordered = append(ordered, p.Stops[stopIdx])
		waypointOrder = append(waypointOrder, stopIdx)
	}

	trip := resp.Trips[0]
	legs := make([]OptimiseLeg, 0, len(trip.Legs))
	for i, leg := range trip.Legs {
		from, to := "", ""
		if i < len(visitOrder) {
			if idx := visitOrder[i]; idx >= 0 && idx < len(inputs) {
				from = inputs[idx]
			}
		}
		if i+1 < len(visitOrder) {
			if idx := visitOrder[i+1]; idx >= 0 && idx < len(inputs) {
				to = inputs[idx]
			}
		}
		legs = append(legs, OptimiseLeg{
			From:            from,
			To:              to,
			DistanceMetres:  leg.Distance,
			DistanceMiles:   MetresToMiles(leg.Distance),
			DurationSeconds: leg.Duration,
		})
	}

	return &OptimiseResult{
		OrderedStops:          ordered,
		WaypointOrder:         waypointOrder,
		TotalDistanceMetres:   trip.Distance,
		TotalDistanceMiles:    MetresToMiles(trip.Distance),
		TotalDurationSeconds:  trip.Duration,
		TotalDurationFriendly: FriendlyDuration(trip.Duration),
		Legs:                  legs,
	}, nil
}
