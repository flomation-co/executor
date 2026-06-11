package journey_common

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

const googleOptimiseOK = `{
  "status": "OK",
  "routes": [{
    "waypoint_order": [2, 0, 1],
    "overview_polyline": {"points": "encoded"},
    "bounds": {"northeast": {"lat": 53.5, "lng": -2.2}, "southwest": {"lat": 51.5, "lng": -2.3}},
    "legs": [
      {"distance": {"value": 80000}, "duration": {"value": 3600}, "steps": []},
      {"distance": {"value": 100000}, "duration": {"value": 4800}, "steps": []},
      {"distance": {"value": 90000}, "duration": {"value": 4200}, "steps": []},
      {"distance": {"value": 60000}, "duration": {"value": 3000}, "steps": []}
    ]
  }]
}`

func TestGoogleOptimiseRoute(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleOptimiseOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	res, err := p.OptimiseRoute(OptimiseParams{
		Start: "London",
		End:   "Manchester",
		Stops: []string{"Birmingham", "Coventry", "Stoke"},
		Mode:  ModeDriving,
	})
	Expect(err).ToNot(HaveOccurred())
	// waypoint_order [2, 0, 1] means: visit stops[2], stops[0], stops[1]
	Expect(res.OrderedStops).To(Equal([]string{"Stoke", "Birmingham", "Coventry"}))
	Expect(res.WaypointOrder).To(Equal([]int{2, 0, 1}))
	Expect(res.TotalDistanceMetres).To(Equal(330000.0))
	Expect(res.Legs).To(HaveLen(4))
	Expect(res.Legs[0].From).To(Equal("London"))
	Expect(res.Legs[0].To).To(Equal("Stoke"))
	Expect(res.Legs[3].To).To(Equal("Manchester"))

	// Confirm Google was asked with optimize:true prefix
	q := tr.last.URL.Query().Get("waypoints")
	Expect(strings.HasPrefix(q, "optimize:true|")).To(BeTrue())
}

func TestGoogleOptimiseRouteCycleNoAnchors(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleOptimiseOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.OptimiseRoute(OptimiseParams{
		Stops: []string{"A", "B", "C", "D"},
		Mode:  ModeDriving,
	})
	Expect(err).ToNot(HaveOccurred())

	// When no anchors set, first stop is used as origin AND destination
	Expect(tr.last.URL.Query().Get("origin")).To(Equal("A"))
	Expect(tr.last.URL.Query().Get("destination")).To(Equal("A"))
}

func TestGoogleOptimiseRouteEmptyStops(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, googleOptimiseOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.OptimiseRoute(OptimiseParams{
		Start: "X",
		End:   "Y",
		Stops: nil,
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("at least one stop"))
}

const mapboxOptimiseOK = `{
  "code": "Ok",
  "trips": [{
    "distance": 330000,
    "duration": 15600,
    "legs": [
      {"distance": 80000, "duration": 3600},
      {"distance": 100000, "duration": 4800},
      {"distance": 90000, "duration": 4200},
      {"distance": 60000, "duration": 3000}
    ]
  }],
  "waypoints": [
    {"waypoint_index": 0, "trips_index": 0, "location": [-0.12, 51.5], "name": "London"},
    {"waypoint_index": 2, "trips_index": 0, "location": [-1.5, 52.4], "name": "Birmingham"},
    {"waypoint_index": 3, "trips_index": 0, "location": [-1.5, 52.4], "name": "Coventry"},
    {"waypoint_index": 1, "trips_index": 0, "location": [-2.18, 53.0], "name": "Stoke"},
    {"waypoint_index": 4, "trips_index": 0, "location": [-2.24, 53.48], "name": "Manchester"}
  ]
}`

func TestMapboxOptimiseRoute(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, mapboxOptimiseOK)
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	res, err := p.OptimiseRoute(OptimiseParams{
		Start: "51.5,-0.12",
		End:   "53.48,-2.24",
		Stops: []string{"52.4,-1.5", "53.0,-2.18", "52.4,-1.5"},
		Mode:  ModeDriving,
	})
	Expect(err).ToNot(HaveOccurred())
	// waypoint_index says: input 0 visited 1st, input 3 visited 2nd, input 1
	// visited 3rd, input 2 visited 4th, input 4 visited 5th
	// Inputs are [start, ...stops, end] = [start, stops[0], stops[1], stops[2], end]
	// Visit order maps to input positions: [0, 3, 1, 2, 4] (input idx in
	// visit position)
	// Stripped of start/end (positions 0 and 4): stops indices [2, 0, 1]
	Expect(res.WaypointOrder).To(Equal([]int{2, 0, 1}))
	Expect(res.TotalDistanceMetres).To(Equal(330000.0))

	Expect(strings.Contains(tr.last.URL.Path, "/optimized-trips/v1/mapbox/driving-traffic/")).To(BeTrue())
	Expect(tr.last.URL.Query().Get("source")).To(Equal("first"))
	Expect(tr.last.URL.Query().Get("destination")).To(Equal("last"))
}

const mapboxOptimiseCycleOK = `{
  "code": "Ok",
  "trips": [{
    "distance": 400000,
    "duration": 20000,
    "legs": [
      {"distance": 100000, "duration": 5000},
      {"distance": 100000, "duration": 5000},
      {"distance": 100000, "duration": 5000},
      {"distance": 100000, "duration": 5000}
    ]
  }],
  "waypoints": [
    {"waypoint_index": 0, "trips_index": 0, "location": [-0.12, 51.5], "name": "A"},
    {"waypoint_index": 1, "trips_index": 0, "location": [-1.5, 52.4], "name": "B"},
    {"waypoint_index": 2, "trips_index": 0, "location": [-2.18, 53.0], "name": "C"},
    {"waypoint_index": 3, "trips_index": 0, "location": [-2.24, 53.48], "name": "D"}
  ]
}`

func TestMapboxOptimiseRouteCycle(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, mapboxOptimiseCycleOK)
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	_, err := p.OptimiseRoute(OptimiseParams{
		Stops: []string{"51.5,-0.12", "52.4,-1.5", "53.0,-2.18", "53.48,-2.24"},
		Mode:  ModeDriving,
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(tr.last.URL.Query().Get("roundtrip")).To(Equal("true"))
}
