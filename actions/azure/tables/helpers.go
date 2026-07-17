package tables

// Helpers the actions share: point-op argument plumbing, the bounded pager
// walk, batch parsing, access-policy parsing, and the response shapers. Split
// out of common.go, which is already the auth/client/error file.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

// marshalRow re-encodes a validated row for the SDK, which parses the bytes
// itself. Re-marshalling rather than passing the operator's original text
// through guarantees the SDK sees exactly what EntityKeys validated.
func marshalRow(row map[string]interface{}) ([]byte, error) {
	raw, err := json.Marshal(row)
	if err != nil {
		return nil, fmt.Errorf("the row could not be encoded: %w", err)
	}
	return raw, nil
}

func etagOf(s string) azcore.ETag { return azcore.ETag(s) }

// Now is the package clock, exported so the SAS action reads the same seam the
// SAS test pins.
func Now() time.Time { return nowFunc() }

// SetNowForTest pins the clock and returns a restore func. Test-only.
func SetNowForTest(t time.Time) func() {
	prev := nowFunc
	nowFunc = func() time.Time { return t }
	return func() { nowFunc = prev }
}

// PointArgs reads the table + composite identity every point operation needs.
// The keys are validated here so a bad one is named by rule rather than
// arriving as an opaque 400 — or, worse, being silently URL-mangled.
func PointArgs(inputs []*core.Connection) (table, partitionKey, rowKey string, err error) {
	if table, err = RequiredString("table", inputs); err != nil {
		return "", "", "", err
	}
	if partitionKey, err = RequiredString("partition_key", inputs); err != nil {
		return "", "", "", err
	}
	if err = ValidateKey("partition_key", partitionKey); err != nil {
		return "", "", "", err
	}
	if rowKey, err = RequiredString("row_key", inputs); err != nil {
		return "", "", "", err
	}
	if err = ValidateKey("row_key", rowKey); err != nil {
		return "", "", "", err
	}
	return table, partitionKey, rowKey, nil
}

// IsNotFound reports whether an error is the service saying the entity is not
// there. The Table service is inconsistent about which code it uses, so match
// on the status as well as the two names it answers with.
func IsNotFound(err error) bool {
	switch ErrorCode(err) {
	case "ResourceNotFound", "EntityNotFound", "TableNotFound":
		return true
	}
	return StatusCode(err) == 404
}

// UpdateModeSummary phrases the mode for a tool_result, saying what it DID
// rather than naming the mode — "replace" in a run record is not self-evident
// three weeks later, and the consequence is the part worth recording.
func UpdateModeSummary(mode aztables.UpdateMode) string {
	if mode == aztables.UpdateModeReplace {
		return "replacing it — any field not supplied was removed"
	}
	return "merging — fields not supplied were left alone"
}

// SelectFields projects an entity down to the named fields, always keeping the
// identity and etag: a projected row that cannot be fed back into Update Row
// is a trap, and dropping the keys would do exactly that.
func SelectFields(entity map[string]interface{}, fields string) map[string]interface{} {
	keep := map[string]bool{"PartitionKey": true, "RowKey": true, "etag": true}
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			keep[f] = true
		}
	}
	out := map[string]interface{}{}
	for k, v := range entity {
		// The "Prop@odata.type" sidecar travels with its property or the type
		// is lost on the way back in.
		base := k
		if i := strings.Index(k, "@odata.type"); i > 0 {
			base = k[:i]
		}
		if keep[base] {
			out[k] = v
		}
	}
	return out
}

// WalkPages drives an azcore pager, bounded.
//
// The bound is the point. $top is PER PAGE, not a total, so a return_all walk
// asks for pages until the service stops offering a continuation — which on a
// large table is never, in any time an operator would tolerate. maxListPages
// makes the worst case finite and reports that it did.
//
// The empty-page case is the subtle one: a filtered scan can legitimately
// return an EMPTY page WITH a continuation token ("nothing matched in the
// range I scanned, keep going"), so stopping on an empty page would silently
// drop results. Only More() decides when the walk is over.
func WalkPages(flow *core.Flow, returnAll bool, more func() bool, next func() (interface{}, error)) (items []interface{}, capped bool, err error) {
	items = []interface{}{}
	for pages := 0; ; pages++ {
		batch, err := next()
		if err != nil {
			return nil, false, err
		}
		if page, ok := batch.([]interface{}); ok {
			items = append(items, page...)
		}
		if !returnAll || !more() {
			return items, false, nil
		}
		if pages+1 >= maxListPages {
			return items, true, nil
		}
		if err := reqContext(flow).Err(); err != nil {
			return nil, false, err
		}
	}
}

