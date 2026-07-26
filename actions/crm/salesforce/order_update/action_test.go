package crm_salesforce_order_update

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// refuseActivation answers the PATCH the way the live org answers "activate an
// order that has no product lines".
func refuseActivation(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`[{"message":"An order must have at least one product.","errorCode":"FAILED_ACTIVATION","fields":[]}]`))
	}))
	restore := salesforce.SetHostForTest(srv.URL)
	return func() {
		restore()
		srv.Close()
	}
}

// TestAFailedActivationNamesTheStepThatAddsProducts is the regression test.
// Update Order's Status dropdown offers Activated, and on an order with no lines
// Salesforce refuses with FAILED_ACTIVATION. Four sibling actions translate that
// code — Activate Order among them, naming Add Product to Order — and this one did
// not, so the operator was left with the bare code and no next step.
func TestAFailedActivationNamesTheStepThatAddsProducts(t *testing.T) {
	defer refuseActivation(t)()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
		{Name: "order_id", Type: core.ConnectionTypeString, Value: "801aj00003DvqyXAAR"},
		{Name: "order_status", Type: core.ConnectionTypeString, Value: "Activated"},
	}
	out, err := Execute(&core.Flow{}, &core.Node{}, inputs)
	if err != nil {
		t.Fatalf("a Salesforce refusal belongs on the error port: %v", err)
	}
	if ok, _ := out["success"].(bool); ok {
		t.Fatal("the order was not activated, so this cannot be a success")
	}
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "Add Product to Order") {
		t.Errorf("the message has to name the step that fixes this, got: %q", msg)
	}
	if !strings.Contains(msg, "at least one product") {
		t.Errorf("Salesforce's own wording should still be carried, got: %q", msg)
	}
}
