package postcodes

import (
	"context"
	"net/http"
	"net/url"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the postcodes.io API root. Package variable so tests can point it
// at a mock server. postcodes.io requires no authentication.
var BaseURL = "https://api.postcodes.io"

// Get performs a GET against the postcodes.io API.
func Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	endpoint := BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
}

// Postcode is a single postcodes.io result. Longitude/Latitude come back null
// for terminated postcodes, decoding to 0 — callers relying on coordinates
// should sanity-check. Distance is only populated by the nearest endpoint.
type Postcode struct {
	Postcode                  string  `json:"postcode"`
	Longitude                 float64 `json:"longitude"`
	Latitude                  float64 `json:"latitude"`
	Country                   string  `json:"country"`
	Region                    string  `json:"region"`
	AdminDistrict             string  `json:"admin_district"`
	AdminWard                 string  `json:"admin_ward"`
	ParliamentaryConstituency string  `json:"parliamentary_constituency"`
	Distance                  float64 `json:"distance,omitempty"`
}
