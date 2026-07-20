//go:build livesmoke

package tables_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

	entity_batch "flomation.app/automate/executor/actions/azure/tables/entity_batch"
	entity_get "flomation.app/automate/executor/actions/azure/tables/entity_get"
	entity_insert "flomation.app/automate/executor/actions/azure/tables/entity_insert"
	entity_query "flomation.app/automate/executor/actions/azure/tables/entity_query"
	entity_update "flomation.app/automate/executor/actions/azure/tables/entity_update"
	table_create "flomation.app/automate/executor/actions/azure/tables/table_create"
	table_delete "flomation.app/automate/executor/actions/azure/tables/table_delete"
	table_get_all "flomation.app/automate/executor/actions/azure/tables/table_get_all"
)

const (
	azuriteKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	azuriteURL = "http://127.0.0.1:10002/devstoreaccount1"
)

func in(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "account_name", Type: core.ConnectionTypeString, Value: "devstoreaccount1"},
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: azuriteKey},
		{Name: "endpoint", Type: core.ConnectionTypeString, Value: azuriteURL},
	}, extra...)
}

func s(n, v string) *core.Connection {
	return &core.Connection{Name: n, Type: core.ConnectionTypeString, Value: v}
}
func o(n, v string) *core.Connection {
	return &core.Connection{Name: n, Type: core.ConnectionTypeObject, Value: v}
}

type execFn func(*core.Flow, *core.Node, []*core.Connection) (map[string]interface{}, error)

func ok(t *testing.T, label string, fn execFn, inputs []*core.Connection) map[string]interface{} {
	t.Helper()
	out, err := fn(&core.Flow{}, nil, inputs)
	if err != nil {
		t.Fatalf("%s: hard error %v", label, err)
	}
	if out["success"] != true {
		t.Fatalf("%s: %v", label, out["error"])
	}
	fmt.Printf("  PASS %-24s %v\n", label, out["tool_result"])
	return out
}

