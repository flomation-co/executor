// httptest coverage for the 13 user actions: happy path plus at least one
// error path each, with the wire assertions that matter (method, path, body
// shape, headers, chunking).
package entra_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"

	user_add_to_group "flomation.app/automate/executor/actions/azure/entra/user_add_to_group"
	user_assign_license "flomation.app/automate/executor/actions/azure/entra/user_assign_license"
	user_check_group_membership "flomation.app/automate/executor/actions/azure/entra/user_check_group_membership"
	user_create "flomation.app/automate/executor/actions/azure/entra/user_create"
	user_delete "flomation.app/automate/executor/actions/azure/entra/user_delete"
	user_get "flomation.app/automate/executor/actions/azure/entra/user_get"
	user_get_all "flomation.app/automate/executor/actions/azure/entra/user_get_all"
	user_get_manager "flomation.app/automate/executor/actions/azure/entra/user_get_manager"
	user_list_groups "flomation.app/automate/executor/actions/azure/entra/user_list_groups"
	user_remove_from_group "flomation.app/automate/executor/actions/azure/entra/user_remove_from_group"
	user_revoke_sessions "flomation.app/automate/executor/actions/azure/entra/user_revoke_sessions"
	user_set_manager "flomation.app/automate/executor/actions/azure/entra/user_set_manager"
	user_update "flomation.app/automate/executor/actions/azure/entra/user_update"
)

