package journey_common

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type googleProvider struct {
	apiKey string
	client *http.Client
}

const (
	googleBaseURL = "https://maps.googleapis.com/maps/api"
)

func (g *googleProvider) Name() string { return ProviderGoogle }

type googleGeocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress  string                 `json:"formatted_address"`
		AddressComponents []googleAddressComp    `json:"address_components"`
		Geometry          googleGeocodeGeometry  `json:"geometry"`
		Types             []string               `json:"types"`
		PartialMatch      bool                   `json:"partial_match"`
		PlaceID           string                 `json:"place_id"`
		Plus              map[string]interface{} `json:"plus_code,omitempty"`
	} `json:"results"`
	ErrorMessage string `json:"error_message"`
}

type googleAddressComp struct {
	LongName  string   `json:"long_name"`
	ShortName string   `json:"short_name"`
	Types     []string `json:"types"`
}

type googleGeocodeGeometry struct {
	Location     LatLng `json:"location"`
	LocationType string `json:"location_type"`
}

func (g *googleProvider) Geocode(query, region string) (*GeocodeResult, error) {
	q := url.Values{
		"address": {query},
		"key":     {g.apiKey},
	}
	if region != "" {
		q.Set("region", region)
	}
	body, err := g.get("/geocode/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid geocode response: %w", err)
	}
	if err := googleStatusErr("geocode", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("journey/google: no geocode results for %q", query)
	}
	r := resp.Results[0]
	return &GeocodeResult{
		Latitude:         r.Geometry.Location.Lat,
		Longitude:        r.Geometry.Location.Lng,
		FormattedAddress: r.FormattedAddress,
		Confidence:       googleConfidence(r.Geometry.LocationType, r.PartialMatch),
	}, nil
}

func (g *googleProvider) ReverseGeocode(lat, lng float64) (*ReverseGeocodeResult, error) {
	q := url.Values{
		"latlng": {fmt.Sprintf("%f,%f", lat, lng)},
		"key":    {g.apiKey},
	}
	body, err := g.get("/geocode/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid reverse geocode response: %w", err)
	}
	if err := googleStatusErr("reverse_geocode", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("journey/google: no reverse geocode results for %f,%f", lat, lng)
	}
	r := resp.Results[0]
	out := &ReverseGeocodeResult{Address: r.FormattedAddress}
	for _, comp := range r.AddressComponents {
		for _, t := range comp.Types {
			switch t {
			case "route":
				out.Street = comp.LongName
			case "postal_town", "locality":
				if out.City == "" {
					out.City = comp.LongName
				}
			case "country":
				out.Country = comp.LongName
			case "postal_code":
				out.Postcode = comp.LongName
			}
		}
	}
	return out, nil
}

type googleDirectionsResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Routes       []struct {
		OverviewPolyline struct {
			Points string `json:"points"`
		} `json:"overview_polyline"`
		Bounds struct {
			Northeast LatLng `json:"northeast"`
			Southwest LatLng `json:"southwest"`
		} `json:"bounds"`
		WaypointOrder []int `json:"waypoint_order"`
		Legs          []struct {
			Distance googleValueText `json:"distance"`
			Duration googleValueText `json:"duration"`
			Steps    []struct {
				HTMLInstructions string          `json:"html_instructions"`
				Distance         googleValueText `json:"distance"`
				Duration         googleValueText `json:"duration"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}

type googleValueText struct {
	Value float64 `json:"value"`
	Text  string  `json:"text"`
}

func (g *googleProvider) CalculateRoute(p RouteParams) (*RouteResult, error) {
	q := url.Values{
		"origin":      {p.Origin},
		"destination": {p.Destination},
		"key":         {g.apiKey},
		"mode":        {googleMode(p.Mode)},
	}
	if len(p.Waypoints) > 0 {
		q.Set("waypoints", strings.Join(p.Waypoints, "|"))
	}
	if p.DepartAt != nil {
		q.Set("departure_time", strconv.FormatInt(p.DepartAt.Unix(), 10))
	}
	if len(p.Avoid) > 0 {
		q.Set("avoid", strings.Join(p.Avoid, "|"))
	}
	body, err := g.get("/directions/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleDirectionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid directions response: %w", err)
	}
	if err := googleStatusErr("directions", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Routes) == 0 {
		return nil, fmt.Errorf("journey/google: no routes returned")
	}
	r := resp.Routes[0]
	var totalDistance, totalDuration float64
	var steps []RouteStep
	for _, leg := range r.Legs {
		totalDistance += leg.Distance.Value
		totalDuration += leg.Duration.Value
		for _, s := range leg.Steps {
			steps = append(steps, RouteStep{
				Instruction:     stripHTML(s.HTMLInstructions),
				DistanceMetres:  s.Distance.Value,
				DurationSeconds: s.Duration.Value,
			})
		}
	}
	return &RouteResult{
		DistanceMetres:   totalDistance,
		DistanceMiles:    MetresToMiles(totalDistance),
		DurationSeconds:  totalDuration,
		DurationFriendly: FriendlyDuration(totalDuration),
		Polyline:         r.OverviewPolyline.Points,
		Steps:            steps,
		Bounds: RouteBounds{
			Northeast: r.Bounds.Northeast,
			Southwest: r.Bounds.Southwest,
		},
	}, nil
}

type googleDistanceMatrixResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Rows         []struct {
		Elements []struct {
			Status   string          `json:"status"`
			Distance googleValueText `json:"distance"`
			Duration googleValueText `json:"duration"`
		} `json:"elements"`
	} `json:"rows"`
}

func (g *googleProvider) GetTravelTime(origin, destination, mode string, departAt *time.Time) (*TravelTimeResult, error) {
	q := url.Values{
		"origins":      {origin},
		"destinations": {destination},
		"key":          {g.apiKey},
		"mode":         {googleMode(mode)},
	}
	if departAt != nil {
		q.Set("departure_time", strconv.FormatInt(departAt.Unix(), 10))
	}
	body, err := g.get("/distancematrix/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleDistanceMatrixResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid distance matrix response: %w", err)
	}
	if err := googleStatusErr("distance_matrix", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Rows) == 0 || len(resp.Rows[0].Elements) == 0 {
		return nil, fmt.Errorf("journey/google: empty distance matrix")
	}
	el := resp.Rows[0].Elements[0]
	if el.Status != "OK" {
		return nil, fmt.Errorf("journey/google: distance matrix element status %s", el.Status)
	}
	return &TravelTimeResult{
		DistanceMetres:   el.Distance.Value,
		DistanceMiles:    MetresToMiles(el.Distance.Value),
		DurationSeconds:  el.Duration.Value,
		DurationFriendly: FriendlyDuration(el.Duration.Value),
	}, nil
}

func (g *googleProvider) get(path string, query url.Values) ([]byte, error) {
	u := googleBaseURL + path + "?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("journey/google: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("journey/google: read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("journey/google: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func googleStatusErr(op, status, msg string) error {
	if status == "OK" || status == "ZERO_RESULTS" {
		return nil
	}
	if msg == "" {
		return fmt.Errorf("journey/google: %s status %s", op, status)
	}
	return fmt.Errorf("journey/google: %s status %s: %s", op, status, msg)
}

func googleMode(mode string) string {
	switch strings.ToLower(mode) {
	case ModeWalking:
		return "walking"
	case ModeCycling:
		return "bicycling"
	case ModeTransit:
		return "transit"
	default:
		return "driving"
	}
}

// googleConfidence maps Google's location_type values onto our three buckets.
// ROOFTOP / RANGE_INTERPOLATED indicate the geocoder found a precise point;
// GEOMETRIC_CENTER is an approximation; APPROXIMATE is a region centroid.
func googleConfidence(locationType string, partial bool) string {
	if partial {
		return "low"
	}
	switch locationType {
	case "ROOFTOP", "RANGE_INTERPOLATED":
		return "high"
	case "GEOMETRIC_CENTER":
		return "medium"
	default:
		return "low"
	}
}

// OptimiseRoute calls Google's Directions API with `waypoints=optimize:true|...`,
// which returns waypoint_order indicating the optimal visit order. When Start
// or End are blank we use the first stop as both anchors (cycle) — Google
// requires explicit origin/destination, so a "tour" of stops is encoded as
// stops[0] → optimised middle → stops[0].
func (g *googleProvider) OptimiseRoute(p OptimiseParams) (*OptimiseResult, error) {
	if len(p.Stops) == 0 {
		return nil, fmt.Errorf("journey/google: optimise_route requires at least one stop")
	}

	origin := p.Start
	destination := p.End
	middle := p.Stops
	cycle := origin == "" && destination == ""
	if cycle {
		origin = p.Stops[0]
		destination = p.Stops[0]
		middle = p.Stops[1:]
	} else if origin == "" {
		origin = p.Stops[0]
		middle = p.Stops[1:]
	} else if destination == "" {
		destination = p.Stops[len(p.Stops)-1]
		middle = p.Stops[:len(p.Stops)-1]
	}

	q := url.Values{
		"origin":      {origin},
		"destination": {destination},
		"key":         {g.apiKey},
		"mode":        {googleMode(p.Mode)},
	}
	if len(middle) > 0 {
		q.Set("waypoints", "optimize:true|"+strings.Join(middle, "|"))
	}
	if p.DepartAt != nil {
		q.Set("departure_time", strconv.FormatInt(p.DepartAt.Unix(), 10))
	}
	body, err := g.get("/directions/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleDirectionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid optimisation response: %w", err)
	}
	if err := googleStatusErr("optimise_route", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Routes) == 0 {
		return nil, fmt.Errorf("journey/google: no routes returned for optimisation")
	}
	r := resp.Routes[0]

	ordered := make([]string, 0, len(middle))
	for _, idx := range r.WaypointOrder {
		if idx >= 0 && idx < len(middle) {
			ordered = append(ordered, middle[idx])
		}
	}

	// Build per-leg breakdown — each leg connects two consecutive points in
	// the full path: origin → ordered[0] → ordered[1] → … → destination.
	path := append([]string{origin}, ordered...)
	path = append(path, destination)
	legs := make([]OptimiseLeg, 0, len(r.Legs))
	var totalDist, totalDur float64
	for i, leg := range r.Legs {
		from, to := "", ""
		if i < len(path) {
			from = path[i]
		}
		if i+1 < len(path) {
			to = path[i+1]
		}
		legs = append(legs, OptimiseLeg{
			From:            from,
			To:              to,
			DistanceMetres:  leg.Distance.Value,
			DistanceMiles:   MetresToMiles(leg.Distance.Value),
			DurationSeconds: leg.Duration.Value,
		})
		totalDist += leg.Distance.Value
		totalDur += leg.Duration.Value
	}

	return &OptimiseResult{
		OrderedStops:          ordered,
		WaypointOrder:         r.WaypointOrder,
		TotalDistanceMetres:   totalDist,
		TotalDistanceMiles:    MetresToMiles(totalDist),
		TotalDurationSeconds:  totalDur,
		TotalDurationFriendly: FriendlyDuration(totalDur),
		Legs:                  legs,
	}, nil
}

// RenderStaticMap builds a Google Static Maps URL and downloads the image.
// The Static Maps API auto-fits the viewport to the path bounds when no
// center/zoom is given, which matches our "Zoom 0 = auto-fit" contract.
//
// We request blue 4px path drawn with the route polyline (Google's `enc:`
// prefix tells the API the path is encoded rather than a list of lat/lngs).
// Each marker becomes a `markers=` query parameter — Google accepts repeated
// keys and renders them in order.
func (g *googleProvider) RenderStaticMap(p StaticMapParams) ([]byte, string, error) {
	if strings.TrimSpace(p.Polyline) == "" {
		return nil, "", fmt.Errorf("journey/google: render_static_map requires a polyline")
	}
	width, height := p.Width, p.Height
	if width <= 0 {
		width = 600
	}
	if height <= 0 {
		height = 400
	}

	params := url.Values{
		"size": {fmt.Sprintf("%dx%d", width, height)},
		"path": {fmt.Sprintf("color:0x0000ffff|weight:4|enc:%s", p.Polyline)},
		"key":  {g.apiKey},
	}
	if p.Zoom > 0 {
		params.Set("zoom", strconv.Itoa(p.Zoom))
	}
	for _, m := range p.Markers {
		spec := ""
		if m.Color != "" {
			spec += "color:" + m.Color + "|"
		}
		if m.Label != "" {
			spec += "label:" + m.Label + "|"
		}
		spec += fmt.Sprintf("%f,%f", m.Lat, m.Lng)
		params.Add("markers", spec)
	}

	u := googleBaseURL + "/staticmap?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("journey/google: static map request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("journey/google: read static map body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("journey/google: static map http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	return body, mime, nil
}

type googleNearbyResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Results      []struct {
		PlaceID          string `json:"place_id"`
		Name             string `json:"name"`
		Vicinity         string `json:"vicinity"`
		Geometry         googleGeocodeGeometry
		Types            []string `json:"types"`
		Rating           float64  `json:"rating"`
		UserRatingsTotal int      `json:"user_ratings_total"`
		PriceLevel       int      `json:"price_level"`
		OpeningHours     *struct {
			OpenNow bool `json:"open_now"`
		} `json:"opening_hours,omitempty"`
	} `json:"results"`
}

func (g *googleProvider) FindNearbyPlaces(p NearbyPlacesParams) ([]Place, error) {
	if p.Radius <= 0 {
		p.Radius = 1500
	}
	q := url.Values{
		"location": {fmt.Sprintf("%f,%f", p.Latitude, p.Longitude)},
		"radius":   {strconv.Itoa(p.Radius)},
		"key":      {g.apiKey},
	}
	if p.Keyword != "" {
		q.Set("keyword", p.Keyword)
	}
	if p.Category != "" {
		q.Set("type", p.Category)
	}
	body, err := g.get("/place/nearbysearch/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleNearbyResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid nearby response: %w", err)
	}
	if err := googleStatusErr("nearby_places", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}

	origin := LatLng{Lat: p.Latitude, Lng: p.Longitude}
	out := make([]Place, 0, len(resp.Results))
	for i, r := range resp.Results {
		if p.Limit > 0 && i >= p.Limit {
			break
		}
		place := Place{
			PlaceID:     r.PlaceID,
			Name:        r.Name,
			Address:     r.Vicinity,
			Latitude:    r.Geometry.Location.Lat,
			Longitude:   r.Geometry.Location.Lng,
			Rating:      r.Rating,
			UserRatings: r.UserRatingsTotal,
			PriceLevel:  r.PriceLevel,
			Types:       r.Types,
			DistanceM:   haversineMetres(origin, r.Geometry.Location),
		}
		if r.OpeningHours != nil {
			openNow := r.OpeningHours.OpenNow
			place.OpenNow = &openNow
		}
		out = append(out, place)
	}
	return out, nil
}

type googlePlaceDetailsResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Result       struct {
		PlaceID              string `json:"place_id"`
		Name                 string `json:"name"`
		FormattedAddress     string `json:"formatted_address"`
		Geometry             googleGeocodeGeometry
		FormattedPhoneNumber string   `json:"formatted_phone_number"`
		Website              string   `json:"website"`
		URL                  string   `json:"url"`
		Rating               float64  `json:"rating"`
		UserRatingsTotal     int      `json:"user_ratings_total"`
		PriceLevel           int      `json:"price_level"`
		Types                []string `json:"types"`
		OpeningHours         *struct {
			WeekdayText []string `json:"weekday_text"`
		} `json:"opening_hours,omitempty"`
	} `json:"result"`
}

func (g *googleProvider) GetPlaceDetails(placeID string) (*PlaceDetails, error) {
	if strings.TrimSpace(placeID) == "" {
		return nil, fmt.Errorf("journey/google: place_id is required")
	}
	q := url.Values{
		"place_id": {placeID},
		"key":      {g.apiKey},
		"fields":   {"place_id,name,formatted_address,geometry,formatted_phone_number,website,url,rating,user_ratings_total,price_level,types,opening_hours"},
	}
	body, err := g.get("/place/details/json", q)
	if err != nil {
		return nil, err
	}
	var resp googlePlaceDetailsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid details response: %w", err)
	}
	if err := googleStatusErr("place_details", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	r := resp.Result
	out := &PlaceDetails{
		PlaceID:     r.PlaceID,
		Name:        r.Name,
		Address:     r.FormattedAddress,
		Latitude:    r.Geometry.Location.Lat,
		Longitude:   r.Geometry.Location.Lng,
		Phone:       r.FormattedPhoneNumber,
		Website:     r.Website,
		Rating:      r.Rating,
		UserRatings: r.UserRatingsTotal,
		PriceLevel:  r.PriceLevel,
		Types:       r.Types,
		GoogleURL:   r.URL,
	}
	if r.OpeningHours != nil {
		out.OpeningHours = r.OpeningHours.WeekdayText
	}
	return out, nil
}

type googleElevationResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Results      []struct {
		Elevation  float64 `json:"elevation"`
		Resolution float64 `json:"resolution"`
		Location   LatLng  `json:"location"`
	} `json:"results"`
}

// GetElevationProfile uses Google's Elevation API. Up to 512 samples per
// request — anything higher is clamped. Returns ascent (sum of positive
// deltas), descent (sum of negative deltas), min/max elevation, and the
// raw samples for downstream graphing.
func (g *googleProvider) GetElevationProfile(p ElevationParams) (*ElevationResult, error) {
	if strings.TrimSpace(p.Polyline) == "" {
		return nil, fmt.Errorf("journey/google: elevation_profile requires a polyline")
	}
	samples := p.SampleCount
	if samples <= 0 {
		samples = 100
	}
	if samples > 512 {
		samples = 512
	}
	q := url.Values{
		"path":    {"enc:" + p.Polyline},
		"samples": {strconv.Itoa(samples)},
		"key":     {g.apiKey},
	}
	body, err := g.get("/elevation/json", q)
	if err != nil {
		return nil, err
	}
	var resp googleElevationResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("journey/google: invalid elevation response: %w", err)
	}
	if err := googleStatusErr("elevation", resp.Status, resp.ErrorMessage); err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("journey/google: no elevation samples returned")
	}

	out := &ElevationResult{
		Samples:            make([]ElevationSample, 0, len(resp.Results)),
		MinElevationMetres: resp.Results[0].Elevation,
		MaxElevationMetres: resp.Results[0].Elevation,
	}
	for i, r := range resp.Results {
		out.Samples = append(out.Samples, ElevationSample{
			Latitude:   r.Location.Lat,
			Longitude:  r.Location.Lng,
			Elevation:  r.Elevation,
			Resolution: r.Resolution,
		})
		if r.Elevation < out.MinElevationMetres {
			out.MinElevationMetres = r.Elevation
		}
		if r.Elevation > out.MaxElevationMetres {
			out.MaxElevationMetres = r.Elevation
		}
		if i > 0 {
			d := r.Elevation - resp.Results[i-1].Elevation
			if d > 0 {
				out.TotalAscentMetres += d
			} else {
				out.TotalDescentMetres += -d
			}
		}
	}
	return out, nil
}

// haversineMetres approximates the great-circle distance between two
// points. Accurate to within ~0.5% for distances under 1000km, which more
// than covers the "how far is this restaurant from me" use case.
func haversineMetres(a, b LatLng) float64 {
	const earthRadiusMetres = 6371000.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	lat1 := toRad(a.Lat)
	lat2 := toRad(b.Lat)
	dLat := lat2 - lat1
	dLng := toRad(b.Lng - a.Lng)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusMetres * 2 * math.Asin(math.Sqrt(h))
}

func stripHTML(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