func TestLiveAgainstAzurite(t *testing.T) {
	const table = "FloSmoke1"
	_, _ = table_delete.Execute(&core.Flow{}, nil, in(s("table", table),
		&core.Connection{Name: "ignore_if_missing", Type: core.ConnectionTypeBoolean, Value: true}))

	ok(t, "table_create", table_create.Execute, in(s("table", table)))
	defer table_delete.Execute(&core.Flow{}, nil, in(s("table", table),
		&core.Connection{Name: "ignore_if_missing", Type: core.ConnectionTypeBoolean, Value: true}))

	ok(t, "table_get_all", table_get_all.Execute, in())

	// Edm.Int64 must survive as a string + sidecar, not silently lose precision.
	ok(t, "entity_insert", entity_insert.Execute, in(s("table", table),
		o("entity", `{"PartitionKey":"uk","RowKey":"1001","Customer":"Acme","Total":42,"Big":"9007199254740993","Big@odata.type":"Edm.Int64"}`)))

	got := ok(t, "entity_get", entity_get.Execute, in(s("table", table),
		s("partition_key", "uk"), s("row_key", "1001")))
	row := got["result"].(map[string]interface{})
	if row["Big"] != "9007199254740993" {
		t.Errorf("Edm.Int64 lost precision on the round trip: %v", row["Big"])
	}
	if _, leaked := row["odata.metadata"]; leaked {
		t.Errorf("odata.metadata reached the flow output: %v", row)
	}
	etag, _ := row["etag"].(string)
	if etag == "" {
		t.Fatal("no etag came back from a real service")
	}

	// Merge must preserve Customer; this is the data-loss guard.
	ok(t, "entity_update/merge", entity_update.Execute, in(s("table", table),
		o("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":43}`), s("etag", etag)))
	after := ok(t, "entity_get (merged)", entity_get.Execute, in(s("table", table),
		s("partition_key", "uk"), s("row_key", "1001")))["result"].(map[string]interface{})
	if after["Customer"] != "Acme" || after["Total"] != float64(43) {
		t.Errorf("merge did not merge: %v", after)
	}

	// The stale etag is now genuinely stale — a real 412.
	stale, err := entity_update.Execute(&core.Flow{}, nil, in(s("table", table),
		o("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":99}`), s("etag", etag)))
	if err != nil {
		t.Fatalf("stale etag: hard error %v", err)
	}
	if stale["success"] != false {
		t.Errorf("a stale etag must fail: %v", stale)
	}
	fmt.Printf("  PASS %-22s %v\n", "412 soft error", stale["error"])

	// Replace is destructive — pin that it really does delete Customer.
	ok(t, "entity_update/replace", entity_update.Execute, in(s("table", table),
		o("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":50}`), s("update_mode", "replace")))
	replaced := ok(t, "entity_get (replaced)", entity_get.Execute, in(s("table", table),
		s("partition_key", "uk"), s("row_key", "1001")))["result"].(map[string]interface{})
	if _, present := replaced["Customer"]; present {
		t.Errorf("replace was supposed to remove Customer: %v", replaced)
	}

	ok(t, "entity_batch", entity_batch.Execute, in(s("table", table),
		o("actions", `[
			{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"2001","Total":1}},
			{"action":"upsert_merge","row":{"PartitionKey":"uk","RowKey":"2002","Total":2}},
			{"action":"delete","row":{"PartitionKey":"uk","RowKey":"1001"}}
		]`)))

	q := ok(t, "entity_query", entity_query.Execute, in(s("table", table),
		s("filter", "PartitionKey eq 'uk'"),
		&core.Connection{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true}))
	if q["count"] != 2 {
		t.Errorf("count = %v, want the two batch rows with 1001 deleted", q["count"])
	}
}

// TestLiveSASRangeIsRejectedWithoutTheAppend is the evidence for
// SignTableSAS's workaround. aztables v1.4.1 signs the partition/row range
// into the string-to-sign but never encodes it into the token, so the service
// recomputes a different signature and rejects the link. This proves both
// halves against a real Table service: the SDK's own token FAILS, ours works.
func TestLiveSASRangeIsRejectedWithoutTheAppend(t *testing.T) {
	const table = "FloSmoke2"
	_, _ = table_delete.Execute(&core.Flow{}, nil, in(s("table", table),
		&core.Connection{Name: "ignore_if_missing", Type: core.ConnectionTypeBoolean, Value: true}))
	ok(t, "table_create", table_create.Execute, in(s("table", table)))
	defer table_delete.Execute(&core.Flow{}, nil, in(s("table", table),
		&core.Connection{Name: "ignore_if_missing", Type: core.ConnectionTypeBoolean, Value: true}))
	ok(t, "entity_insert", entity_insert.Execute, in(s("table", table),
		o("entity", `{"PartitionKey":"uk","RowKey":"1001","Total":42}`)))

	cred, err := aztables.NewSharedKeyCredential("devstoreaccount1", azuriteKey)
	if err != nil {
		t.Fatalf("credential: %v", err)
	}
	values := aztables.SASSignatureValues{
		TableName:         table,
		Permissions:       "r",
		ExpiryTime:        time.Now().UTC().Add(time.Hour),
		StartPartitionKey: "uk",
		EndPartitionKey:   "uk",
	}

	// The table name goes in verbatim, NOT lowercased to match the
	// signature's canonical name: the service lowercases for signature
	// validation but looks the table up by the path as sent, and that lookup
	// is case-sensitive here. table_generate_sas builds its URL the same way.
	read := func(token string) error {
		c, err := aztables.NewClientWithNoCredential(
			azuriteURL+"/"+table+"?"+token, nil)
		if err != nil {
			return err
		}
		_, err = c.GetEntity(context.Background(), "uk", "1001", nil)
		return err
	}

	bare, err := values.Sign(cred)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if strings.Contains(bare, "spk=") {
		t.Skip("aztables now encodes the range itself — SignTableSAS's workaround can go")
	}
	if err := read(bare); err == nil {
		t.Error("the SDK's own token was ACCEPTED with a range set — the workaround may no longer be needed")
	} else {
		fmt.Printf("  PASS %-24s SDK token rejected as expected: %s\n", "sas (bare SDK)", tables.ErrorCode(err))
	}

	fixed, err := tables.SignTableSAS(values, cred)
	if err != nil {
		t.Fatalf("SignTableSAS: %v", err)
	}
	if err := read(fixed); err != nil {
		t.Fatalf("our token was rejected too — the workaround does not work: %v", err)
	}
	fmt.Printf("  PASS %-24s range-limited SAS read the row\n", "sas (SignTableSAS)")
}