func TestUserCreate(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	defer entra.SetReplicationForTest(2, time.Millisecond)()
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	var readinessGets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// user_create polls GET /users/{id} after the POST until Graph will
		// serve the new object; record the creating call only.
		if r.Method == http.MethodGet {
			readinessGets++
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"u-new"}`))
			return
		}
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"u-new","userPrincipalName":"jane@contoso.com","displayName":"Jane"}`))
	}))
	defer srv.Close()

	out, err := user_create.Execute(nil, nil, authInputs(srv.URL,
		str("display_name", "Jane"),
		str("user_principal_name", "jane@contoso.com"),
		str("mail_nickname", "jane"),
		secret("password", "Pa55w0rd!"),
		boolean("force_change_password", true),
		obj("additional_fields", `{"givenName":"Jane","usageLocation":"GB"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if gotMethod != "POST" || gotPath != "/v1.0/users" {
		t.Fatalf("call = %s %s", gotMethod, gotPath)
	}
	// The wait requires consecutive confirmations: Graph replicas disagree, so
	// one 200 is no evidence the next reader will find the object.
	if readinessGets != 2 {
		t.Errorf("readiness polls = %d, want 2 consecutive confirmations before the id goes downstream", readinessGets)
	}
	if gotBody["displayName"] != "Jane" || gotBody["mailNickname"] != "jane" {
		t.Fatalf("body = %v", gotBody)
	}
	// account_enabled untouched → default true, not false.
	if gotBody["accountEnabled"] != true {
		t.Fatalf("accountEnabled = %v, want default true", gotBody["accountEnabled"])
	}
	profile, _ := gotBody["passwordProfile"].(map[string]interface{})
	if profile["password"] != "Pa55w0rd!" || profile["forceChangePasswordNextSignIn"] != true {
		t.Fatalf("passwordProfile = %v", profile)
	}
	if gotBody["usageLocation"] != "GB" {
		t.Fatalf("additional_fields not merged: %v", gotBody)
	}
	if out["id"] != "u-new" {
		t.Fatalf("id = %v", out["id"])
	}
}

func TestUserCreateValidatesBeforeCalling(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()

	base := func(nick string) []*core.Connection {
		return authInputs(srv.URL,
			str("display_name", "Jane"),
			str("user_principal_name", "jane@contoso.com"),
			str("mail_nickname", nick),
			secret("password", "p"),
		)
	}
	// mailNickname with an @ must fail client-side, without an HTTP call.
	out, err := user_create.Execute(nil, nil, base("jane@contoso.com"))
	wantSoftFailure(t, out, err, "mail_nickname")
	if called {
		t.Fatal("client-side validation must not reach the API")
	}
}

func TestUserCreateSurfacesGraphError(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 400, "Request_BadRequest", "Another object with the same value for property userPrincipalName already exists.")
	}))
	defer srv.Close()

	out, err := user_create.Execute(nil, nil, authInputs(srv.URL,
		str("display_name", "Jane"),
		str("user_principal_name", "jane@contoso.com"),
		str("mail_nickname", "jane"),
		secret("password", "p"),
	))
	wantSoftFailure(t, out, err, "Request_BadRequest")
}

func TestUserGetAppliesRichDefaultSelect(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotPath, gotSelect string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSelect = r.URL.Query().Get("$select")
		_, _ = w.Write([]byte(`{"id":"u1","userPrincipalName":"jane@contoso.com"}`))
	}))
	defer srv.Close()

	out, err := user_get.Execute(nil, nil, authInputs(srv.URL, str("user_id", "jane@contoso.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	// A UPN in the path is segment-encoded.
	if gotPath != "/v1.0/users/jane@contoso.com" && gotPath != "/v1.0/users/jane%40contoso.com" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotSelect != entra.DefaultUserSelect {
		t.Fatalf("$select = %q, want the rich default", gotSelect)
	}
	// The SharePoint-backed personal props are descoped by design.
	for _, banned := range []string{"aboutMe", "birthday", "skills"} {
		if strings.Contains(gotSelect, banned) {
			t.Errorf("default $select contains descoped property %q", banned)
		}
	}

	// An explicit select wins over the default.
	_, err = user_get.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1"), str("select", "id,displayName")))
	if err != nil {
		t.Fatalf("Execute custom select: %v", err)
	}
	if gotSelect != "id,displayName" {
		t.Fatalf("custom $select = %q", gotSelect)
	}
}

func TestUserGetNotFound(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'nope' does not exist.")
	}))
	defer srv.Close()

	out, err := user_get.Execute(nil, nil, authInputs(srv.URL, str("user_id", "nope")))
	wantSoftFailure(t, out, err, "check the ID/UPN")
}

func TestUserGetAllPagesAndFilters(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	calls := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("ConsistencyLevel") != "eventual" {
			t.Errorf("ConsistencyLevel = %q", r.Header.Get("ConsistencyLevel"))
		}
		q := r.URL.Query()
		if calls == 1 {
			// $count only on OUR page-one URL — the follow-up is the nextLink
			// verbatim, which here deliberately omits it.
			if q.Get("$count") != "true" {
				t.Errorf("$count missing")
			}
			if q.Get("$filter") != "startswith(displayName,'A')" || q.Get("$search") != `"displayName:smith"` {
				t.Errorf("params not passed through: %s", r.URL.RawQuery)
			}
			// Return All pins $top to Graph's 999 maximum.
			if q.Get("$top") != "999" {
				t.Errorf("$top = %q, want 999 on Return All", q.Get("$top"))
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"value":           []map[string]interface{}{{"id": "1"}, {"id": "2"}},
				"@odata.nextLink": srv.URL + "/v1.0/users?$skiptoken=p2",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": []map[string]interface{}{{"id": "3"}}})
	}))
	defer srv.Close()

	out, err := user_get_all.Execute(nil, nil, authInputs(srv.URL,
		str("filter", "startswith(displayName,'A')"),
		str("search", `"displayName:smith"`),
		boolean("return_all", true),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if out["count"] != 3 || calls != 2 {
		t.Fatalf("count=%v calls=%d", out["count"], calls)
	}
}

func TestUserGetAllErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 400, "Request_UnsupportedQuery", "Unsupported or invalid query filter clause.")
	}))
	defer srv.Close()

	out, err := user_get_all.Execute(nil, nil, authInputs(srv.URL, str("filter", "bogus eq")))
	wantSoftFailure(t, out, err, "Request_UnsupportedQuery")
}

func TestUserUpdateMergesConvenienceAndRawFields(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := user_update.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"),
		boolean("account_enabled", false),
		str("job_title", "Engineer"),
		// update_fields is the power-user last word: its jobTitle wins.
		obj("update_fields", `{"jobTitle":"Principal Engineer","usageLocation":"GB"}`),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if gotMethod != "PATCH" || gotPath != "/v1.0/users/u1" {
		t.Fatalf("call = %s %s", gotMethod, gotPath)
	}
	if gotBody["accountEnabled"] != false || gotBody["usageLocation"] != "GB" {
		t.Fatalf("body = %v", gotBody)
	}
	if gotBody["jobTitle"] != "Principal Engineer" {
		t.Fatalf("update_fields must override the convenience field, got %v", gotBody["jobTitle"])
	}
	if out["id"] != "u1" {
		t.Fatalf("id = %v", out["id"])
	}
}

func TestUserUpdateNothingToUpdate(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	defer srv.Close()

	out, err := user_update.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	wantSoftFailure(t, out, err, "nothing to update")
}

func TestUserDelete(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := user_delete.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "DELETE" || gotPath != "/v1.0/users/u1" {
		t.Fatalf("call = %s %s success = %v", gotMethod, gotPath, out["success"])
	}

	// Error path: unknown user.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'u2' does not exist.")
	}))
	defer srv2.Close()
	out, err = user_delete.Execute(nil, nil, authInputs(srv2.URL, str("user_id", "u2")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestUserAddToGroupBindsDirectoryObject(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotPath string
	var gotBody map[string]interface{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := user_add_to_group.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if gotPath != "/v1.0/groups/g1/members/$ref" {
		t.Fatalf("path = %q", gotPath)
	}
	// The binding must be absolute on the SAME Graph host (sovereign clouds).
	if gotBody["@odata.id"] != srv.URL+"/v1.0/directoryObjects/u1" {
		t.Fatalf("@odata.id = %v", gotBody["@odata.id"])
	}
}

func TestUserAddToGroupAlreadyMember(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 400, "Request_BadRequest", "One or more added object references already exist for the following modified properties: 'members'.")
	}))
	defer srv.Close()

	out, err := user_add_to_group.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), str("user_id", "u1")))
	wantSoftFailure(t, out, err, "already a member")
}

func TestUserRemoveFromGroup(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := user_remove_from_group.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1"), str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "DELETE" || gotPath != "/v1.0/groups/g1/members/u1/$ref" {
		t.Fatalf("call = %s %s success=%v", gotMethod, gotPath, out["success"])
	}

	out, err = user_remove_from_group.Execute(nil, nil, authInputs(srv.URL, str("group_id", "g1")))
	wantSoftFailure(t, out, err, "user_id is required")
}

func TestUserListGroupsSwitchesRelationOnTransitive(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"value":[{"id":"g1"},{"id":"g2"}]}`))
	}))
	defer srv.Close()

	out, err := user_list_groups.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["count"] != 2 || gotPath != "/v1.0/users/u1/memberOf" {
		t.Fatalf("count=%v path=%q", out["count"], gotPath)
	}

	_, err = user_list_groups.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1"), boolean("transitive", true)))
	if err != nil {
		t.Fatalf("Execute transitive: %v", err)
	}
	if gotPath != "/v1.0/users/u1/transitiveMemberOf" {
		t.Fatalf("transitive path = %q", gotPath)
	}
}

func TestUserListGroupsErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'nope' does not exist.")
	}))
	defer srv.Close()

	out, err := user_list_groups.Execute(nil, nil, authInputs(srv.URL, str("user_id", "nope")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestUserCheckGroupMembershipChunksAndAggregates(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var batches [][]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/users/u1/checkMemberGroups" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body := decodeBody(t, r)
		ids, _ := body["groupIds"].([]interface{})
		batches = append(batches, ids)
		// Match the first id of every batch.
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": []interface{}{ids[0]}})
	}))
	defer srv.Close()

	// 45 ids → 3 batches of 20/20/5 (Graph's checkMemberGroups cap).
	ids := make([]string, 45)
	for i := range ids {
		ids[i] = fmt.Sprintf("g%02d", i)
	}
	out, err := user_check_group_membership.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"),
		str("group_ids", strings.Join(ids, ",")),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	if len(batches) != 3 || len(batches[0]) != 20 || len(batches[1]) != 20 || len(batches[2]) != 5 {
		t.Fatalf("batches = %d", len(batches))
	}
	matched, _ := out["member_of"].([]interface{})
	if len(matched) != 3 {
		t.Fatalf("member_of = %v", matched)
	}
	// ANY match → is_member true.
	if out["is_member"] != true {
		t.Fatalf("is_member = %v", out["is_member"])
	}
}

func TestUserCheckGroupMembershipNoMatch(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	out, err := user_check_group_membership.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"), str("group_ids", "g1,g2")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["is_member"] != false {
		t.Fatalf("is_member = %v, want false when nothing matched", out["is_member"])
	}
}

func TestUserCheckGroupMembershipErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 400, "Request_BadRequest", "Group ids should not be more than 20.")
	}))
	defer srv.Close()

	out, err := user_check_group_membership.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"), str("group_ids", "g1")))
	wantSoftFailure(t, out, err, "Request_BadRequest")

	out, err = user_check_group_membership.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"), str("group_ids", " , ,")))
	wantSoftFailure(t, out, err, "group_ids")
}

func TestUserAssignLicenseBody(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/users/u1/assignLicense" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotBody = decodeBody(t, r)
		_, _ = w.Write([]byte(`{"id":"u1","assignedLicenses":[{"skuId":"sku-1"}]}`))
	}))
	defer srv.Close()

	out, err := user_assign_license.Execute(nil, nil, authInputs(srv.URL,
		str("user_id", "u1"),
		str("add_sku_ids", "sku-1, sku-2"),
		str("remove_sku_ids", "sku-old"),
		str("disabled_plans", "plan-a"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	add, _ := gotBody["addLicenses"].([]interface{})
	if len(add) != 2 {
		t.Fatalf("addLicenses = %v", add)
	}
	first, _ := add[0].(map[string]interface{})
	if first["skuId"] != "sku-1" {
		t.Fatalf("skuId = %v", first["skuId"])
	}
	plans, _ := first["disabledPlans"].([]interface{})
	if len(plans) != 1 || plans[0] != "plan-a" {
		t.Fatalf("disabledPlans = %v", plans)
	}
	remove, _ := gotBody["removeLicenses"].([]interface{})
	if len(remove) != 1 || remove[0] != "sku-old" {
		t.Fatalf("removeLicenses = %v", remove)
	}
}

func TestUserAssignLicenseErrors(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	// Neither add nor remove → soft error without a call.
	srvNever := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	defer srvNever.Close()
	out, err := user_assign_license.Execute(nil, nil, authInputs(srvNever.URL, str("user_id", "u1")))
	wantSoftFailure(t, out, err, "nothing to assign")

	// Graph's usage-location refusal gets the actionable hint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 400, "Request_BadRequest", "License assignment cannot be done for user with invalid usage location.")
	}))
	defer srv.Close()
	out, err = user_assign_license.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1"), str("add_sku_ids", "sku-1")))
	wantSoftFailure(t, out, err, "usageLocation")
}

func TestUserRevokeSessions(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"value":true}`))
	}))
	defer srv.Close()

	out, err := user_revoke_sessions.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || gotMethod != "POST" || gotPath != "/v1.0/users/u1/revokeSignInSessions" {
		t.Fatalf("call = %s %s success=%v", gotMethod, gotPath, out["success"])
	}
	result, _ := out["result"].(map[string]interface{})
	if result["value"] != true {
		t.Fatalf("result = %v", result)
	}

	out, err = user_revoke_sessions.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "user_id is required")
}

func TestUserGetManager(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/users/u1/manager" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"mgr-1","displayName":"Boss"}`))
	}))
	defer srv.Close()

	out, err := user_get_manager.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "mgr-1" {
		t.Fatalf("out = %v", out)
	}

	// No manager set → the specific soft failure, not a raw 404.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'manager' does not exist.")
	}))
	defer srv404.Close()
	out, err = user_get_manager.Execute(nil, nil, authInputs(srv404.URL, str("user_id", "u1")))
	wantSoftFailure(t, out, err, "No manager is set")
}

func TestUserSetManager(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	var gotBody map[string]interface{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	out, err := user_set_manager.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1"), str("manager_id", "mgr-1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success=false: %v", out["error"])
	}
	// Manager assignment is PUT (replace-the-reference), not POST.
	if gotMethod != "PUT" || gotPath != "/v1.0/users/u1/manager/$ref" {
		t.Fatalf("call = %s %s", gotMethod, gotPath)
	}
	if gotBody["@odata.id"] != srv.URL+"/v1.0/users/mgr-1" {
		t.Fatalf("@odata.id = %v", gotBody["@odata.id"])
	}

	out, err = user_set_manager.Execute(nil, nil, authInputs(srv.URL, str("user_id", "u1")))
	wantSoftFailure(t, out, err, "manager_id is required")
}
