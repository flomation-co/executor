package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// parseExpr is a convenience so the cases below can be written as the Go source
// an action author would actually type.
func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	e, err := parser.ParseExpr(src)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return e
}

func TestStringValue(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
		ok   bool
	}{
		{"plain literal", `"hello"`, "hello", true},

		// The regression this whole change exists for. slack/rich_message wrote
		// its Description exactly like this and the generator silently stored "".
		{"concatenation", `"Send a rich message. " + "Use this when you need formatting."`,
			"Send a rich message. Use this when you need formatting.", true},

		// Concatenation is left-associative, so three or more operands arrive as
		// a nested BinaryExpr — the recursion has to walk it, not just peek at
		// the two immediate operands.
		{"three operands", `"a" + "b" + "c"`, "abc", true},
		{"nested and parenthesised", `("a" + ("b" + "c")) + "d"`, "abcd", true},

		{"escapes survive folding", `"line\n" + "\ttabbed \"quoted\""`, "line\n\ttabbed \"quoted\"", true},
		{"raw string", "`raw`", "raw", true},
		{"raw and interpreted mixed", "`a` + \"b\"", "ab", true},
		{"empty literal is a real value", `""`, "", true},

		// Not resolvable: report that, so a caller can complain rather than
		// silently record "".
		{"int literal is not a string", `42`, "", false},
		{"identifier", `someConst`, "", false},
		{"selector", `core.ActionTypeAction`, "", false},
		{"function call", `strings.Repeat("a", 3)`, "", false},
		{"string plus identifier", `"a" + b`, "", false},
		{"identifier plus string", `a + "b"`, "", false},
		{"non-add operator", `"a" == "b"`, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := stringValue(parseExpr(t, c.src))
			if ok != c.ok {
				t.Fatalf("stringValue(%s) ok = %v, want %v", c.src, ok, c.ok)
			}
			if got != c.want {
				t.Errorf("stringValue(%s) = %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// TestStringValueIsNotFooledByAdjacentTypes pins the distinction that made the
// original bug invisible: a concatenation is a BinaryExpr, NOT a BasicLit. A
// type assertion to *ast.BasicLit — which is what every call site here used to
// do — fails on it, and the old code turned that failure into a `continue`.
func TestStringValueIsNotFooledByAdjacentTypes(t *testing.T) {
	concat := parseExpr(t, `"a" + "b"`)

	if _, isBasicLit := concat.(*ast.BasicLit); isBasicLit {
		t.Fatal(`"a" + "b" parsed as a *ast.BasicLit — the premise of this fix is wrong`)
	}
	if _, isBinary := concat.(*ast.BinaryExpr); !isBinary {
		t.Fatalf(`"a" + "b" is %T, expected *ast.BinaryExpr`, concat)
	}
	if got, ok := stringValue(concat); !ok || got != "ab" {
		t.Errorf("stringValue = %q, %v; want \"ab\", true", got, ok)
	}
}

// TestIntTypeLiteralStillParses guards the one metadata field that is not a
// string. No action writes `Type = 1` today (all 1370 use core.ActionType*),
// but the generator has always accepted it, and quietly dropping a supported
// form is the same class of bug as the one being fixed here.
func TestIntTypeLiteralStillParses(t *testing.T) {
	lit, ok := parseExpr(t, `1`).(*ast.BasicLit)
	if !ok {
		t.Fatal("1 did not parse as a *ast.BasicLit")
	}
	if lit.Kind != token.INT {
		t.Fatalf("kind = %v, want INT", lit.Kind)
	}
	// stringValue must REFUSE it (it is not a string) while leaving it readable
	// as a literal — that pairing is what the call site depends on.
	if _, ok := stringValue(lit); ok {
		t.Error("stringValue accepted an int literal; the Type branch would then read a stale value")
	}
}

// The committed manifest is what the api ingests and the editor renders, so a
// blank string in it is a blank string in front of an operator. These two tests
// read the real thing rather than a fixture: the failure being guarded against
// was invisible precisely because it produced valid-looking output.
const committedManifest = "../../internal/assets/manifest/manifest.json"

type manifestEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Inputs      []struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	} `json:"inputs"`
}

func loadCommittedManifest(t *testing.T) map[string]manifestEntry {
	t.Helper()
	b, err := os.ReadFile(committedManifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]manifestEntry
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("manifest is empty — this test would pass vacuously")
	}
	return m
}

// TestNoActionHasABlankDescription — the description is the sentence under the
// action's name in the palette, and the hint an agent reads when choosing it.
//
// slack/rich_message shipped with this empty because its Description constant
// was written as "..." + "..." and the generator only understood a lone
// literal. Nothing failed; the palette just had a gap.
func TestNoActionHasABlankDescription(t *testing.T) {
	var blank []string
	for id, e := range loadCommittedManifest(t) {
		if strings.TrimSpace(e.Description) == "" {
			blank = append(blank, id)
		}
	}
	sort.Strings(blank)
	if len(blank) > 0 {
		t.Errorf(`%d action(s) have a blank description: %v

If the action's source clearly HAS a Description, the generator failed to read
it rather than the author failing to write it — check that the value is a
string constant stringValue() can fold (cmd/manifest/manifest.go), then re-run
make manifest.`, len(blank), blank)
	}
}

// TestNoInputHasABlankLabel — the label is the field's caption. Blank means the
// operator gets an unlabelled box.
//
// This caught two more victims of the same generator bug that a grep for
// concatenated Description constants missed entirely, because the concatenation
// was inside the Inputs array instead: plan/create's tasks_json and
// slack/rich_message's blocks. Both are the JSON-shaped input of an
// agent-facing action, so the blank label was eating the only description of
// the expected format.
func TestNoInputHasABlankLabel(t *testing.T) {
	var blank []string
	for id, e := range loadCommittedManifest(t) {
		for _, in := range e.Inputs {
			if strings.TrimSpace(in.Label) == "" {
				blank = append(blank, id+"#"+in.Name)
			}
		}
	}
	sort.Strings(blank)
	if len(blank) > 0 {
		t.Errorf("%d input(s) have a blank label: %v\n\nSee the note on TestNoActionHasABlankDescription — a blank here is more often the\ngenerator dropping a value than an author omitting one.", len(blank), blank)
	}
}

// TestDeclaredInputDefaultsReachTheManifest — an input's declared default has to
// survive into the manifest, because that file is what the editor renders.
//
// The generator handled every other Connection field and simply had no case for
// Value, so `Value: true` parsed, built and tested cleanly and then arrived as
// null. The editor drew an unticked box while the action's own label advertised
// a default and its Execute applied one — three things disagreeing with nothing
// to indicate it. This reads the committed manifest rather than a fixture,
// because that mismatch was invisible precisely because the output looked valid.
func TestDeclaredInputDefaultsReachTheManifest(t *testing.T) {
	m := loadCommittedManifestWithValues(t)

	// Representative of each kind that can carry one: a boolean default, and a
	// string default. If the Value case is dropped again, both go null.
	want := []struct {
		action, input string
		value         interface{}
	}{
		{"aws/s3/put_public_access_block", "block_public_acls", true},
		{"crm/apollo/enrichment/people_match", "reveal_personal_emails", true},
		{"ecommerce/woocommerce/product_delete", "force", true},
		{"image/convert", "format", "png"},
		{"graphics/animated_title", "colour", "#ffffff"},
	}

	for _, w := range want {
		e, ok := m[w.action]
		if !ok {
			t.Errorf("%s missing from the manifest", w.action)
			continue
		}
		found := false
		for _, i := range e.Inputs {
			if i.Name != w.input {
				continue
			}
			found = true
			if i.Value != w.value {
				t.Errorf("%s#%s default = %v (%T), want %v — a declared default is not reaching the editor",
					w.action, w.input, i.Value, i.Value, w.value)
			}
		}
		if !found {
			t.Errorf("%s has no input %q", w.action, w.input)
		}
	}
}

// TestBooleanDefaultsMatchTheirExecuteBehaviour guards the direction that
// actually bites: a checkbox that renders one way and behaves another.
//
// Every boolean default in the tree was audited against its Execute when the
// Value case was added. All but the S3 public-access-block flags were already
// applied in Go, so surfacing them changed nothing but the tick; those flags
// were NOT, which meant "Block Public Access" blocked nothing unless an operator
// ticked all four boxes. This asserts the secure default specifically, since
// that is the one where a regression is a security regression.
func TestBooleanDefaultsMatchTheirExecuteBehaviour(t *testing.T) {
	m := loadCommittedManifestWithValues(t)
	for _, action := range []string{"aws/s3/put_public_access_block", "aws/s3/put_account_public_access_block"} {
		e, ok := m[action]
		if !ok {
			t.Fatalf("%s missing from the manifest", action)
		}
		for _, name := range []string{"block_public_acls", "ignore_public_acls", "block_public_policy", "restrict_public_buckets"} {
			var got interface{}
			for _, i := range e.Inputs {
				if i.Name == name {
					got = i.Value
				}
			}
			if got != true {
				t.Errorf("%s#%s default = %v, want true — this action must block public access by default, not silently permit it", action, name, got)
			}
		}
	}
}

type manifestEntryWithValues struct {
	Inputs []struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	} `json:"inputs"`
}

func loadCommittedManifestWithValues(t *testing.T) map[string]manifestEntryWithValues {
	t.Helper()
	b, err := os.ReadFile(committedManifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]manifestEntryWithValues
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("manifest is empty — this test would pass vacuously")
	}
	return m
}
