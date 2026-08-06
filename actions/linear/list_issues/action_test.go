package linear_list_issues

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
	. "github.com/onsi/gomega"
)

func conns(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// Filtering by project NAME must resolve the name to a UUID and apply it as the
// issue filter, and the returned issues (and summary) must carry the project so
// a caller can tell which project each issue is in.
func TestExecute_FiltersByProjectNameAndExposesProject(t *testing.T) {
	RegisterTestingT(t)

	var listVars map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.Unmarshal(raw, &req)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(req.Query, "projects(first: 250)"):
			// ResolveProjectID name lookup.
			_, _ = w.Write([]byte(`{"data":{"projects":{"nodes":[
				{"id":"proj-sales-uuid","name":"Sales"},
				{"id":"proj-eng-uuid","name":"Engineering"}
			]}}}`))
		case strings.Contains(req.Query, "ListIssues"):
			listVars = req.Variables
			_, _ = w.Write([]byte(`{"data":{"issues":{"nodes":[
				{"id":"iss1","identifier":"FLO-343","title":"Prospect","state":{"name":"In Progress"},
				 "priorityLabel":"High","assignee":null,"team":{"key":"FLO"},"project":{"id":"proj-sales-uuid","name":"Sales"},
				 "labels":{"nodes":[]}}
			]}}}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	orig := linear.APIURL
	linear.APIURL = srv.URL
	defer func() { linear.APIURL = orig }()

	res, err := Execute(nil, nil, conns(
		[2]string{"api_key", "lin_test"},
		[2]string{"project", "Sales"},
		[2]string{"state_name", "In Progress"},
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeTrue())

	// The list query was filtered by the RESOLVED project UUID.
	filter := listVars["filter"].(map[string]interface{})
	proj := filter["project"].(map[string]interface{})["id"].(map[string]interface{})
	Expect(proj["eq"]).To(Equal("proj-sales-uuid"))

	// The summary exposes the project so the caller can identify Sales issues.
	Expect(res["tool_result"].(string)).To(ContainSubstring("project:Sales"))
	Expect(res["tool_result"].(string)).To(ContainSubstring("FLO-343"))
}

// An unresolvable project name fails cleanly (success=false), never silently
// dropping the filter and returning the wrong issues.
func TestExecute_UnresolvableProjectFailsClosed(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"projects":{"nodes":[{"id":"x","name":"Engineering"}]}}}`))
	}))
	defer srv.Close()
	orig := linear.APIURL
	linear.APIURL = srv.URL
	defer func() { linear.APIURL = orig }()

	res, err := Execute(nil, nil, conns([2]string{"api_key", "lin_test"}, [2]string{"project", "Sales"}))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"].(string)).To(ContainSubstring("Sales"))
}
