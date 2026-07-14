package pgvector

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	"github.com/lib/pq"
)

// Metadata filtering, in two tiers.
//
// The simple tier (metadata_filter, a key/value grid) is AND-ed exact equality.
// That is the whole of what n8n can express, and it is the 95% case: "only
// search documents where source = handbook".
//
// The advanced tier (metadata_filter_json) adds comparison, membership, text
// matching, existence and boolean grouping:
//
//	{"price": {"gt": 10},
//	 "tag":   {"in": ["a","b"]},
//	 "$or":   [{"status":{"eq":"live"}}, {"pinned":{"eq":true}}]}
//
// Both compile to the same bound predicates. The KEY is bound, not
// interpolated — LangChain (and therefore n8n) builds this clause as
//
//	`${this.metadataColumnName}->>'${key}' = $${paramCount}`
//
// with the key coming from an expression-capable UI field, which is a live SQL
// injection in shipped n8n. `metadata->>$1 = $2` is the same query with none of
// the hole, so there is no reason to do it the other way.

const (
	maxFilterDepth      = 5
	maxFilterPredicates = 50
)

// Filter is a compiled WHERE fragment plus the values it binds.
type Filter struct {
	SQL  string // "" when there is nothing to filter on
	Args []interface{}
}

// builder threads the running $n counter through a recursive compile.
type builder struct {
	col        string // pre-quoted metadata column
	args       []interface{}
	predicates int
}

func (b *builder) next(v interface{}) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// BuildFilter compiles both filter surfaces into one AND-ed WHERE fragment.
//
// startArg is the number of parameters already bound by the caller (the query
// vector, the limit, ...), so the placeholders continue the caller's sequence
// rather than restarting at $1.
func BuildFilter(cols ColumnSet, simple []*core.Connection, advancedJSON string, startArg int) (Filter, error) {
	kv, err := parseKeyValueFilter(simple)
	if err != nil {
		return Filter{}, err
	}
	adv := strings.TrimSpace(advancedJSON)

	if len(kv) == 0 && (adv == "" || adv == "{}" || adv == "null") {
		return Filter{}, nil
	}
	if !cols.HasMetadata() {
		return Filter{}, fmt.Errorf(
			"this table has no metadata column, so there's nothing to filter on — " +
				"remove the metadata filter, or point this step at a table with a jsonb metadata column")
	}

	b := &builder{col: cols.QMetadata, args: make([]interface{}, startArg)}

	var clauses []string
	// Deterministic order: the key/value grid is already ordered by the operator.
	// The cap is enforced here as well as in compileField — both surfaces feed
	// the same WHERE clause, so counting the grid's rows without checking them
	// would leave it able to blow the bound on its own.
	for _, pair := range kv {
		b.predicates++
		if b.predicates > maxFilterPredicates {
			return Filter{}, fmt.Errorf("the metadata filter has more than %d conditions", maxFilterPredicates)
		}
		clauses = append(clauses, b.col+"->>"+b.next(pair.key)+" = "+b.next(pair.value))
	}

	if adv != "" && adv != "{}" && adv != "null" {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(adv), &obj); err != nil {
			return Filter{}, fmt.Errorf(
				"the Advanced Metadata Filter isn't valid JSON: %v. It should look like "+
					`{"source": {"eq": "handbook"}, "page": {"gt": 3}}`, err)
		}
		sqlFrag, err := b.compileObject(obj, 1)
		if err != nil {
			return Filter{}, err
		}
		if sqlFrag != "" {
			clauses = append(clauses, sqlFrag)
		}
	}

	if len(clauses) == 0 {
		return Filter{}, nil
	}
	// Drop the caller's placeholder slots back off — they own those args.
	return Filter{
		SQL:  strings.Join(clauses, " AND "),
		Args: b.args[startArg:],
	}, nil
}

type kvPair struct{ key, value string }

