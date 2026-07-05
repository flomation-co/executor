package foodstandards

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the Food Standards Agency FHRS API root. It is a package variable
// (not a const) so tests can redirect it to a local mock server.
var BaseURL = "https://api.ratings.food.gov.uk"

// apiVersion is the mandatory FHRS API version header value.
const apiVersion = "2"

// Get performs a GET against the FHRS API, adding the required version header.
// The FHRS API requires no authentication.
func Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	endpoint := BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return ukgov_common.Fetch(ctx, http.MethodGet, endpoint, map[string]string{
		"x-api-version": apiVersion,
	})
}

// Geocode is an establishment's map coordinates (FHRS returns them as strings).
type Geocode struct {
	Longitude string `json:"longitude"`
	Latitude  string `json:"latitude"`
}

// Establishment is a single food business as returned by the FHRS API.
type Establishment struct {
	FHRSID             int64   `json:"FHRSID"`
	BusinessName       string  `json:"BusinessName"`
	BusinessType       string  `json:"BusinessType"`
	AddressLine1       string  `json:"AddressLine1"`
	AddressLine2       string  `json:"AddressLine2"`
	AddressLine3       string  `json:"AddressLine3"`
	AddressLine4       string  `json:"AddressLine4"`
	PostCode           string  `json:"PostCode"`
	RatingValue        string  `json:"RatingValue"`
	RatingDate         string  `json:"RatingDate"`
	LocalAuthorityName string  `json:"LocalAuthorityName"`
	Geocode            Geocode `json:"geocode"`
}

// Address joins the establishment's address lines and postcode, skipping empty
// components, into a single human-readable string.
func (e Establishment) Address() string {
	parts := make([]string, 0, 5)
	for _, p := range []string{e.AddressLine1, e.AddressLine2, e.AddressLine3, e.AddressLine4, e.PostCode} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}
