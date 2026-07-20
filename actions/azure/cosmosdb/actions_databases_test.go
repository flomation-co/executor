// Happy-path + error-path coverage for the azure/cosmosdb database actions.
package cosmosdb_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	database_create "flomation.app/automate/executor/actions/azure/cosmosdb/database_create"
	database_delete "flomation.app/automate/executor/actions/azure/cosmosdb/database_delete"
	database_get "flomation.app/automate/executor/actions/azure/cosmosdb/database_get"
	database_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/database_get_all"
)

func TestDatabaseCreate(t *testing.T) {
	RegisterTestingT(t)
	var gotBody map[string]interface{}
	var gotThroughput, gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotThroughput = r.Header.Get("x-ms-offer-throughput")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("x-ms-request-charge", "4.95")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"db1","_rid":"x","_self":"dbs/x/"}`))
	}))
	defer server.Close()

	out, err := database_create.Execute(nil, nil, authFor(server.URL, str("database", "db1"), integer("throughput", 500)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodPost))
	Expect(gotPath).To(Equal("/dbs"))
	Expect(gotBody).To(Equal(map[string]interface{}{"id": "db1"}))
	Expect(gotThroughput).To(Equal("500"))
	Expect(out["id"]).To(Equal("db1"))
	Expect(out["request_charge"]).To(Equal("4.95"))
}

func TestDatabaseCreateConflictIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"Conflict","message":"exists"}`))
	}))
	defer server.Close()

	out, err := database_create.Execute(nil, nil, authFor(server.URL, str("database", "db1")))
	Expect(err).To(BeNil(), "provider errors are soft failures")
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("already exists"))
}

func TestDatabaseCreateRejectsBothThroughputModes(t *testing.T) {
	RegisterTestingT(t)
	// Validation fails before any network call — the endpoint is unreachable.
	out, err := database_create.Execute(nil, nil, authFor("http://127.0.0.1:1",
		str("database", "db1"), integer("throughput", 500), integer("autoscale_max", 4000)))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("mutually exclusive"))
}

func TestDatabaseGet(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("x-ms-request-charge", "1.00")
		_, _ = w.Write([]byte(`{"id":"db1","_rid":"sys","_ts":1}`))
	}))
	defer server.Close()

	out, err := database_get.Execute(nil, nil, authFor(server.URL, str("database", "db1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotPath).To(Equal("/dbs/db1"))
	// simplify defaults on: system props stripped.
	Expect(out["result"]).To(Equal(map[string]interface{}{"id": "db1"}))
}

func TestDatabaseGetNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	}))
	defer server.Close()

	out, err := database_get.Execute(nil, nil, authFor(server.URL, str("database", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not found"))
}

func TestDatabaseGetAll(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ms-request-charge", "2.00")
		_, _ = w.Write([]byte(`{"Databases":[{"id":"a","_rid":"r1"},{"id":"b","_rid":"r2"}],"_count":2}`))
	}))
	defer server.Close()

	out, err := database_get_all.Execute(nil, nil, authFor(server.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["count"]).To(Equal(2))
	results := out["results"].([]interface{})
	Expect(results[0]).To(Equal(map[string]interface{}{"id": "a"}), "simplify defaults on")
}

func TestDatabaseGetAllErrorIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"Unauthorized","message":"bad signature"}`))
	}))
	defer server.Close()

	out, err := database_get_all.Execute(nil, nil, authFor(server.URL))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("Unauthorized"))
}

func TestDatabaseDelete(t *testing.T) {
	RegisterTestingT(t)
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("x-ms-request-charge", "5.00")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	out, err := database_delete.Execute(nil, nil, authFor(server.URL, str("database", "db1")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(gotMethod).To(Equal(http.MethodDelete))
	Expect(gotPath).To(Equal("/dbs/db1"))
	Expect(out["result"]).To(HaveKeyWithValue("deleted", true))
}

func TestDatabaseDeleteNotFoundIsSoft(t *testing.T) {
	RegisterTestingT(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NotFound","message":"gone"}`))
	}))
	defer server.Close()

	out, err := database_delete.Execute(nil, nil, authFor(server.URL, str("database", "nope")))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("not found"))
}