// ---------------------------------------------------------------------------
// Batch
// ---------------------------------------------------------------------------

// Batch is a validated transaction: the SDK actions plus the facts the result
// and the error annotator need.
type Batch struct {
	PartitionKey string
	RowKeys      []string
	Actions      []aztables.TransactionAction
}

// batchActionTypes maps the operator-facing verb to the SDK's. The names are
// the flow author's vocabulary (upsert_merge, not insertmerge) because the
// service's own names are internal jargon.
var batchActionTypes = map[string]aztables.TransactionType{
	"insert":         aztables.TransactionTypeAdd,
	"merge":          aztables.TransactionTypeUpdateMerge,
	"replace":        aztables.TransactionTypeUpdateReplace,
	"upsert_merge":   aztables.TransactionTypeInsertMerge,
	"upsert_replace": aztables.TransactionTypeInsertReplace,
	"delete":         aztables.TransactionTypeDelete,
}

func batchActionNames() string {
	names := make([]string, 0, len(batchActionTypes))
	for k := range batchActionTypes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ParseBatch reads and validates the changes array.
//
// Every rule here is enforced client-side deliberately:
//
//   - Same partition. This is the service's rule — a changeset is atomic
//     precisely because it lands on one partition — but AZURITE DOES NOT
//     ENFORCE IT (verified: a batch mixing two partition keys came back
//     successful against the emulator while real Azure returns 400). So the
//     emulator cannot be trusted to catch this and the operator would only
//     find out in production, from an error that does not name the cause.
//   - No duplicate RowKeys. The service rejects them with an error that names
//     neither key.
//   - The 100-entity cap, checked before we build a multipart body for a batch
//     that cannot succeed.
func ParseBatch(inputs []*core.Connection, name string) (Batch, error) {
	value, err := OptionalJSON(name, inputs)
	if err != nil {
		return Batch{}, err
	}
	list, ok := value.([]interface{})
	if !ok {
		return Batch{}, fmt.Errorf(`%s must be a JSON array of changes, e.g. [{"action":"upsert_merge","row":{"PartitionKey":"orders","RowKey":"1001"}}]`, name)
	}
	if len(list) == 0 {
		return Batch{}, fmt.Errorf("%s is empty — a transaction needs at least one change", name)
	}
	if len(list) > MaxBatchEntities {
		return Batch{}, fmt.Errorf("%s has %d changes — a transaction is capped at %d. Split it, or loop", name, len(list), MaxBatchEntities)
	}

	batch := Batch{Actions: make([]aztables.TransactionAction, 0, len(list))}
	seen := map[string]int{}
	for i, item := range list {
		change, ok := item.(map[string]interface{})
		if !ok {
			return Batch{}, fmt.Errorf(`%s[%d] must be an object like {"action":"upsert_merge","row":{…}}`, name, i)
		}

		verb, _ := change["action"].(string)
		verb = strings.TrimSpace(strings.ToLower(verb))
		actionType, known := batchActionTypes[verb]
		if !known {
			return Batch{}, fmt.Errorf("%s[%d] has action %q — use one of: %s", name, i, verb, batchActionNames())
		}

		row, ok := change["row"].(map[string]interface{})
		if !ok {
			return Batch{}, fmt.Errorf(`%s[%d] has no "row" object — every change carries the row it applies to, including a delete (which needs only PartitionKey and RowKey)`, name, i)
		}
		partitionKey, rowKey, err := EntityKeys(row)
		if err != nil {
			return Batch{}, fmt.Errorf("%s[%d]: %w", name, i, err)
		}

		if i == 0 {
			batch.PartitionKey = partitionKey
		} else if partitionKey != batch.PartitionKey {
			return Batch{}, fmt.Errorf("%s[%d] is in partition %q but the transaction started in %q — every change in one transaction must share a PartitionKey, because that is what makes it all-or-nothing. Split it into one transaction per partition",
				name, i, partitionKey, batch.PartitionKey)
		}
		if first, dup := seen[rowKey]; dup {
			return Batch{}, fmt.Errorf("%s[%d] targets RowKey %q, which %s[%d] already targets — a transaction may touch each row only once", name, i, rowKey, name, first)
		}
		seen[rowKey] = i
		batch.RowKeys = append(batch.RowKeys, rowKey)

		raw, err := marshalRow(row)
		if err != nil {
			return Batch{}, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		action := aztables.TransactionAction{ActionType: actionType, Entity: raw}
		if etag, _ := change["etag"].(string); etag != "" {
			action.IfMatch = to.Ptr(etagOf(etag))
		}
		batch.Actions = append(batch.Actions, action)
	}
	return batch, nil
}

// BatchErrorf renders a failed transaction, naming the change that broke it.
//
// The service reports the failure as the INDEX of the offending operation,
// prefixed onto the error code ("1:EntityAlreadyExists"). An index is not
// something an operator can act on — this turns it back into the RowKey they
// wrote — and it also says the transaction rolled back, because the whole
// point of a batch is that a partial failure changed nothing.
//
// Not every batch failure carries an index. When the service answers 202 with
// a 4xx inside the multipart body, aztables raises the error from the OUTER
// response, whose status is 202 and which has no error code — the inner status
// is lost before we ever see it. Those fall through to the plain message.
func (a Auth) BatchErrorf(err error, batch Batch) string {
	base := a.Errorf(err)
	i, ok := batchFailureIndex(err)
	if !ok || i >= len(batch.RowKeys) {
		return base
	}
	return fmt.Sprintf("change %d of %d (RowKey %q) failed and the whole transaction was rolled back — %s",
		i+1, len(batch.RowKeys), batch.RowKeys[i], base)
}

func batchFailureIndex(err error) (int, bool) {
	idx, _, split := strings.Cut(ErrorCode(err), ":")
	if !split {
		return 0, false
	}
	i, convErr := strconv.Atoi(idx)
	if convErr != nil || i < 0 {
		return 0, false
	}
	return i, true
}

// SignTableSAS signs a table SAS and returns a token the service will actually
// accept.
//
// The append is not a nicety — it is a workaround for a bug in aztables
// v1.4.1. SASSignatureValues.Sign signs StartPartitionKey/StartRowKey/
// EndPartitionKey/EndRowKey INTO the string-to-sign, but never copies them
// into the SASQueryParameters it encodes (sas_service.go:52), and those fields
// are unexported so nothing outside the package can set them. The token that
// comes back therefore omits the range it was signed with, so the service
// recomputes the string-to-sign with empty range slots, gets a different
// signature, and rejects the link with a bare 403.
//
// Appending the four params makes the token agree with the signature that was
// already computed over them, which is exactly what the service expects. When
// no range is set there is nothing to append and this is a plain Sign.
func SignTableSAS(values aztables.SASSignatureValues, cred *aztables.SharedKeyCredential) (string, error) {
	token, err := values.Sign(cred)
	if err != nil {
		return "", err
	}
	for _, p := range []struct{ key, value string }{
		{"spk", values.StartPartitionKey},
		{"srk", values.StartRowKey},
		{"epk", values.EndPartitionKey},
		{"erk", values.EndRowKey},
	} {
		if p.value != "" {
			token += "&" + p.key + "=" + url.QueryEscape(p.value)
		}
	}
	return token, nil
}

// ---------------------------------------------------------------------------
// Access policies
// ---------------------------------------------------------------------------

// ParseAccessPolicies reads the policies array.
//
// An empty array is VALID and means "remove every policy" — that is the only
// way to clear them, so it must not be confused with the input being unset.
func ParseAccessPolicies(inputs []*core.Connection, name string) ([]*aztables.SignedIdentifier, error) {
	value, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	list, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON array, e.g. [{"id":"readonly","permissions":"r","expiry":"2027-01-01T00:00:00Z"}] — send [] to remove every policy`, name)
	}
	if len(list) > MaxAccessPolicies {
		return nil, fmt.Errorf("%s has %d policies — a table holds at most %d", name, len(list), MaxAccessPolicies)
	}

	out := make([]*aztables.SignedIdentifier, 0, len(list))
	for i, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`%s[%d] must be an object like {"id":"readonly","permissions":"r","expiry":"2027-01-01T00:00:00Z"}`, name, i)
		}
		id, _ := obj["id"].(string)
		if id = strings.TrimSpace(id); id == "" {
			return nil, fmt.Errorf("%s[%d] has no id — a stored policy is referenced by name, so it needs one", name, i)
		}
		perms, _ := obj["permissions"].(string)
		if err := (&aztables.SASPermissions{}).Parse(perms); err != nil || perms == "" {
			return nil, fmt.Errorf(`%s[%d] has permissions %q — use any subset of "raud" (read, add, update, delete)`, name, i, perms)
		}

		policy := &aztables.AccessPolicy{Permission: to.Ptr(perms)}
		if raw, _ := obj["expiry"].(string); raw != "" {
			t, err := parsePolicyTime(fmt.Sprintf("%s[%d].expiry", name, i), raw)
			if err != nil {
				return nil, err
			}
			policy.Expiry = to.Ptr(t)
		}
		if raw, _ := obj["start"].(string); raw != "" {
			t, err := parsePolicyTime(fmt.Sprintf("%s[%d].start", name, i), raw)
			if err != nil {
				return nil, err
			}
			policy.Start = to.Ptr(t)
		}
		out = append(out, &aztables.SignedIdentifier{ID: to.Ptr(id), AccessPolicy: policy})
	}
	return out, nil
}

func parsePolicyTime(field, raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q is not a valid timestamp — use RFC3339, e.g. 2027-01-01T00:00:00Z", field, raw)
	}
	return t.UTC(), nil
}

// ---------------------------------------------------------------------------
// Response shaping
// ---------------------------------------------------------------------------

// ShapeAccessPolicy renders a stored policy as a flat object a flow can read,
// and one that round-trips back into Set Access Policies unchanged.
func ShapeAccessPolicy(id *aztables.SignedIdentifier) map[string]interface{} {
	out := map[string]interface{}{}
	if id == nil {
		return out
	}
	if id.ID != nil {
		out["id"] = *id.ID
	}
	if id.AccessPolicy == nil {
		return out
	}
	if id.AccessPolicy.Permission != nil {
		out["permissions"] = *id.AccessPolicy.Permission
	}
	if id.AccessPolicy.Start != nil {
		out["start"] = id.AccessPolicy.Start.UTC().Format(time.RFC3339)
	}
	if id.AccessPolicy.Expiry != nil {
		out["expiry"] = id.AccessPolicy.Expiry.UTC().Format(time.RFC3339)
	}
	return out
}

// ShapeServiceProperties flattens the account's Table service settings.
func ShapeServiceProperties(props aztables.ServiceProperties) map[string]interface{} {
	out := map[string]interface{}{
		"cors":           shapeCors(props.Cors),
		"logging":        shapeLogging(props.Logging),
		"hour_metrics":   shapeMetrics(props.HourMetrics),
		"minute_metrics": shapeMetrics(props.MinuteMetrics),
	}
	return out
}

func shapeCors(rules []*aztables.CorsRule) []interface{} {
	out := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		if r == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"allowed_origins":    deref(r.AllowedOrigins),
			"allowed_methods":    deref(r.AllowedMethods),
			"allowed_headers":    deref(r.AllowedHeaders),
			"exposed_headers":    deref(r.ExposedHeaders),
			"max_age_in_seconds": derefInt32(r.MaxAgeInSeconds),
		})
	}
	return out
}

func shapeLogging(l *aztables.Logging) map[string]interface{} {
	if l == nil {
		return nil
	}
	return map[string]interface{}{
		"version":          deref(l.Version),
		"read":             derefBool(l.Read),
		"write":            derefBool(l.Write),
		"delete":           derefBool(l.Delete),
		"retention_policy": shapeRetention(l.RetentionPolicy),
	}
}

func shapeMetrics(m *aztables.Metrics) map[string]interface{} {
	if m == nil {
		return nil
	}
	return map[string]interface{}{
		"version":          deref(m.Version),
		"enabled":          derefBool(m.Enabled),
		"include_apis":     derefBool(m.IncludeAPIs),
		"retention_policy": shapeRetention(m.RetentionPolicy),
	}
}

func shapeRetention(r *aztables.RetentionPolicy) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"enabled": derefBool(r.Enabled),
		"days":    derefInt32(r.Days),
	}
}

// ShapeGeoReplication renders the secondary-region replication status.
func ShapeGeoReplication(g *aztables.GeoReplication) map[string]interface{} {
	if g == nil {
		return nil
	}
	out := map[string]interface{}{}
	if g.Status != nil {
		out["status"] = string(*g.Status)
	}
	if g.LastSyncTime != nil {
		out["last_sync_time"] = g.LastSyncTime.UTC().Format(time.RFC3339)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	return b != nil && *b
}

func derefInt32(i *int32) interface{} {
	if i == nil {
		return nil
	}
	return int(*i)
}
