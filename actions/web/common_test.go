package web_common

// InjectParentHeaderIfInternal is a tiny security boundary: it has to
// add the header for genuine API callbacks and never add it for any
// other destination. The cost of a false positive (leaking an internal
// execution ID to a third-party host) is high enough that every shape
// of input is worth covering explicitly.

import (
	"net/http"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func mustRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("building request for %q: %v", target, err)
	}
	return req
}

func TestInjectParentHeader_TableDriven(t *testing.T) {
	RegisterTestingT(t)

	type tc struct {
		name        string
		targetURL   string
		ctx         *core.ExecutionContext
		expectValue string // "" means: header must NOT be set
	}

	const execID = "exec-abc-123"
	cases := []tc{
		{
			name:      "nil context — never inject",
			targetURL: "http://api.flomation.local/api/v1/flo/x/trigger/y/execute",
			ctx:       nil,
		},
		{
			name:      "empty APIURL — never inject",
			targetURL: "http://api.flomation.local/whatever",
			ctx:       &core.ExecutionContext{ExecutionID: execID},
		},
		{
			name:      "empty execution ID — nothing to link",
			targetURL: "http://api.flomation.local/whatever",
			ctx:       &core.ExecutionContext{APIURL: "http://api.flomation.local"},
		},
		{
			name:      "external URL — third-party host must not receive the header",
			targetURL: "https://httpbin.org/post",
			ctx: &core.ExecutionContext{
				APIURL:      "http://api.flomation.local",
				ExecutionID: execID,
			},
		},
		{
			name:      "internal URL — exact host match injects",
			targetURL: "http://api.flomation.local/api/v1/flo/x/trigger/y/execute",
			ctx: &core.ExecutionContext{
				APIURL:      "http://api.flomation.local",
				ExecutionID: execID,
			},
			expectValue: execID,
		},
		{
			name:      "same host, different port — still considered internal",
			targetURL: "https://api.flomation.local:9081/api/v1/internal/flo/x/trigger/y/execute",
			ctx: &core.ExecutionContext{
				APIURL:      "http://api.flomation.local:9080",
				ExecutionID: execID,
			},
			expectValue: execID,
		},
		{
			name:      "case-insensitive host match",
			targetURL: "http://API.Flomation.Local/api/v1/flo/x/trigger/y/execute",
			ctx: &core.ExecutionContext{
				APIURL:      "http://api.flomation.local",
				ExecutionID: execID,
			},
			expectValue: execID,
		},
		{
			name:      "malformed APIURL — no panic, no header",
			targetURL: "http://api.flomation.local/whatever",
			ctx: &core.ExecutionContext{
				APIURL:      "://not-a-url",
				ExecutionID: execID,
			},
		},
		{
			name:      "host-shadow attempt — only suffix match on internal host must not leak",
			targetURL: "http://api.flomation.local.evil.com/exfil",
			ctx: &core.ExecutionContext{
				APIURL:      "http://api.flomation.local",
				ExecutionID: execID,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			RegisterTestingT(t)
			req := mustRequest(t, c.targetURL)
			InjectParentHeaderIfInternal(req, c.ctx)
			got := req.Header.Get(ParentExecutionHeader)
			Expect(got).To(Equal(c.expectValue),
				"target %q with ctx %+v", c.targetURL, c.ctx)
		})
	}
}

// TestInjectParentHeader_NilRequest is a separate test because the nil
// branch is genuinely a panic-or-not question — table-driven coverage
// would have to construct a sentinel value to represent "no request",
// which obscures the actual contract: "doesn't panic on nil".
func TestInjectParentHeader_NilRequest(t *testing.T) {
	RegisterTestingT(t)
	Expect(func() {
		InjectParentHeaderIfInternal(nil, &core.ExecutionContext{
			APIURL:      "http://api.flomation.local",
			ExecutionID: "exec-1",
		})
	}).NotTo(Panic())
}
