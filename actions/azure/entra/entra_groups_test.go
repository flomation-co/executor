// httptest coverage for the 9 group actions.
package entra_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	entra "flomation.app/automate/executor/actions/azure/entra"

	group_add_members "flomation.app/automate/executor/actions/azure/entra/group_add_members"
	group_create "flomation.app/automate/executor/actions/azure/entra/group_create"
	group_delete "flomation.app/automate/executor/actions/azure/entra/group_delete"
	group_get "flomation.app/automate/executor/actions/azure/entra/group_get"
	group_get_all "flomation.app/automate/executor/actions/azure/entra/group_get_all"
	group_list_members "flomation.app/automate/executor/actions/azure/entra/group_list_members"
	group_list_owners "flomation.app/automate/executor/actions/azure/entra/group_list_owners"
	group_remove_member "flomation.app/automate/executor/actions/azure/entra/group_remove_member"
	group_update "flomation.app/automate/executor/actions/azure/entra/group_update"
)

func TestGroupCreateSecurityDefaults(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1.0/groups" {
			t.Errorf("call = %s %s", r.Method, r.URL.Path)
		}
		gotBody = decodeBody(t, r)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"g-new","displayName":"Sales"}`))
	}))
	defer srv.Close()

	out, err := group_create.Execute(nil, nil, authInputs(srv.URL,
		str("display_name", "Sales"),
		str("mail_nickname", "sales"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "g-new" {
		t.Fatalf("out = %v", out)
	}
	// Graph requires both flags explicitly; an unset dropdown means Security.
	if gotBody["mailEnabled"] != false || gotBody["securityEnabled"] != true {
		t.Fatalf("security defaults: %v", gotBody)
	}
	if _, present := gotBody["groupTypes"]; present {
		t.Fatalf("plain security group must not send groupTypes: %v", gotBody["groupTypes"])
	}
}

func TestGroupCreateUnifiedDynamicWithOwners(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotBody map[string]interface{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"g-new"}`))
	}))
	defer srv.Close()

	out, err := group_create.Execute(nil, nil, authInputs(srv.URL,
		str("display_name", "Sales"),
		str("mail_nickname", "sales"),
		str("group_type", "unified"),
		str("visibility", "Private"),
		text("membership_rule", `(user.department -eq "Sales")`),
		str("owners", "o1, o2"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	types, _ := gotBody["groupTypes"].([]interface{})
	joined := fmt.Sprintf("%v", types)
	if !strings.Contains(joined, "Unified") || !strings.Contains(joined, "DynamicMembership") {
		t.Fatalf("groupTypes = %v", types)
	}
	if gotBody["mailEnabled"] != true || gotBody["securityEnabled"] != false {
		t.Fatalf("unified flags: %v", gotBody)
	}
	if gotBody["membershipRuleProcessingState"] != "On" {
		t.Fatalf("rule processing state = %v", gotBody["membershipRuleProcessingState"])
	}
	if gotBody["visibility"] != "Private" {
		t.Fatalf("visibility = %v", gotBody["visibility"])
	}
	owners, _ := gotBody["owners@odata.bind"].([]interface{})
	if len(owners) != 2 || owners[0] != srv.URL+"/v1.0/users/o1" {
		t.Fatalf("owners bind = %v", owners)
	}
}

func TestGroupCreateValidatesNickname(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	defer srv.Close()

	out, err := group_create.Execute(nil, nil, authInputs(srv.URL,
		str("display_name", "Sales"),
		str("mail_nickname", "sales team"), // space → invalid
	))
	wantSoftFailure(t, out, err, "mail_nickname")
}

func TestGroupGet(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotPath, gotSelect string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSelect = r.URL.Query().Get("$select")
		_, _ = w.Write([]byte(`{"id":"g1","displayName":"Sales"}`))
	}))
	defer srv.Close()

	out, err := group_get.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), str("select", "id,displayName")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotPath != "/v1.0/groups/g1" || gotSelect != "id,displayName" {
		t.Fatalf("path=%q select=%q out=%v", gotPath, gotSelect, out)
	}

	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'g9' does not exist.")
	}))
	defer srv404.Close()
	out, err = group_get.Execute(nil, nil, authInputs(srv404.URL, str("group_id", "g9")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestGroupGetAllSingleLimitedPage(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var srv *httptest.Server
	calls := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("ConsistencyLevel") != "eventual" {
			t.Errorf("ConsistencyLevel = %q", r.Header.Get("ConsistencyLevel"))
		}
		if got := r.URL.Query().Get("$top"); got != "5" {
			t.Errorf("$top = %q, want the explicit limit", got)
		}
		// A pending nextLink that must NOT be followed without Return All.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"value":           []map[string]interface{}{{"id": "g1"}},
			"@odata.nextLink": srv.URL + "/v1.0/groups?$skiptoken=x",
		})
	}))
	defer srv.Close()

	out, err := group_get_all.Execute(nil, nil, authInputs(srv.URL, integer("limit", 5)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 1 || calls != 1 {
		t.Fatalf("count=%v calls=%d", out["count"], calls)
	}
}

func TestGroupGetAllErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 401, "InvalidAuthenticationToken", "Access token has expired or is not yet valid.")
	}))
	defer srv.Close()

	out, err := group_get_all.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "InvalidAuthenticationToken")
}

