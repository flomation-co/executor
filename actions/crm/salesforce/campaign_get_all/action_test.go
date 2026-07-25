package crm_salesforce_campaign_get_all

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

type queryRecorder struct {
	soql string
}

func (q *queryRecorder) serve(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sobjects/Campaign/describe"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name": "Campaign",
				"fields": []map[string]interface{}{
					{"name": "IsActive", "type": "boolean"},
					{"name": "Status", "type": "picklist"},
					{"name": "Type", "type": "picklist"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/query"):
			q.soql = r.URL.Query().Get("q")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 0, "done": true, "records": []interface{}{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

func auth() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
	}
}

// "Active Campaigns Only" is the same shape of unconditional promise as Get
// Many Users' tick box, and had the same bug: the ANY toggle ORed it away, so
// the action returned every active campaign plus every switched-off campaign
// matching any other filter. Status and Campaign Type are ordinary filters and
// stay under the toggle — "Planned OR Webinar" is a coherent request.
func TestMatchAnyFilterDoesNotOrActiveCampaignsOnly(t *testing.T) {
	rec := &queryRecorder{}
	defer rec.serve(t)()

	inputs := append(auth(),
		&core.Connection{Name: "active_only", Type: core.ConnectionTypeBoolean, Value: true},
		&core.Connection{Name: "campaign_status", Type: core.ConnectionTypeString, Value: "In Progress"},
		&core.Connection{Name: "campaign_type", Type: core.ConnectionTypeString, Value: "Webinar"},
		&core.Connection{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Value: true},
	)

	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}

	want := "WHERE IsActive = true AND (Status = 'In Progress' OR Type = 'Webinar')"
	if !strings.Contains(rec.soql, want) {
		t.Errorf("Active Campaigns Only must stay ANDed:\n got %s\nwant ...%s...", rec.soql, want)
	}
	if strings.Contains(rec.soql, "IsActive = true OR") {
		t.Errorf("Active Campaigns Only is still being ORed away, got:\n%s", rec.soql)
	}
}
