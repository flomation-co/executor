package salesforce

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Scope vs filters — what "Match ANY filter instead of all" is allowed to touch
// ---------------------------------------------------------------------------
//
// A get-many action collects WHERE terms from two different kinds of input, and
// they are NOT interchangeable:
//
//   - SCOPE. The action's own built-in boundary boxes, whose labels are
//     unconditional promises: "Active Users Only", "Due On or After",
//     "Starts On or Before". An operator reading those has been told what the
//     result set is, full stop.
//   - FILTERS. The operator's own Filter Field / Comparison / Value row and the
//     More Filters JSON. "Match ANY filter instead of all" is about these.
//
// Feeding both into one flat Condition list and letting the toggle pick a single
// connective ORs the scope away, and for a RANGE the result is not merely wider,
// it is a tautology: "Due On or After 1 July OR Due On or Before 31 July" is
// true of every task that has a due date at all — including the ones with none,
// since SOQL lets NULL satisfy `<= date`. Verified against a live org: 6 rows
// ANDed, 33 (the entire Task table) ORed, with every other filter on the form
// annihilated at the same time. "Active Users Only" ORed is the same shape of
// wrong: it returns every active user in the org PLUS the deactivated ones the
// tick box exists to hide.
//
// So scope is always ANDed and the toggle joins the filters only:
//
//	WHERE IsActive = true AND (Department = 'Sales' OR Department = 'Support')
//
// SOQL takes that grouping happily, including with a NOT LIKE term inside the
// group (`A AND ((NOT (Name LIKE 'z%')) OR B)` — verified live), which the flat
// builder cannot express because it exposes no brackets.

// BuildScopedQueryTyped assembles a get-many statement in which the action's own
// scope terms are ALWAYS ANDed and only the operator's filters are joined by the
// Match ANY toggle. It is otherwise BuildQueryTyped: same validation, same
// field-type-aware literals, same one cached describe, same degradation to the
// value-only heuristic when the connected user cannot run that describe.
//
// With the toggle off, or with nothing in one of the two lists, the emitted SOQL
// is byte-identical to what BuildQueryTyped would have produced — the bracketed
// form appears only when it changes the meaning.
func BuildScopedQueryTyped(instanceURL, token, object, fields string, scope, filters []Condition, matchAnyFilter bool, orderBy string, limit int, applyLimit bool) (string, error) {
	switch {
	case len(scope) == 0:
		// Nothing to protect — the toggle only ever sees the operator's filters.
		return BuildQueryTyped(instanceURL, token, object, fields, filters, matchAnyFilter, orderBy, limit, applyLimit)
	case len(filters) == 0:
		// Scope alone. Note the false: ORing two ends of a range together is the
		// tautology above, and the toggle has no filters to apply itself to.
		return BuildQueryTyped(instanceURL, token, object, fields, scope, false, orderBy, limit, applyLimit)
	case !matchAnyFilter:
		all := make([]Condition, 0, len(scope)+len(filters))
		all = append(all, scope...)
		all = append(all, filters...)
		return BuildQueryTyped(instanceURL, token, object, fields, all, false, orderBy, limit, applyLimit)
	}

	// The one case that needs brackets: scope AND (filter OR filter OR ...).
	var types map[string]string
	if t, err := FieldTypes(instanceURL, token, object); err == nil {
		types = t
	}
	scopeWhere, err := BuildWhereTyped(scope, false, types)
	if err != nil {
		return "", err
	}
	filterWhere, err := BuildWhereTyped(filters, true, types)
	if err != nil {
		return "", err
	}
	sel, err := BuildSelect(object, fields)
	if err != nil {
		return "", err
	}

	query := sel + " " + scopeWhere + " AND (" + strings.TrimPrefix(filterWhere, "WHERE ") + ")"
	if ob := strings.TrimSpace(orderBy); ob != "" {
		clause, err := BuildOrderBy(ob)
		if err != nil {
			return "", err
		}
		query += " " + clause
	}
	if applyLimit && limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}
	return query, nil
}
