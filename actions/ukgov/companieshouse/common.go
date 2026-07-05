package companieshouse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the Companies House Public Data API root. Package variable so
// tests can point it at a mock server.
var BaseURL = "https://api.company-information.service.gov.uk"

// Note: the api_key secret input is inlined into each action's Inputs literal
// rather than shared from here — the manifest generator only resolves literal
// composite literals, so a shared var reference would be skipped (see the AWS
// actions, which inline their secret inputs for the same reason).

// Get performs an authenticated GET against the Companies House API. The API
// key is supplied as the Basic auth username with a blank password.
func Get(ctx context.Context, apiKey, path string, query url.Values) (int, []byte, error) {
	endpoint := BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return ukgov_common.Fetch(ctx, http.MethodGet, endpoint, map[string]string{
		"Authorization": ukgov_common.BasicAuthHeader(apiKey, ""),
	})
}

// StatusMessage maps a non-success Companies House HTTP status to a friendly,
// AI-readable message. 404 is handled per-action (its meaning is contextual).
func StatusMessage(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "Companies House authentication failed — check the API key."
	case http.StatusTooManyRequests:
		return "Companies House rate limit exceeded (600 requests per 5 minutes). Try again shortly."
	default:
		return fmt.Sprintf("Companies House returned status %d.", status)
	}
}

// Address is the registered-office / correspondence address shape reused across
// company profiles, officers and PSCs.
type Address struct {
	Premises     string `json:"premises"`
	AddressLine1 string `json:"address_line_1"`
	AddressLine2 string `json:"address_line_2"`
	Locality     string `json:"locality"`
	Region       string `json:"region"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
}

// OneLine renders the address as a single comma-separated string, skipping
// empty components.
func (a Address) OneLine() string {
	parts := make([]string, 0, 7)
	for _, p := range []string{a.Premises, a.AddressLine1, a.AddressLine2, a.Locality, a.Region, a.PostalCode, a.Country} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}