func TestGroupUpdate(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := group_update.Execute(nil, nil, authInputs(srv.URL,
		str("group_id", "g1"),
		obj("update_fields", `{"description":"Handles inbound"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "PATCH" || gotPath != "/v1.0/groups/g1" {
		t.Fatalf("call = %s %s success=%v", gotMethod, gotPath, out["success"])
	}
	if gotBody["description"] != "Handles inbound" {
		t.Fatalf("body = %v", gotBody)
	}

	// Missing update_fields → soft error.
	out, err = group_update.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1")))
	wantSoftFailure(t, out, err, "update_fields is required")
}

func TestGroupDelete(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := group_delete.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "DELETE" || gotPath != "/v1.0/groups/g1" {
		t.Fatalf("call = %s %s success=%v", gotMethod, gotPath, out["success"])
	}

	out, err = group_delete.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "group_id is required")
}

func TestGroupListMembersSwitchesRelationAndPages(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotPath string
	calls := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotPath = r.URL.Path
		if calls == 1 && r.URL.Query().Get("$select") != "id,displayName" {
			t.Errorf("$select = %q", r.URL.Query().Get("$select"))
		}
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"value":           []map[string]interface{}{{"id": "u1"}},
				"@odata.nextLink": srv.URL + "/v1.0/groups/g1/members?$skiptoken=x",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": []map[string]interface{}{{"id": "u2"}}})
	}))
	defer srv.Close()

	out, err := group_list_members.Execute(nil, nil, authInputs(srv.URL,
		str("group_id", "g1"),
		str("select", "id,displayName"),
		boolean("return_all", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["count"] != 2 || calls != 2 {
		t.Fatalf("count=%v calls=%d", out["count"], calls)
	}

	_, err = group_list_members.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), boolean("transitive", true)))
	if err != nil {
		t.Fatalf("Execute transitive: %v", err)
	}
	if gotPath != "/v1.0/groups/g1/transitiveMembers" {
		t.Fatalf("transitive path = %q", gotPath)
	}
}

func TestGroupListMembersErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'g9' does not exist.")
	}))
	defer srv.Close()

	out, err := group_list_members.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g9")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestGroupAddMembersChunksTwentyPerPatch(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var batches [][]interface{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/v1.0/groups/g1" {
			t.Errorf("call = %s %s", r.Method, r.URL.Path)
		}
		body := decodeBody(t, r)
		binds, _ := body["members@odata.bind"].([]interface{})
		batches = append(batches, binds)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	ids := make([]string, 25)
	for i := range ids {
		ids[i] = fmt.Sprintf("u%02d", i)
	}
	out, err := group_add_members.Execute(nil, nil, authInputs(srv.URL,
		str("group_id", "g1"),
		str("user_ids", strings.Join(ids, ",")),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	// 25 ids → 20 + 5 (Graph's members@odata.bind cap).
	if len(batches) != 2 || len(batches[0]) != 20 || len(batches[1]) != 5 {
		t.Fatalf("batches = %d", len(batches))
	}
	if batches[0][0] != srv.URL+"/v1.0/directoryObjects/u00" {
		t.Fatalf("bind = %v", batches[0][0])
	}
}

func TestGroupAddMembersReportsPartialProgressOnFailure(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(204)
			return
		}
		graphError(w, 400, "Request_BadRequest", "One or more added object references already exist for the following modified properties: 'members'.")
	}))
	defer srv.Close()

	ids := make([]string, 25)
	for i := range ids {
		ids[i] = fmt.Sprintf("u%02d", i)
	}
	out, err := group_add_members.Execute(nil, nil, authInputs(srv.URL,
		str("group_id", "g1"),
		str("user_ids", strings.Join(ids, ",")),
	))
	// Batch 1 landed, batch 2 failed — the error must say how far it got.
	wantSoftFailure(t, out, err, "added 20 of 25")
	wantSoftFailure(t, out, err, "already a member")
}

func TestGroupRemoveMember(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := group_remove_member.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "DELETE" || gotPath != "/v1.0/groups/g1/members/u1/$ref" {
		t.Fatalf("call = %s %s success=%v", gotMethod, gotPath, out["success"])
	}

	srvErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'u1' does not exist.")
	}))
	defer srvErr.Close()
	out, err = group_remove_member.Execute(nil, nil, authInputs(srvErr.URL, str("group_id", "g1"), str("user_id", "u1")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestGroupListOwners(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/groups/g1/owners" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"value":[{"id":"o1"},{"id":"o2"}]}`))
	}))
	defer srv.Close()

	out, err := group_list_owners.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 2 {
		t.Fatalf("out = %v", out)
	}

	out, err = group_list_owners.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "group_id is required")
}
