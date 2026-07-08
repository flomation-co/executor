package helpdesk_intercom_article_search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

func searchInputs(extra ...*core.Connection) []*core.Connection {
	inputs := []*core.Connection{
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "phrase", Type: core.ConnectionTypeString, Value: "getting started"},
	}
	return append(inputs, extra...)
}

func articles(n int, prefix string) []map[string]interface{} {
	out := make([]map[string]interface{}, n)
	for i := range out {
		out[i] = map[string]interface{}{"id": fmt.Sprintf("%s-%d", prefix, i), "title": "A"}
	}
	return out
}

// A page larger than the requested limit must be trimmed client-side — the
// endpoint is not trusted to honor per_page.
func TestSearchTrimsOversizedPageToLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"articles": articles(5, "a")},
		})
	}))
	defer srv.Close()
	defer intercom.SetBaseForTest(srv.URL)()

	out, err := Execute(nil, nil, searchInputs(
		&core.Connection{Name: "limit", Type: core.ConnectionTypeString, Value: "2"},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if got := out["count"]; got != 2 {
		t.Fatalf("count = %v, want 2 (trimmed)", got)
	}
}

// Return All follows the starting_after cursor across pages and keeps
// everything.
func TestSearchReturnAllFollowsCursor(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := map[string]interface{}{
			"data": map[string]interface{}{"articles": articles(3, fmt.Sprintf("p%d", calls))},
		}
		if calls == 1 {
			page["pages"] = map[string]interface{}{"next": map[string]interface{}{"starting_after": "cur2"}}
		}
		if calls == 2 && r.URL.Query().Get("starting_after") != "cur2" {
			t.Errorf("second page missing cursor, query=%q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	defer srv.Close()
	defer intercom.SetBaseForTest(srv.URL)()

	out, err := Execute(nil, nil, searchInputs(
		&core.Connection{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true},
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if got := out["count"]; got != 6 {
		t.Fatalf("count = %v, want 6 (no trim on Return All)", got)
	}
}
