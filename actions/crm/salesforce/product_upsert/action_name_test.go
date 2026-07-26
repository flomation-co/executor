package crm_salesforce_product_upsert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

// The regression these tests pin down: Product2.Name is an idLookup field, so
// matching on Name — the DEFAULT when Match On is left blank — takes Salesforce's
// native upsert, and UpsertRecord MUST strip the match field from the body
// (Salesforce answers 400 "The Name field should not be specified in the sobject
// data" otherwise). The summary used to be built from the operator's Product Name
// box, so it quoted a name that had been thrown away — and a rename-only run
// sent an empty body, wrote nothing at all, and still reported "Updated".
//
// Verified against the live org before and after the fix.

// recorder captures every request the action makes, because the whole question
// is what reached Salesforce versus what the operator was told.
type recorder struct {
	patches []map[string]interface{}
	paths   []string
}

func (rec *recorder) serve(t *testing.T, storedName string) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.paths = append(rec.paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/sobjects/Product2/Name/"):
			raw, _ := io.ReadAll(r.Body)
			body := map[string]interface{}{}
			_ = json.Unmarshal(raw, &body)
			rec.patches = append(rec.patches, body)
			// Salesforce's own upsert answer: an id and created flag, and NO Name.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "01t5f000004AbCdAAK", "success": true, "created": false, "errors": []interface{}{},
			})
		case strings.Contains(r.URL.Path, "/query"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"totalSize": 1, "done": true,
				"records": []map[string]interface{}{{"Id": "01t5f000004AbCdAAK", "Name": storedName}},
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

func base() []*core.Connection {
	return []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "instance_url", Type: core.ConnectionTypeString, Value: "https://x.my.salesforce.com"},
	}
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// Matching on Name (the default) while asking for a DIFFERENT Name is a
// contradiction Salesforce will not honour, so it must be refused BEFORE
// anything is written — not silently dropped and reported as a rename.
func TestRenameWhileMatchingOnNameIsRefused(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		// Match On deliberately left blank: this is the default configuration.
		str("match_value", "GenWatt Diesel 200kW"),
		str("name", "GenWatt Diesel 200kW (2027 model)"),
	))
	if err == nil {
		t.Fatalf("a rename that cannot happen must be refused, got success: %v", out)
	}
	msg := err.Error()
	if !strings.Contains(msg, "Update Product") {
		t.Errorf("the refusal must name the action that CAN rename a product, got %q", msg)
	}
	if !strings.Contains(msg, "GenWatt Diesel 200kW (2027 model)") {
		t.Errorf("the refusal must quote the name that would have been discarded, got %q", msg)
	}
	if len(rec.paths) != 0 {
		t.Errorf("nothing may be written when the combination is refused, got %v", rec.paths)
	}
}

// The same value in both boxes is the ordinary sync shape and must keep working.
func TestMatchingOnNameWithTheSameValueStillWorks(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		str("match_value", "GenWatt Diesel 200kW"),
		str("name", "GenWatt Diesel 200kW"),
		str("product_code", "GC1040"),
	))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if ok, _ := out["success"].(bool); !ok {
		t.Fatalf("expected success, got %v", out["error"])
	}
	if len(rec.patches) != 1 {
		t.Fatalf("expected one upsert, got %v", rec.patches)
	}
	// The match field must NOT be in the body — Salesforce rejects it outright.
	if _, present := rec.patches[0]["Name"]; present {
		t.Errorf("Name must be stripped from the body on the native path, got %v", rec.patches[0])
	}
	if rec.patches[0]["ProductCode"] != "GC1040" {
		t.Errorf("the fields the operator did fill in must still be written, got %v", rec.patches[0])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, `product "GenWatt Diesel 200kW"`) {
		t.Errorf("summary should name the product that was stored, got %q", summary)
	}
}

// A field-level refresh matched on Name leaves the Product Name box blank. The
// summary must still name the product — from the value Salesforce stored, which
// on this path is the match value. Before the fix the label came from the empty
// box and the product went unnamed in the run log.
func TestSummaryNamesTheProductWhenTheNameBoxIsBlank(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		str("match_value", "GenWatt Diesel 200kW"),
		str("description", "Standby generator, 200kW"),
	))
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, `Updated product "GenWatt Diesel 200kW"`) {
		t.Errorf("summary must name the product Salesforce holds, got %q", summary)
	}
}

// The same contradiction posted through the advanced escape hatch. Additional
// Fields is raw JSON that merges straight into the body, so it bypassed the
// typed Product Name check entirely — the write path then stripped the match
// field and the rename vanished, with the run reporting "Updated". The guard
// therefore has to look at the MERGED body, not the input box.
func TestRenameSmuggledThroughAdditionalFieldsIsAlsoRefused(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	out, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		str("match_value", "GenWatt Diesel 200kW"),
		&core.Connection{
			Name: "additional_fields", Type: core.ConnectionTypeObject,
			Value: map[string]interface{}{"Name": "GenWatt Diesel 200kW (2027 model)"},
		},
	))
	if err == nil {
		t.Fatalf("a rename through Additional Fields cannot happen either and must be refused, got: %v", out)
	}
	if msg := err.Error(); !strings.Contains(msg, "GenWatt Diesel 200kW (2027 model)") {
		t.Errorf("the refusal must quote the value that would have been discarded, got %q", msg)
	}
	if len(rec.paths) != 0 {
		t.Errorf("nothing may be written when the combination is refused, got %v", rec.paths)
	}
}

// Capitalisation in Additional Fields is whatever the operator typed, and
// Salesforce field names are case-insensitive — so a lower-case "name" is the
// same contradiction and must not slip past the guard.
func TestSmuggledRenameIsCaughtRegardlessOfCapitalisation(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	_, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		str("match_value", "GenWatt Diesel 200kW"),
		&core.Connection{
			Name: "additional_fields", Type: core.ConnectionTypeObject,
			Value: map[string]interface{}{"name": "Something else entirely"},
		},
	))
	if err == nil {
		t.Fatal("a lower-case name key is the same contradiction and must be refused")
	}
	if len(rec.paths) != 0 {
		t.Errorf("nothing may be written when the combination is refused, got %v", rec.paths)
	}
}

// Matching on a NON-Name field has the identical failure: the write path strips
// whatever the match field is, so a smuggled Product Code while matching on
// Product Code is discarded just as silently.
func TestSmuggledMatchFieldIsRefusedForAnyMatchField(t *testing.T) {
	rec := &recorder{}
	defer rec.serve(t, "GenWatt Diesel 200kW")()

	_, err := Execute(&core.Flow{}, &core.Node{}, append(base(),
		str("match_field", "ProductCode"),
		str("match_value", "GEN-200"),
		&core.Connection{
			Name: "additional_fields", Type: core.ConnectionTypeObject,
			Value: map[string]interface{}{"ProductCode": "GEN-201"},
		},
	))
	if err == nil {
		t.Fatal("changing the match field through Additional Fields must be refused for any match field, not just Name")
	}
	if msg := err.Error(); !strings.Contains(msg, "ProductCode") {
		t.Errorf("the refusal must name the field involved, got %q", msg)
	}
	if len(rec.paths) != 0 {
		t.Errorf("nothing may be written when the combination is refused, got %v", rec.paths)
	}
}