func parseKeyValueFilter(conns []*core.Connection) ([]kvPair, error) {
	var out []kvPair
	for _, c := range conns {
		if c == nil || c.Value == nil {
			continue
		}
		// A KeyValueArray arrives as []any of {"key":…,"value":…}, or as its
		// JSON text when it came through the ${...} substitution pass.
		var raw []map[string]interface{}
		switch v := c.Value.(type) {
		case string:
			s := strings.TrimSpace(v)
			if s == "" || s == "[]" || s == "null" {
				continue
			}
			if err := json.Unmarshal([]byte(s), &raw); err != nil {
				return nil, fmt.Errorf("couldn't read the Metadata Filter: %v", err)
			}
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("couldn't read the Metadata Filter: %v", err)
			}
			if err := json.Unmarshal(b, &raw); err != nil {
				return nil, fmt.Errorf("couldn't read the Metadata Filter: %v", err)
			}
		}
		for _, e := range raw {
			k := stringify(e["key"])
			if k == "" {
				continue
			}
			out = append(out, kvPair{key: k, value: stringify(e["value"])})
		}
	}
	return out, nil
}

// compileObject compiles {"field": <spec>, "$and": [...], "$or": [...]}.
func (b *builder) compileObject(obj map[string]json.RawMessage, depth int) (string, error) {
	if depth > maxFilterDepth {
		return "", fmt.Errorf("the Advanced Metadata Filter is nested more than %d levels deep", maxFilterDepth)
	}

	// Sort keys so the generated SQL is deterministic (tests, and query-plan caching).
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sortStrings(keys)

	var clauses []string
	for _, key := range keys {
		raw := obj[key]

		if key == "$and" || key == "$or" {
			var groups []map[string]json.RawMessage
			if err := json.Unmarshal(raw, &groups); err != nil {
				return "", fmt.Errorf("%s must be a list of filter objects", key)
			}
			var sub []string
			for _, g := range groups {
				frag, err := b.compileObject(g, depth+1)
				if err != nil {
					return "", err
				}
				if frag != "" {
					sub = append(sub, frag)
				}
			}
			if len(sub) == 0 {
				continue
			}
			joiner := " AND "
			if key == "$or" {
				joiner = " OR "
			}
			clauses = append(clauses, "("+strings.Join(sub, joiner)+")")
			continue
		}

		frag, err := b.compileField(key, raw)
		if err != nil {
			return "", err
		}
		if frag != "" {
			clauses = append(clauses, frag)
		}
	}

	return strings.Join(clauses, " AND "), nil
}

// compileField compiles one field's spec. A bare scalar is shorthand for eq,
// so {"source":"handbook"} works as well as {"source":{"eq":"handbook"}}.
func (b *builder) compileField(field string, raw json.RawMessage) (string, error) {
	b.predicates++
	if b.predicates > maxFilterPredicates {
		return "", fmt.Errorf("the Advanced Metadata Filter has more than %d conditions", maxFilterPredicates)
	}

	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		// Not an object — treat as equality shorthand.
		var scalar interface{}
		if err := json.Unmarshal(raw, &scalar); err != nil {
			return "", fmt.Errorf("couldn't read the filter for %q", field)
		}
		return b.col + "->>" + b.next(field) + " = " + b.next(stringify(scalar)), nil
	}

	ops := make([]string, 0, len(spec))
	for k := range spec {
		ops = append(ops, k)
	}
	sortStrings(ops)

	var clauses []string
	for _, op := range ops {
		frag, err := b.compileOp(field, op, spec[op])
		if err != nil {
			return "", err
		}
		clauses = append(clauses, frag)
	}
	if len(clauses) == 1 {
		return clauses[0], nil
	}
	return "(" + strings.Join(clauses, " AND ") + ")", nil
}

