package police

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	ukgov_common "flomation.app/automate/executor/actions/ukgov"
)

// BaseURL is the data.police.uk API root. Package variable so tests can point
// it at a mock server. The Police API requires no authentication.
var BaseURL = "https://data.police.uk/api"

// Get performs a GET against the Police API.
func Get(ctx context.Context, path string, query url.Values) (int, []byte, error) {
	endpoint := BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return ukgov_common.Fetch(ctx, http.MethodGet, endpoint, nil)
}

// Street is the nearest street to a crime or stop (an approximation — the
// Police API deliberately anonymises exact locations).
type Street struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Location is an anonymised crime/stop location.
type Location struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Street    Street `json:"street"`
}

// OutcomeStatus is the latest recorded outcome of a crime.
type OutcomeStatus struct {
	Category string `json:"category"`
	Date     string `json:"date"`
}

// Crime is a single street-level crime record.
type Crime struct {
	Category      string         `json:"category"`
	Location      *Location      `json:"location"`
	Month         string         `json:"month"`
	OutcomeStatus *OutcomeStatus `json:"outcome_status"`
	PersistentID  string         `json:"persistent_id"`
	ID            int64          `json:"id"`
}

// Stop is a single stop-and-search record. Outcome is interface{} because the
// Police API returns it as either a descriptive string or a boolean false.
type Stop struct {
	Type           string      `json:"type"`
	Datetime       string      `json:"datetime"`
	ObjectOfSearch string      `json:"object_of_search"`
	Outcome        interface{} `json:"outcome"`
	Gender         string      `json:"gender"`
	AgeRange       string      `json:"age_range"`
	Legislation    string      `json:"legislation"`
	Location       *Location   `json:"location"`
}

// TopCounts renders the top n keys by descending count (ties broken
// alphabetically for determinism) as "key (count), key (count)".
func TopCounts(counts map[string]int, n int) string {
	type kv struct {
		key string
		val int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].val != pairs[j].val {
			return pairs[i].val > pairs[j].val
		}
		return pairs[i].key < pairs[j].key
	})
	if n > len(pairs) {
		n = len(pairs)
	}
	parts := make([]string, 0, n)
	for _, p := range pairs[:n] {
		parts = append(parts, fmt.Sprintf("%s (%d)", p.key, p.val))
	}
	return strings.Join(parts, ", ")
}
