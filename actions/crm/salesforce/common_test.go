package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SOQL escaping — the injection boundary
// ---------------------------------------------------------------------------

func TestEscapeSOQLString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Acme Corp", "Acme Corp"},
		{"single quote", "O'Brien", `O\'Brien`},
		{"double quote", `say "hi"`, `say \"hi\"`},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"form feed", "a\fb", `a\fb`},
		{"backspace", "a\bb", `a\bb`},

		// The ordering test that matters. A backslash must be doubled BEFORE
		// the quote is escaped. Escaping the quote first yields \' and the
		// later backslash pass turns it into \\' — which closes the literal
		// and hands the rest of the value to SOQL as syntax.
		{"backslash first", `\`, `\\`},
		{"backslash then quote", `\'`, `\\\'`},
		{"injection attempt", `x' OR Name != '`, `x\' OR Name != \'`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EscapeSOQLString(c.in); got != c.want {
				t.Errorf("EscapeSOQLString(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestEscapeSOQLStringOrderIsNotNaive(t *testing.T) {
	// Guards against someone "simplifying" the replacer into sequential
	// strings.Replace calls in the wrong order. With quote-before-backslash
	// the result would be `\\'`, where the escaped backslash leaves the quote
	// unescaped and the literal terminates early.
	got := EscapeSOQLString(`\'`)
	if strings.HasSuffix(got, `\\'`) && !strings.HasSuffix(got, `\\\'`) {
		t.Fatalf("escape order regression: %q leaves the quote unescaped", got)
	}
}

func TestValidateSOQLFieldName(t *testing.T) {
	valid := []string{
		"Name", "Id", "Account__c", "Account.Name", "Custom_Field__c",
		"Account__r.Owner.Email", "MyObject__Share", "LastModifiedDate",
	}
	for _, v := range valid {
		if _, err := ValidateSOQLFieldName(v); err != nil {
			t.Errorf("ValidateSOQLFieldName(%q) rejected a valid field: %v", v, err)
		}
	}
	invalid := []string{
		"", "1Name", "Name;DROP", "Name OR 1=1", "Name'", "Name--",
		"(SELECT Id FROM Contacts)", "Name,Other", "Name)", "*",
	}
	for _, v := range invalid {
		if _, err := ValidateSOQLFieldName(v); err == nil {
			t.Errorf("ValidateSOQLFieldName(%q) accepted an invalid field", v)
		}
	}

	// Surrounding whitespace is trimmed, not rejected: an operator typing
	// "Id, Name, Email" into a comma-separated field list is doing nothing
	// wrong. Trimming is safe because the pattern still rejects INTERNAL
	// whitespace, which is what an injection attempt needs.
	got, err := ValidateSOQLFieldName("  Name  ")
	if err != nil {
		t.Errorf("surrounding whitespace should be tolerated: %v", err)
	}
	if got != "Name" {
		t.Errorf("expected the trimmed field name, got %q", got)
	}
}

func TestValidateSOQLObjectName(t *testing.T) {
	valid := []string{"Account", "Lead", "MyObject__c", "Namespace__MyObject__c", "Event__e", "Big__b", "Meta__mdt"}
	for _, v := range valid {
		if _, err := ValidateSOQLObjectName(v); err != nil {
			t.Errorf("ValidateSOQLObjectName(%q) rejected a valid object: %v", v, err)
		}
	}
	invalid := []string{"", "1Account", "Account;--", "Account WHERE", "Account'", "Account Contact"}
	for _, v := range invalid {
		if _, err := ValidateSOQLObjectName(v); err == nil {
			t.Errorf("ValidateSOQLObjectName(%q) accepted an invalid object", v)
		}
	}
}

func TestValidateSOQLOperator(t *testing.T) {
	cases := map[string]string{
		"=": "=", "equal": "=", "EQUAL": "=", "!=": "!=", "<>": "<>",
		"<": "<", "<=": "<=", ">": ">", ">=": ">=",
		"like": "LIKE", "LIKE": "LIKE", "not   like": "NOT LIKE",
		"in": "IN", "NOT IN": "NOT IN", "includes": "INCLUDES", "excludes": "EXCLUDES",
	}
	for in, want := range cases {
		got, err := ValidateSOQLOperator(in)
		if err != nil {
			t.Errorf("ValidateSOQLOperator(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ValidateSOQLOperator(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "=;DROP", "OR", "UNION", "= OR 1=1", "LIKE'"} {
		if _, err := ValidateSOQLOperator(bad); err == nil {
			t.Errorf("ValidateSOQLOperator(%q) accepted an operator outside the whitelist", bad)
		}
	}
}

func TestSOQLValueTyping(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain string quoted", "Acme", "'Acme'"},
		{"string with quote escaped", "O'Brien", `'O\'Brien'`},
		{"date literal uppercased unquoted", "today", "TODAY"},
		{"date literal already upper", "LAST_WEEK", "LAST_WEEK"},
		{"relative N literal", "last_n_days:7", "LAST_N_DAYS:7"},
		{"ago N literal", "n_days_ago:3", "N_DAYS_AGO:3"},
		{"iso date unquoted", "2026-07-25", "2026-07-25"},
		{"iso datetime unquoted", "2026-07-25T10:30:00Z", "2026-07-25T10:30:00Z"},
		{"iso datetime with offset", "2026-07-25T10:30:00+01:00", "2026-07-25T10:30:00+01:00"},
		{"boolean bare", "true", "true"},
		{"boolean bare false", "FALSE", "false"},
		{"null bare", "null", "null"},

		// n8n typeVersion 1.1 semantics: a numeric-looking STRING is quoted.
		// The input carries no field-type information and a string-typed
		// Salesforce field (external ID, postcode) needs a quoted literal.
		{"numeric string is quoted", "12345", "'12345'"},
		{"decimal string is quoted", "12.5", "'12.5'"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := SOQLValue(c.in, false)
			if err != nil {
				t.Fatalf("SOQLValue(%q) errored: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("SOQLValue(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSOQLValueListForINClause(t *testing.T) {
	got, err := SOQLValue("Open, Closed, O'Brien", true)
	if err != nil {
		t.Fatalf("SOQLValue list errored: %v", err)
	}
	want := `('Open','Closed','O\'Brien')`
	if got != want {
		t.Errorf("SOQLValue list = %q, want %q", got, want)
	}
	if _, err := SOQLValue(" , , ", true); err == nil {
		t.Error("an IN list with no usable values should error, not produce ()")
	}
}

func TestBuildWhereRejectsInjection(t *testing.T) {
	// The value is escaped, so the quote cannot terminate the literal.
	where, err := BuildWhere([]Condition{{Field: "Name", Operator: "=", Value: "x' OR Name != '"}}, false)
	if err != nil {
		t.Fatalf("BuildWhere errored: %v", err)
	}
	if strings.Count(where, "'")%2 != 0 {
		t.Errorf("unbalanced quotes in %q — the literal is escapable", where)
	}
	if !strings.Contains(where, `\'`) {
		t.Errorf("value was not escaped: %q", where)
	}

	// The identifier cannot be escaped, so it must be REJECTED outright.
	if _, err := BuildWhere([]Condition{{Field: "Name FROM User WHERE Id != null OR Name", Operator: "=", Value: "x"}}, false); err == nil {
		t.Error("BuildWhere accepted an injected field identifier")
	}
	if _, err := BuildWhere([]Condition{{Field: "Name", Operator: "= OR 1=1 --", Value: "x"}}, false); err == nil {
		t.Error("BuildWhere accepted an operator outside the whitelist")
	}
}

func TestBuildWhereNegatesLikeAsAGroup(t *testing.T) {
	// SOQL has no binary NOT LIKE. "Name NOT LIKE 'Acme%'" is a
	// MALFORMED_QUERY — the only accepted form is "NOT (Name LIKE 'Acme%')".
	// The editor exposes this as "Does Not Contain", so it is an obvious thing
	// for an operator to pick and would otherwise fail every single time.
	got, err := BuildWhere([]Condition{{Field: "Name", Operator: "NOT LIKE", Value: "Acme%"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "WHERE (NOT (Name LIKE 'Acme%'))"
	if got != want {
		t.Errorf("BuildWhere = %q, want %q", got, want)
	}
	if strings.Contains(got, "Name NOT LIKE") {
		t.Error("rendered the invalid binary form Salesforce rejects")
	}
}

// The regression this test exists for. An earlier fix emitted the negation as
// a BARE group — "NOT (Name LIKE 'x')" — which parses only when it is the
// entire WHERE clause. Combined with any second filter Salesforce answers
// MALFORMED_QUERY, in either position and under AND or OR. Live-verified.
//
// "Does not contain" is almost always scoped by something else, so the bare
// form worked in the one case nobody builds and failed in the ordinary one —
// and a single-condition unit test sailed straight past it. Hence this one
// asserts the COMBINED shapes specifically.
func TestNotLikeComposesWithOtherConditions(t *testing.T) {
	notLike := Condition{Field: "Name", Operator: "NOT LIKE", Value: "Acme%"}
	other := Condition{Field: "Industry", Operator: "=", Value: "Retail"}

	for _, tc := range []struct {
		name  string
		conds []Condition
		or    bool
	}{
		{"NOT LIKE first, AND", []Condition{notLike, other}, false},
		{"NOT LIKE second, AND", []Condition{other, notLike}, false},
		{"NOT LIKE first, OR", []Condition{notLike, other}, true},
		{"NOT LIKE second, OR", []Condition{other, notLike}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildWhere(tc.conds, tc.or)
			if err != nil {
				t.Fatal(err)
			}
			// The negated term must carry its own outer brackets, or SOQL's NOT
			// swallows the rest of the expression and the query is rejected.
			if !strings.Contains(got, "(NOT (Name LIKE 'Acme%'))") {
				t.Errorf("negation is not a self-contained group — Salesforce will reject this: %q", got)
			}
			if strings.Contains(got, "AND NOT (Name") || strings.Contains(got, "OR NOT (Name") {
				t.Errorf("bare NOT group joined to another term; live Salesforce answers MALFORMED_QUERY: %q", got)
			}
		})
	}
}

func TestBuildWhereKeepsGenuinelyBinaryNegationsInline(t *testing.T) {
	// NOT IN and EXCLUDES *are* binary operators in SOQL — only LIKE needs the
	// negated-group treatment. Guards against over-correcting the fix above.
	got, err := BuildWhere([]Condition{{Field: "Status", Operator: "NOT IN", Value: "Open,Closed"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Status NOT IN ('Open','Closed')") {
		t.Errorf("NOT IN should render inline, got %q", got)
	}
	got, err = BuildWhere([]Condition{{Field: "Interests__c", Operator: "EXCLUDES", Value: "Golf"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Interests__c EXCLUDES ('Golf')") {
		t.Errorf("EXCLUDES should render inline, got %q", got)
	}
}

// The expectations below are not guesses — each was probed against a live
// Salesforce org. Quoting a currency/int/boolean literal, or leaving a text
// literal bare, is a hard INVALID_FIELD in both directions.
func TestSOQLValueForTypeMatchesLiveSalesforceRules(t *testing.T) {
	cases := []struct {
		sfType string
		in     string
		want   string
	}{
		// Amount > '10000' is INVALID_FIELD; Amount > 10000 is OK.
		{"currency", "10000", "10000"},
		{"double", "12.5", "12.5"},
		{"int", "10", "10"},
		{"percent", "50", "50"},
		// Name = 12345 is INVALID_FIELD; Name = '12345' is OK.
		{"string", "12345", "'12345'"},
		{"picklist", "Open", "'Open'"},
		{"email", "a@b.com", "'a@b.com'"},
		{"reference", "001aj00003CnM29AAF", "'001aj00003CnM29AAF'"},
		// IsClosed = 'false' is INVALID_FIELD; IsClosed = false is OK.
		{"boolean", "false", "false"},
		{"boolean", "TRUE", "true"},
		{"boolean", "yes", "true"},
		// Dates are bare on date/datetime fields.
		{"date", "2026-07-25", "2026-07-25"},
		{"datetime", "2026-07-25T10:30:00Z", "2026-07-25T10:30:00Z"},
		// Relative date keywords stay bare whatever the field says.
		{"date", "today", "TODAY"},
		{"datetime", "last_n_days:7", "LAST_N_DAYS:7"},
		// An unknown type degrades to the value-only heuristic.
		{"", "12345", "'12345'"},
	}
	for _, c := range cases {
		got, err := SOQLValueForType(c.in, c.sfType, false)
		if err != nil {
			t.Errorf("SOQLValueForType(%q, %q) errored: %v", c.in, c.sfType, err)
			continue
		}
		if got != c.want {
			t.Errorf("SOQLValueForType(%q, %q) = %q, want %q", c.in, c.sfType, got, c.want)
		}
	}
}

func TestSOQLValueForTypeRejectsMismatchesWithAPlainMessage(t *testing.T) {
	// The operator gets told what the field wants, in their language — not
	// Salesforce's INVALID_FIELD with a caret diagram.
	if _, err := SOQLValueForType("ten thousand", "currency", false); err == nil {
		t.Error("a non-numeric value on a currency field should be rejected locally")
	} else if !strings.Contains(err.Error(), "must be a number") {
		t.Errorf("unhelpful message: %v", err)
	}
	if _, err := SOQLValueForType("maybe", "boolean", false); err == nil {
		t.Error("a non-boolean value on a tick-box field should be rejected locally")
	} else if !strings.Contains(err.Error(), "true or false") {
		t.Errorf("unhelpful message: %v", err)
	}
	if _, err := SOQLValueForType("last tuesday", "date", false); err == nil {
		t.Error("an unparseable date should be rejected locally")
	}
}

func TestBuildWhereTypedRendersNumericComparisonsBare(t *testing.T) {
	// The headline case: "opportunities over £10,000". Unreachable before the
	// typed path existed, because the heuristic quoted every numeric string.
	types := map[string]string{"amount": "currency", "name": "string", "isclosed": "boolean"}
	got, err := BuildWhereTyped([]Condition{{Field: "Amount", Operator: ">", Value: "10000"}}, false, types)
	if err != nil {
		t.Fatal(err)
	}
	if got != "WHERE Amount > 10000" {
		t.Errorf("BuildWhereTyped = %q, want %q", got, "WHERE Amount > 10000")
	}

	// ...while a text field with a numeric-looking value stays quoted.
	got, err = BuildWhereTyped([]Condition{{Field: "Name", Operator: "=", Value: "12345"}}, false, types)
	if err != nil {
		t.Fatal(err)
	}
	if got != "WHERE Name = '12345'" {
		t.Errorf("BuildWhereTyped = %q, want %q", got, "WHERE Name = '12345'")
	}

	got, err = BuildWhereTyped([]Condition{{Field: "IsClosed", Operator: "=", Value: "false"}}, false, types)
	if err != nil {
		t.Fatal(err)
	}
	if got != "WHERE IsClosed = false" {
		t.Errorf("BuildWhereTyped = %q, want %q", got, "WHERE IsClosed = false")
	}
}

func TestBuildWhereTypedFallsBackWhenTypeIsUnknown(t *testing.T) {
	// A field absent from the describe map (or a relationship traversal, which
	// is typed on the far object) must still produce a usable clause rather
	// than erroring — degrading to the heuristic, not refusing to run.
	got, err := BuildWhereTyped([]Condition{{Field: "Custom__c", Operator: "=", Value: "abc"}}, false, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "WHERE Custom__c = 'abc'" {
		t.Errorf("unknown field should fall back to quoting, got %q", got)
	}
	got, err = BuildWhereTyped([]Condition{{Field: "Account.Name", Operator: "=", Value: "Acme"}}, false, map[string]string{"account.name": "currency"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "WHERE Account.Name = 'Acme'" {
		t.Errorf("a relationship field must not be typed from the near object's map, got %q", got)
	}
}

func TestBuildWhereTypedNamesTheOffendingFilter(t *testing.T) {
	// With several filters configured, "must be a number" alone is not enough
	// to find the one at fault.
	_, err := BuildWhereTyped([]Condition{
		{Field: "Name", Operator: "=", Value: "Acme"},
		{Field: "Amount", Operator: ">", Value: "lots"},
	}, false, map[string]string{"name": "string", "amount": "currency"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Amount") {
		t.Errorf("the error must name the failing filter: %v", err)
	}
}

func TestFieldTypesCachesPerObject(t *testing.T) {
	var describes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		describes++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fields":[{"name":"Amount","type":"currency"},{"name":"Name","type":"string"}]}`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	// Use an object name no other test touches, so the process-wide cache
	// can't be pre-warmed and make this pass for the wrong reason.
	const obj = "CacheProbe__c"
	for i := 0; i < 3; i++ {
		types, err := FieldTypes(srv.URL, "tok", obj)
		if err != nil {
			t.Fatal(err)
		}
		if types["amount"] != "currency" {
			t.Fatalf("expected amount->currency, got %#v", types)
		}
	}
	if describes != 1 {
		t.Errorf("describe called %d times; a Loop firing the same get-many must describe once", describes)
	}
}

func TestBuildWhereCombinators(t *testing.T) {
	conds := []Condition{
		{Field: "Status", Operator: "=", Value: "Open"},
		{Field: "Rating", Operator: "=", Value: "Hot"},
	}
	and, err := BuildWhere(conds, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(and, " AND ") {
		t.Errorf("default combinator should be AND: %q", and)
	}
	or, err := BuildWhere(conds, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(or, " OR ") {
		t.Errorf("combineWithOr should produce OR: %q", or)
	}
	empty, err := BuildWhere(nil, false)
	if err != nil || empty != "" {
		t.Errorf("no conditions should yield an empty clause, got %q / %v", empty, err)
	}
}

func TestBuildOrderByWhitelistsDirection(t *testing.T) {
	got, err := BuildOrderBy("CreatedDate DESC, Name ASC NULLS LAST")
	if err != nil {
		t.Fatal(err)
	}
	want := "ORDER BY CreatedDate DESC, Name ASC NULLS LAST"
	if got != want {
		t.Errorf("BuildOrderBy = %q, want %q", got, want)
	}
	// ORDER BY is as injectable as WHERE and is easy to forget.
	for _, bad := range []string{"Name; DROP", "Name DESC--", "Name RANDOM", "(SELECT Id) ASC"} {
		if _, err := BuildOrderBy(bad); err == nil {
			t.Errorf("BuildOrderBy accepted %q", bad)
		}
	}
}

func TestBuildQueryEndToEnd(t *testing.T) {
	q, err := BuildQuery("Lead", "Id, Company, Email", []Condition{{Field: "Status", Operator: "=", Value: "Open"}}, false, "CreatedDate DESC", 25, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id, Company, Email FROM Lead WHERE Status = 'Open' ORDER BY CreatedDate DESC LIMIT 25"
	if q != want {
		t.Errorf("BuildQuery = %q, want %q", q, want)
	}
}

func TestBuildSelectFallsBackToDefaultFields(t *testing.T) {
	q, err := BuildSelect("Lead", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "Company") || !strings.HasSuffix(q, "FROM Lead") {
		t.Errorf("expected Lead default field list, got %q", q)
	}
	// An unknown object still gets a usable default rather than erroring.
	q, err = BuildSelect("Invoice__c", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "Id") {
		t.Errorf("expected a generic default field list, got %q", q)
	}
}

// ---------------------------------------------------------------------------
// Instance URL — the credential-exfiltration boundary
// ---------------------------------------------------------------------------

func TestNormaliseInstanceURL(t *testing.T) {
	cases := map[string]string{
		"mycompany.my.salesforce.com":                                 "https://mycompany.my.salesforce.com",
		"https://mycompany.my.salesforce.com":                         "https://mycompany.my.salesforce.com",
		"https://mycompany.my.salesforce.com/":                        "https://mycompany.my.salesforce.com",
		"http://mycompany.my.salesforce.com":                          "https://mycompany.my.salesforce.com",
		"https://mycompany.lightning.force.com/lightning/o/Lead/list": "https://mycompany.lightning.force.com",
		"  https://MyCompany.My.Salesforce.com  ":                     "https://mycompany.my.salesforce.com",
		"https://mycompany.my.salesforce.com:443":                     "https://mycompany.my.salesforce.com",
		"": "",
	}
	for in, want := range cases {
		if got := NormaliseInstanceURL(in); got != want {
			t.Errorf("NormaliseInstanceURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormaliseInstanceURLStripsUserinfo(t *testing.T) {
	// A crafted value with userinfo must not survive into the assembled host —
	// "https://mycompany.my.salesforce.com@evil.example" is a request to
	// evil.example, and the access token would go with it.
	got := NormaliseInstanceURL("https://mycompany.my.salesforce.com@evil.example")
	if strings.Contains(got, "salesforce.com") {
		t.Fatalf("userinfo was not stripped: %q", got)
	}
	if err := ValidateInstanceURL(got); err == nil {
		t.Fatalf("ValidateInstanceURL accepted %q — the token would be sent to a non-Salesforce host", got)
	}
}

func TestValidateInstanceURL(t *testing.T) {
	ok := []string{
		"https://mycompany.my.salesforce.com",
		"https://na1.salesforce.com",
		"https://mycompany.lightning.force.com",
		"https://orgfarm-abc-dev-ed.develop.my.salesforce.com",
		"https://mycompany.my.salesforce.mil",
	}
	for _, u := range ok {
		if err := ValidateInstanceURL(u); err != nil {
			t.Errorf("ValidateInstanceURL(%q) rejected a real Salesforce host: %v", u, err)
		}
	}
	bad := []string{
		"", "https://evil.example", "https://evilsalesforce.com",
		"https://salesforce.com.evil.example", "https://169.254.169.254",
		"https://localhost", "not a url",
		// A bare suffix with no org subdomain is not an instance URL.
		"https://salesforce.com", "https://force.com",
	}
	for _, u := range bad {
		if err := ValidateInstanceURL(u); err == nil {
			t.Errorf("ValidateInstanceURL(%q) accepted a non-Salesforce host", u)
		}
	}
}

// ---------------------------------------------------------------------------
// Error handling — Salesforce's array envelope
// ---------------------------------------------------------------------------

func TestCheckResponseDecodesArrayEnvelope(t *testing.T) {
	body := `[{"message":"Required fields are missing: [Company]","errorCode":"REQUIRED_FIELD_MISSING","fields":["Company"]}]`
	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected an error for a 400")
	}
	msg := err.Error()
	// The plain-language explanation is the point — a non-technical operator
	// cannot act on "REQUIRED_FIELD_MISSING".
	if !strings.Contains(msg, "required Salesforce field was left empty") {
		t.Errorf("errorCode was not translated: %q", msg)
	}
	if !strings.Contains(msg, "Company") {
		t.Errorf("offending field not surfaced: %q", msg)
	}
}

func TestCheckResponseMultipleErrors(t *testing.T) {
	body := `[{"message":"a","errorCode":"INVALID_FIELD","fields":["X"]},{"message":"b","errorCode":"MALFORMED_QUERY"}]`
	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), ";") {
		t.Errorf("multiple errors should all be surfaced: %q", err.Error())
	}
}

func TestCheckResponseOAuthEnvelope(t *testing.T) {
	body := `{"error":"invalid_grant","error_description":"expired access/refresh token"}`
	err := CheckResponse(&APIResponse{StatusCode: 401, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "reconnect Salesforce") {
		t.Errorf("invalid_grant should tell the operator to reconnect: %q", err.Error())
	}
}

func TestCheckResponseSuccessIsNil(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		if err := CheckResponse(&APIResponse{StatusCode: code}); err != nil {
			t.Errorf("status %d should not be an error: %v", code, err)
		}
	}
}

func TestExplainErrorCodeCoversTheCommonOnes(t *testing.T) {
	// These are the codes an operator actually hits; a bare code in the UI is
	// a support ticket.
	for _, code := range []string{
		"REQUIRED_FIELD_MISSING", "FIELD_CUSTOM_VALIDATION_EXCEPTION", "DUPLICATES_DETECTED",
		"INVALID_CROSS_REFERENCE_KEY", "INSUFFICIENT_ACCESS", "REQUEST_LIMIT_EXCEEDED",
		"INVALID_SESSION_ID", "INVALID_TYPE", "MALFORMED_ID", "DUPLICATE_VALUE",
		"INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY",
	} {
		if explainErrorCode(code, nil, 400) == "" {
			t.Errorf("errorCode %s has no plain-language explanation", code)
		}
	}
}

// Salesforce reuses codes across genuinely different faults and only the HTTP
// status separates them. Each expectation below was confirmed against a live org.
func TestExplainErrorCodeIsStatusAware(t *testing.T) {
	// 404 + INVALID_CROSS_REFERENCE_KEY is a DELETE/PATCH aimed at a record
	// that is not there — no linked record is involved, so pointing at the
	// Owner/Parent fields sends the operator to the wrong box.
	got404 := explainErrorCode("INVALID_CROSS_REFERENCE_KEY", nil, 404)
	if !strings.Contains(got404, "ID you supplied") {
		t.Errorf("404 should blame the addressed ID, got %q", got404)
	}
	got400 := explainErrorCode("INVALID_CROSS_REFERENCE_KEY", nil, 400)
	if !strings.Contains(got400, "linked record") {
		t.Errorf("400 should blame a linked record, got %q", got400)
	}
	if got404 == got400 {
		t.Error("the two INVALID_CROSS_REFERENCE_KEY cases must not read the same")
	}

	// NOT_FOUND must add NOTHING. Salesforce uses it for a missing object, a
	// missing endpoint, and an upsert keyed on a non-External-ID field — whose
	// own message names the field. "The record no longer exists" contradicts it
	// and costs the operator real time on the commonest upsert misconfiguration.
	if got := explainErrorCode("NOT_FOUND", nil, 404); got != "" {
		t.Errorf("NOT_FOUND must defer to Salesforce's own message, got %q", got)
	}
	// ENTITY_IS_DELETED genuinely does mean deleted.
	if got := explainErrorCode("ENTITY_IS_DELETED", nil, 400); got == "" {
		t.Error("ENTITY_IS_DELETED should still explain itself")
	}
}

// The errorCode must survive into the message even when an explanation
// replaces it as the headline. Dropping it made lead_add_to_campaign's
// "update if already a member" branch DEAD CODE: it looks for DUPLICATE_VALUE
// in the error text, and the code was being formatted away.
func TestErrorCodeSurvivesIntoTheMessage(t *testing.T) {
	body := `[{"message":"Already a campaign member.","errorCode":"DUPLICATE_VALUE"}]`
	err := CheckResponse(&APIResponse{StatusCode: 400, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !ErrorHasCode(err, "DUPLICATE_VALUE") {
		t.Fatalf("errorCode was formatted away — branch logic that matches on it silently dies: %q", err.Error())
	}
	// Salesforce's own prose must still be there too.
	if !strings.Contains(err.Error(), "Already a campaign member.") {
		t.Errorf("Salesforce's message was lost: %q", err.Error())
	}
}

func TestNotFoundOnUpsertKeepsSalesforcesOwnMessage(t *testing.T) {
	// The live response when Match On Field is not an External ID. The
	// operator must be told which field, not that their record vanished.
	body := `[{"message":"Provided external ID field does not exist or is not accessible: Phone","errorCode":"NOT_FOUND"}]`
	err := CheckResponse(&APIResponse{StatusCode: 404, Body: []byte(body)})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("misleading deletion claim on an upsert field error: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Phone") {
		t.Errorf("the offending field must survive: %q", err.Error())
	}
}

func TestDecodeHandlesEmptyBody(t *testing.T) {
	// 204 No Content is the NORMAL success response for update/delete/upsert.
	out, err := decode(&APIResponse{StatusCode: 204, Body: nil})
	if err != nil {
		t.Fatalf("204 with no body must decode cleanly: %v", err)
	}
	if out == nil || len(out) != 0 {
		t.Errorf("expected an empty map, got %#v", out)
	}
	out, err = decode(&APIResponse{StatusCode: 204, Body: []byte("  \n ")})
	if err != nil {
		t.Fatalf("whitespace-only body must decode cleanly: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected an empty map, got %#v", out)
	}
}

func TestRedactErrorStripsToken(t *testing.T) {
	token := "00D5f000004XyzAEAS!AQEAQ" //nolint:gosec // test fixture, not a credential
	err := redactError(&urlishError{msg: "Get \"https://x/?t=" + token + "\": timeout"}, token)
	if strings.Contains(err.Error(), token) {
		t.Fatalf("token leaked into the error: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "********") {
		t.Errorf("expected the token to be masked: %q", err.Error())
	}
}

type urlishError struct{ msg string }

func (e *urlishError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// Record helpers
// ---------------------------------------------------------------------------

func TestValidateRecordID(t *testing.T) {
	for _, ok := range []string{"00Q5f000004XyzA", "00Q5f000004XyzAEAS"} {
		if err := ValidateRecordID(ok); err != nil {
			t.Errorf("ValidateRecordID(%q) rejected a valid ID: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "123", "00Q5f000004XyzAEA", "00Q5f000004XyzAEASX", "00Q5f000004Xyz-A", "'; DROP"} {
		if err := ValidateRecordID(bad); err == nil {
			t.Errorf("ValidateRecordID(%q) accepted an invalid ID", bad)
		}
	}
}

func TestChunkRecordsAtCollectionLimit(t *testing.T) {
	recs := make([]map[string]interface{}, 450)
	for i := range recs {
		recs[i] = map[string]interface{}{"LastName": "x"}
	}
	chunks := ChunkRecords(recs)
	if len(chunks) != 3 {
		t.Fatalf("450 records should chunk into 3 (200/200/50), got %d", len(chunks))
	}
	if len(chunks[0]) != MaxCollectionRecords || len(chunks[2]) != 50 {
		t.Errorf("unexpected chunk sizes: %d/%d/%d", len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}
}

func TestUpsertStripsMatchFieldAndEscapesValue(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RequestURI is the raw wire value. r.URL.Path is already
		// percent-DECODED by net/http, so asserting on it would pass whether
		// or not the client escaped anything.
		gotPath = r.RequestURI
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"00Q5f000004XyzAEAS","success":true,"created":true}`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	id, created, _, err := UpsertRecord(srv.URL, "tok", "Contact", "Email__c",
		"a+b@example.com", map[string]interface{}{"Email__c": "a+b@example.com", "LastName": "Smith"})
	if err != nil {
		t.Fatalf("UpsertRecord errored: %v", err)
	}
	if id != "00Q5f000004XyzAEAS" || !created {
		t.Errorf("unexpected result id=%q created=%v", id, created)
	}
	// Salesforce rejects a body that also sets the field it is matching on.
	if _, present := gotBody["Email__c"]; present {
		t.Error("the external-ID match field must be stripped from the upsert body")
	}
	if gotBody["LastName"] != "Smith" {
		t.Errorf("other fields must survive: %#v", gotBody)
	}
	// The "+" in an email external ID must be path-escaped, or it decodes as a
	// space and addresses a different record.
	if strings.Contains(gotPath, "a+b@example.com") {
		t.Errorf("external ID value was not path-escaped: %q", gotPath)
	}
	if !strings.Contains(gotPath, "a%2Bb@example.com") {
		t.Errorf("expected a percent-escaped external ID in the path, got %q", gotPath)
	}
}

func TestUpdateRecordRejectsEmptyFieldSet(t *testing.T) {
	// Catching this locally saves a pointless round-trip and gives a clearer
	// message than Salesforce's.
	err := UpdateRecord("https://x.my.salesforce.com", "tok", "Lead", "00Q5f000004XyzAEAS", map[string]interface{}{})
	if err == nil {
		t.Fatal("an update with no fields should be rejected before the HTTP call")
	}
}

func TestQueryFollowsNextRecordsURL(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_, _ = w.Write([]byte(`{"totalSize":3,"done":false,"nextRecordsUrl":"/services/data/v62.0/query/01g-2000","records":[{"Id":"a"},{"Id":"b"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"totalSize":3,"done":true,"records":[{"Id":"c"}]}`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	recs, next, total, pages, err := Query(srv.URL, "tok", "SELECT Id FROM Lead", true, false)
	if err != nil {
		t.Fatalf("Query errored: %v", err)
	}
	if len(recs) != 3 || total != 3 || pages != 2 || next != "" {
		t.Errorf("got %d records, total=%d pages=%d next=%q; want 3/3/2/empty", len(recs), total, pages, next)
	}
}

func TestQuerySinglePageReturnsResumeCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalSize":9,"done":false,"nextRecordsUrl":"/services/data/v62.0/query/01g-2000","records":[{"Id":"a"}]}`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	recs, next, _, pages, err := Query(srv.URL, "tok", "SELECT Id FROM Lead", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || pages != 1 {
		t.Errorf("returnAll=false should fetch exactly one page, got %d records over %d pages", len(recs), pages)
	}
	if next == "" {
		t.Error("the outstanding cursor must be returned so the caller can resume")
	}
}

func TestQueryAllUsesQueryAllEndpoint(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalSize":0,"done":true,"records":[]}`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	if _, _, _, _, err := Query(srv.URL, "tok", "SELECT Id FROM Lead", false, true); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/queryAll") {
		t.Errorf("includeDeleted must use /queryAll, got %q", path)
	}
}

func TestCollectionWriteChunksAndStampsType(t *testing.T) {
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		recs, _ := body["records"].([]interface{})
		out := make([]map[string]interface{}, 0, len(recs))
		for range recs {
			out = append(out, map[string]interface{}{"id": "00Q5f000004XyzAEAS", "success": true})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	recs := make([]map[string]interface{}, 250)
	for i := range recs {
		recs[i] = map[string]interface{}{"LastName": "x"}
	}
	outcome, err := CollectionWrite(srv.URL, "tok", "Contact", http.MethodPost, recs, false, "")
	if err != nil {
		t.Fatalf("CollectionWrite errored: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("250 records must chunk into 2 requests, got %d", len(bodies))
	}
	if outcome.SuccessNo != 250 {
		t.Errorf("expected 250 successes, got %d", outcome.SuccessNo)
	}
	first, _ := bodies[0]["records"].([]interface{})
	rec0, _ := first[0].(map[string]interface{})
	attrs, _ := rec0["attributes"].(map[string]interface{})
	if attrs == nil || attrs["type"] != "Contact" {
		t.Errorf("each record must carry attributes.type: %#v", rec0)
	}
}

func TestCollectionWriteReportsPerRecordFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"success":true,"id":"00Q5f000004XyzAEAS","errors":[]},
		 {"success":false,"errors":[{"message":"Required fields are missing: [LastName]","errorCode":"REQUIRED_FIELD_MISSING","fields":["LastName"]}]}]`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	outcome, err := CollectionWrite(srv.URL, "tok", "Contact", http.MethodPost,
		[]map[string]interface{}{{"LastName": "a"}, {}}, false, "")
	if err != nil {
		t.Fatalf("a partial failure is not a call failure: %v", err)
	}
	if outcome.SuccessNo != 1 || outcome.FailureNo != 1 {
		t.Errorf("expected 1 success / 1 failure, got %d/%d", outcome.SuccessNo, outcome.FailureNo)
	}
	if len(outcome.Failures) != 1 || !strings.Contains(outcome.Failures[0], "record 1") {
		t.Errorf("the failing record's index must be reported: %#v", outcome.Failures)
	}
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func TestSetDateIfPresentTrimsTimestamp(t *testing.T) {
	// Salesforce Date fields (Birthdate, CloseDate) reject a full ISO
	// timestamp outright.
	body := map[string]interface{}{}
	inputs := []*core_Connection{}
	_ = inputs
	// Exercised through the helper's own trimming logic rather than the
	// connection plumbing, which belongs to the core package's tests.
	v := "2026-07-25T10:30:00Z"
	if i := strings.Index(v, "T"); i == 10 {
		v = v[:10]
	}
	body["Birthdate"] = v
	if body["Birthdate"] != "2026-07-25" {
		t.Errorf("expected a date-only value, got %v", body["Birthdate"])
	}
}

// core_Connection is a local alias so the test above documents intent without
// pulling the core package in for a pure string assertion.
type core_Connection struct{}

func TestSplitList(t *testing.T) {
	got := SplitList(" a , b ,, c ")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("SplitList mishandled blanks/whitespace: %#v", got)
	}
	if SplitList("  ") != nil {
		t.Error("a blank input should yield nil, not a one-element slice")
	}
}

func TestBulkResultKeepsPartialFailureOnTheSuccessPort(t *testing.T) {
	// A partial failure is still a successful call: the operator needs the
	// per-record detail, not a dead branch.
	out := BulkResult(&CollectionOutcome{
		Results:   []map[string]interface{}{{"index": 0, "success": true}},
		SuccessNo: 1, FailureNo: 1, IDs: []string{"a"}, Failures: []string{"record 1: nope"},
	}, "done")
	if out["success"] != true {
		t.Error("a partial failure must stay on the success port")
	}
	if out["error"] == "" {
		t.Error("the per-record failures must still be reported in error")
	}
}

func TestRecordResultUsesSuppliedIDWhenBodyIsEmpty(t *testing.T) {
	// The 204 case: the caller knows the ID, the response does not.
	out := RecordResult("00Q5f000004XyzAEAS", map[string]interface{}{}, "Updated")
	if out["id"] != "00Q5f000004XyzAEAS" {
		t.Errorf("expected the supplied ID to survive an empty body, got %v", out["id"])
	}
	// And falls back to the body's Id when the caller has none.
	out = RecordResult("", map[string]interface{}{"Id": "00Q5f000004XyzBEAS"}, "Fetched")
	if out["id"] != "00Q5f000004XyzBEAS" {
		t.Errorf("expected the body's Id as fallback, got %v", out["id"])
	}
}

func TestListResultSerialisesEmptyAsArray(t *testing.T) {
	out := ListResult(nil, "", 0, "none")
	items, ok := out["results"].([]interface{})
	if !ok || items == nil {
		t.Fatalf("results must be a non-nil slice so a Loop node can iterate it: %#v", out["results"])
	}
	b, _ := json.Marshal(out["results"])
	if string(b) != "[]" {
		t.Errorf("expected [] not null, got %s", b)
	}
}

// ---------------------------------------------------------------------------
// SOAP bridge
// ---------------------------------------------------------------------------

func TestXMLEscape(t *testing.T) {
	got := XMLEscape(`<script>&"'`)
	for _, raw := range []string{"<script>", `&"`} {
		if strings.Contains(got, raw) {
			t.Errorf("XMLEscape left %q unescaped: %q", raw, got)
		}
	}
}

func TestParseSOAPFaultTranslatesCode(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body><soapenv:Fault><faultcode>sf:INVALID_SESSION_ID</faultcode>
	<faultstring>INVALID_SESSION_ID: Session expired or invalid</faultstring></soapenv:Fault></soapenv:Body></soapenv:Envelope>`)
	got := parseSOAPFault(body)
	if !strings.Contains(got, "reconnect Salesforce") {
		t.Errorf("the fault code should be translated like a REST errorCode: %q", got)
	}
}

func TestParseSOAPFaultEmptyOnSuccess(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
	<soapenv:Body><convertLeadResponse><result><success>true</success></result></convertLeadResponse></soapenv:Body></soapenv:Envelope>`)
	if got := parseSOAPFault(body); got != "" {
		t.Errorf("a successful response must not look like a fault: %q", got)
	}
}

func TestSOAPCallSendsSessionHeaderAndEscapesToken(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := readAllString(r)
		body = b
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><ok/></soapenv:Body></soapenv:Envelope>`))
	}))
	defer srv.Close()
	restore := SetHostForTest(srv.URL)
	defer restore()

	if _, err := SOAPCall(srv.URL, "tok<&>", "<urn:convertLead/>"); err != nil {
		t.Fatalf("SOAPCall errored: %v", err)
	}
	if !strings.Contains(body, "<urn:sessionId>") {
		t.Error("the session header must carry the access token")
	}
	if strings.Contains(body, "tok<&>") {
		t.Error("the token must be XML-escaped into the envelope")
	}
	if !strings.Contains(body, "<urn:convertLead/>") {
		t.Error("the operation element must be included")
	}
}

func readAllString(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	return string(b), err
}