func (b *builder) compileOp(field, op string, raw json.RawMessage) (string, error) {
	// jsonb's own operators (?, ?|, @>) are not lib/pq placeholders — lib/pq
	// uses $n exclusively — so they pass through to the server untouched.
	switch op {
	case "eq":
		return b.col + "->>" + b.next(field) + " = " + b.next(scalarArg(raw)), nil

	case "neq":
		// NULL-tolerant, matching LangChain's semantics: a document that has no
		// such key at all does satisfy "tag != draft".
		k1 := b.next(field)
		v := b.next(scalarArg(raw))
		k2 := b.next(field)
		return "(" + b.col + "->>" + k1 + " IS NULL OR " + b.col + "->>" + k2 + " <> " + v + ")", nil

	case "in", "notIn":
		vals, err := stringArray(raw)
		if err != nil {
			return "", fmt.Errorf("%s.%s must be a list of values", field, op)
		}
		clause := b.col + "->>" + b.next(field) + " = ANY(" + b.next(pq.Array(vals)) + "::text[])"
		if op == "notIn" {
			return "NOT (" + clause + ")", nil
		}
		return clause, nil

	case "gt", "gte", "lt", "lte":
		sym := map[string]string{"gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[op]
		var num float64
		if err := json.Unmarshal(raw, &num); err != nil {
			return "", fmt.Errorf("%s.%s must be a number", field, op)
		}
		// The ->> cast to numeric is what makes "page > 3" mean what the
		// operator expects; without it Postgres compares "10" < "9" as text.
		return "(" + b.col + "->>" + b.next(field) + ")::numeric " + sym + " " + b.next(num), nil

	case "like", "ilike":
		sym := "LIKE"
		if op == "ilike" {
			sym = "ILIKE"
		}
		return b.col + "->>" + b.next(field) + " " + sym + " " + b.next(scalarArg(raw)), nil

	case "exists":
		var want bool
		if err := json.Unmarshal(raw, &want); err != nil {
			return "", fmt.Errorf("%s.exists must be true or false", field)
		}
		clause := b.col + " ? " + b.next(field)
		if !want {
			return "NOT (" + clause + ")", nil
		}
		return clause, nil

	case "isNull":
		var want bool
		if err := json.Unmarshal(raw, &want); err != nil {
			return "", fmt.Errorf("%s.isNull must be true or false", field)
		}
		if want {
			return b.col + "->>" + b.next(field) + " IS NULL", nil
		}
		return b.col + "->>" + b.next(field) + " IS NOT NULL", nil

	case "arrayContains":
		vals, err := stringArray(raw)
		if err != nil {
			return "", fmt.Errorf("%s.arrayContains must be a list of values", field)
		}
		return b.col + "->" + b.next(field) + " ?| " + b.next(pq.Array(vals)) + "::text[]", nil

	case "contains":
		// Whole-object containment against the metadata column.
		return b.col + " @> " + b.next(string(raw)) + "::jsonb", nil
	}

	return "", fmt.Errorf(
		"%q isn't a filter operator. Use one of: eq, neq, in, notIn, gt, gte, lt, lte, like, ilike, "+
			"exists, isNull, arrayContains, contains", op)
}

// scalarArg renders a JSON scalar as the text jsonb's ->> operator will compare
// against. Everything out of ->> is text, so the bound value must be too:
// {"page": {"eq": 3}} has to compare against "3", not the integer 3.
func scalarArg(raw json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.Trim(string(raw), `"`)
	}
	return stringify(v)
}

func stringArray(raw json.RawMessage) ([]string, error) {
	var vals []interface{}
	if err := json.Unmarshal(raw, &vals); err != nil {
		return nil, err
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = stringify(v)
	}
	return out, nil
}

// stringify renders a JSON scalar the way jsonb's ->> would, so that filters
// compare like with like. Notably a JSON number decodes to float64, and
// fmt.Sprint would render 3 as "3" but 1e21 as "1e+21"; strconv with 'f'/-1
// matches Postgres's own text output for the values that actually appear in
// metadata.
func stringify(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// sortStrings is a tiny insertion sort — the slices here are filter keys, so
// single digits, and this avoids pulling "sort" in for one call.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
