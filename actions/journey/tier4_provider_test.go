package journey_common

import (
	"testing"

	. "github.com/onsi/gomega"
)

const googleNearbyOK = `{
  "status": "OK",
  "results": [
    {
      "place_id": "ChIJrTLr-GyuEmsRBfy61i59si0",
      "name": "Cafe One",
      "vicinity": "10 Test St, London",
      "geometry": {"location": {"lat": 51.5040, "lng": -0.1280}},
      "types": ["cafe", "food"],
      "rating": 4.6,
      "user_ratings_total": 250,
      "price_level": 2,
      "opening_hours": {"open_now": true}
    },
    {
      "place_id": "ChIJabc",
      "name": "Cafe Two",
      "vicinity": "20 Test St, London",
      "geometry": {"location": {"lat": 51.5050, "lng": -0.1290}},
      "types": ["cafe"],
      "rating": 4.2,
      "user_ratings_total": 100
    }
  ]
}`

func TestGoogleFindNearbyPlaces(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleNearbyOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	places, err := p.FindNearbyPlaces(NearbyPlacesParams{
		Latitude:  51.5034,
		Longitude: -0.1276,
		Radius:    1000,
		Category:  "cafe",
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(places).To(HaveLen(2))
	Expect(places[0].Name).To(Equal("Cafe One"))
	Expect(places[0].Rating).To(Equal(4.6))
	Expect(places[0].OpenNow).ToNot(BeNil())
	Expect(*places[0].OpenNow).To(BeTrue())
	// Haversine distance from (51.5034,-0.1276) to (51.5040,-0.1280) is ~75m
	Expect(places[0].DistanceM).To(BeNumerically(">", 50))
	Expect(places[0].DistanceM).To(BeNumerically("<", 150))

	q := tr.last.URL.Query()
	Expect(q.Get("location")).To(ContainSubstring("51.5034"))
	Expect(q.Get("type")).To(Equal("cafe"))
	Expect(q.Get("radius")).To(Equal("1000"))
}

func TestGoogleFindNearbyPlacesLimit(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, googleNearbyOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	places, err := p.FindNearbyPlaces(NearbyPlacesParams{
		Latitude: 51.5, Longitude: -0.1, Limit: 1,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(places).To(HaveLen(1))
}

const googlePlaceDetailsOK = `{
  "status": "OK",
  "result": {
    "place_id": "ChIJ123",
    "name": "Test Restaurant",
    "formatted_address": "1 Test Lane, London EC1A 1AA",
    "geometry": {"location": {"lat": 51.515, "lng": -0.072}},
    "formatted_phone_number": "020 1234 5678",
    "website": "https://test.example.com",
    "url": "https://maps.google.com/?cid=123",
    "rating": 4.5,
    "user_ratings_total": 500,
    "price_level": 3,
    "types": ["restaurant", "food"],
    "opening_hours": {
      "weekday_text": [
        "Monday: 12:00 PM – 11:00 PM",
        "Tuesday: 12:00 PM – 11:00 PM"
      ]
    }
  }
}`

func TestGoogleGetPlaceDetails(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googlePlaceDetailsOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	details, err := p.GetPlaceDetails("ChIJ123")
	Expect(err).ToNot(HaveOccurred())
	Expect(details.Name).To(Equal("Test Restaurant"))
	Expect(details.Phone).To(Equal("020 1234 5678"))
	Expect(details.Website).To(Equal("https://test.example.com"))
	Expect(details.Rating).To(Equal(4.5))
	Expect(details.OpeningHours).To(HaveLen(2))
	Expect(details.GoogleURL).To(ContainSubstring("maps.google.com"))

	Expect(tr.last.URL.Query().Get("place_id")).To(Equal("ChIJ123"))
}

func TestGoogleGetPlaceDetailsMissingID(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, googlePlaceDetailsOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.GetPlaceDetails("  ")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("place_id is required"))
}

const googleElevationOK = `{
  "status": "OK",
  "results": [
    {"elevation": 100.0, "resolution": 5.0, "location": {"lat": 51.5, "lng": -0.1}},
    {"elevation": 150.0, "resolution": 5.0, "location": {"lat": 51.6, "lng": -0.1}},
    {"elevation": 120.0, "resolution": 5.0, "location": {"lat": 51.7, "lng": -0.1}},
    {"elevation": 200.0, "resolution": 5.0, "location": {"lat": 51.8, "lng": -0.1}}
  ]
}`

func TestGoogleGetElevationProfile(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleElevationOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	res, err := p.GetElevationProfile(ElevationParams{
		Polyline:    "_p~iF~ps|U_ulLnnqC",
		SampleCount: 4,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(res.Samples).To(HaveLen(4))
	// Deltas: +50, -30, +80 → ascent = 130, descent = 30
	Expect(res.TotalAscentMetres).To(Equal(130.0))
	Expect(res.TotalDescentMetres).To(Equal(30.0))
	Expect(res.MinElevationMetres).To(Equal(100.0))
	Expect(res.MaxElevationMetres).To(Equal(200.0))

	q := tr.last.URL.Query()
	Expect(q.Get("samples")).To(Equal("4"))
	Expect(q.Get("path")).To(ContainSubstring("enc:_p~iF~ps|U"))
}

func TestGoogleElevationClampsToMax(t *testing.T) {
	RegisterTestingT(t)

	client, tr := stubClient(200, googleElevationOK)
	p, _ := NewProviderWithClient("google", "test-key", client)

	_, err := p.GetElevationProfile(ElevationParams{
		Polyline:    "_p~iF~ps|U",
		SampleCount: 9999,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(tr.last.URL.Query().Get("samples")).To(Equal("512"))
}

func TestMapboxTier4NotSupported(t *testing.T) {
	RegisterTestingT(t)

	client, _ := stubClient(200, "")
	p, _ := NewProviderWithClient("mapbox", "test-key", client)

	_, err := p.FindNearbyPlaces(NearbyPlacesParams{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not supported"))
	Expect(err.Error()).To(ContainSubstring("use provider=google"))

	_, err = p.GetPlaceDetails("anything")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not supported"))

	_, err = p.GetElevationProfile(ElevationParams{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not supported"))
}

func TestHaversineMetres(t *testing.T) {
	RegisterTestingT(t)

	// London to Manchester is roughly 262 km great-circle.
	london := LatLng{Lat: 51.5074, Lng: -0.1278}
	manchester := LatLng{Lat: 53.4808, Lng: -2.2426}
	dist := haversineMetres(london, manchester)
	Expect(dist).To(BeNumerically("~", 262000, 5000))

	// Same point → 0
	Expect(haversineMetres(london, london)).To(BeNumerically("~", 0, 1))
}
