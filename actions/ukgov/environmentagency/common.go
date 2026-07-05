package environmentagency

import (
	"context"
	"net/http"
	"net/url"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the Environment Agency Real Time Flood Monitoring API root.
// Package variable so tests can point it at a mock server. No authentication
// required (Open Government Licence).
var BaseURL = "https://environment.data.gov.uk/flood-monitoring"

// Get performs a GET against the flood-monitoring API. If path already contains
// a query string, pass a nil query and it will be used verbatim.
func Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	endpoint := BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
}
