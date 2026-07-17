// httptest coverage for the 3 directory actions.
package entra_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	entra "flomation.app/automate/executor/actions/azure/entra"

	deleted_item_restore "flomation.app/automate/executor/actions/azure/entra/deleted_item_restore"
	guest_invite "flomation.app/automate/executor/actions/azure/entra/guest_invite"
	subscribed_skus_get_all "flomation.app/automate/executor/actions/azure/entra/subscribed_skus_get_all"
)

func TestGuestInviteDefaultsAndOutputs(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1.0/invitations" {
			t.Errorf("call = %s %s", r.Method, r.URL.Path)
		}
		gotBody = decodeBody(t, r)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"inv-1","inviteRedeemUrl":"https://invitations.microsoft.com/redeem/x","status":"PendingAcceptance","invitedUser":{"id":"u-guest"}}`))
	}))
	defer srv.Close()

	out, err := guest_invite.Execute(nil, nil, authInputs(srv.URL, str("email", "guest@partner.com")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "inv-1" {
		t.Fatalf("out = %v", out)
	}
	if gotBody["invitedUserEmailAddress"] != "guest@partner.com" {
		t.Fatalf("body = %v", gotBody)
	}
	// Defaults: Microsoft's landing page, invitation email on.
	if gotBody["inviteRedirectUrl"] != "https://myapplications.microsoft.com" {
		t.Fatalf("inviteRedirectUrl = %v", gotBody["inviteRedirectUrl"])
	}
	if gotBody["sendInvitationMessage"] != true {
		t.Fatalf("sendInvitationMessage = %v, want default true", gotBody["sendInvitationMessage"])
	}
	if out["invite_redeem_url"] != "https://invitations.microsoft.com/redeem/x" {
		t.Fatalf("invite_redeem_url = %v", out["invite_redeem_url"])
	}
	if out["invited_user_id"] != "u-guest" {
		t.Fatalf("invited_user_id = %v", out["invited_user_id"])
	}
}

func TestGuestInviteCustomMessageAndOptOut(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"inv-2","invitedUser":{"id":"u2"}}`))
	}))
	defer srv.Close()

	_, err := guest_invite.Execute(nil, nil, authInputs(srv.URL,
		str("email", "guest@partner.com"),
		str("display_name", "Guest One"),
		boolean("send_invitation", false),
		text("message", "Welcome aboard"),
	))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotBody["sendInvitationMessage"] != false {
		t.Fatalf("sendInvitationMessage = %v, want explicit false", gotBody["sendInvitationMessage"])
	}
	if gotBody["invitedUserDisplayName"] != "Guest One" {
		t.Fatalf("invitedUserDisplayName = %v", gotBody["invitedUserDisplayName"])
	}
	info, _ := gotBody["invitedUserMessageInfo"].(map[string]interface{})
	if info["customizedMessageBody"] != "Welcome aboard" {
		t.Fatalf("invitedUserMessageInfo = %v", info)
	}
}

func TestGuestInviteErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 403, "Authorization_RequestDenied", "Insufficient privileges to complete the operation.")
	}))
	defer srv.Close()

	out, err := guest_invite.Execute(nil, nil, authInputs(srv.URL, str("email", "guest@partner.com")))
	wantSoftFailure(t, out, err, "Authorization_RequestDenied")

	out, err = guest_invite.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "email is required")
}

func TestDeletedItemRestore(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"id":"u1","displayName":"Jane","userPrincipalName":"jane@contoso.com"}`))
	}))
	defer srv.Close()

	out, err := deleted_item_restore.Execute(nil, nil, authInputs(srv.URL, str("object_id", "u1")))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["id"] != "u1" {
		t.Fatalf("out = %v", out)
	}
	if gotMethod != "POST" || gotPath != "/v1.0/directory/deletedItems/u1/restore" {
		t.Fatalf("call = %s %s", gotMethod, gotPath)
	}

	// Restore window elapsed / never deleted → Graph 404.
	srv404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 404, "Request_ResourceNotFound", "Resource 'u1' does not exist.")
	}))
	defer srv404.Close()
	out, err = deleted_item_restore.Execute(nil, nil, authInputs(srv404.URL, str("object_id", "u1")))
	wantSoftFailure(t, out, err, "Request_ResourceNotFound")
}

func TestSubscribedSkusGetAll(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	var gotQuery string
	var gotConsistency string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/subscribedSkus" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotQuery = r.URL.RawQuery
		gotConsistency = r.Header.Get("ConsistencyLevel")
		_, _ = w.Write([]byte(`{"value":[{"skuId":"sku-1","skuPartNumber":"ENTERPRISEPACK","consumedUnits":42}]}`))
	}))
	defer srv.Close()

	out, err := subscribed_skus_get_all.Execute(nil, nil, authInputs(srv.URL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["success"] != true || out["count"] != 1 {
		t.Fatalf("out = %v", out)
	}
	// /subscribedSkus supports only $select — the advanced-query pair must be
	// absent or Graph rejects the call.
	if gotQuery != "" {
		t.Fatalf("query = %q, want none", gotQuery)
	}
	if gotConsistency != "" {
		t.Fatalf("ConsistencyLevel = %q, want unset", gotConsistency)
	}
}

func TestSubscribedSkusGetAllErrorPath(t *testing.T) {
	defer entra.SetTokenForTest("tok")()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		graphError(w, 403, "Authorization_RequestDenied", "Insufficient privileges to complete the operation.")
	}))
	defer srv.Close()

	out, err := subscribed_skus_get_all.Execute(nil, nil, authInputs(srv.URL))
	wantSoftFailure(t, out, err, "Authorization_RequestDenied")
}
