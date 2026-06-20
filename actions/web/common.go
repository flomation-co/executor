package web_common

// Shared helpers for the web/* action category — primarily plumbing
// that needs to know "is this request targeting the Flomation API"
// before adding service-to-service headers.

import (
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
)

// ParentExecutionHeader is the name of the X-Flomation-Parent-Execution-ID
// header that links a newly-triggered execution to the one that fired
// the HTTP request. The API trusts it only on the internal mTLS engine,
// so external destinations never see it — see InjectParentHeaderIfInternal.
const ParentExecutionHeader = "X-Flomation-Parent-Execution-ID"

// InjectParentHeaderIfInternal adds the parent-execution header to req
// only when the request URL targets the API the executor was launched
// against. The host comparison is exact and case-insensitive on the
// hostname portion; scheme and port differences are tolerated because
// the same logical API may serve public TLS and internal mTLS on
// different ports.
//
// External URLs (third-party APIs, arbitrary fetch targets) are left
// alone — leaking an internal execution ID to a third party would be
// a privacy regression. The function is a no-op when:
//
//   - ctx is nil (action ran outside an execution, e.g. unit tests)
//   - ctx.APIURL is empty (runner didn't pass it through)
//   - ctx.ExecutionID is empty (we have nothing to link to)
//   - the destination host doesn't match the API host
func InjectParentHeaderIfInternal(req *http.Request, ctx *core.ExecutionContext) {
	if req == nil || ctx == nil || ctx.APIURL == "" || ctx.ExecutionID == "" {
		return
	}
	apiHost, err := hostnameOf(ctx.APIURL)
	if err != nil || apiHost == "" {
		return
	}
	reqHost := strings.ToLower(req.URL.Hostname())
	if reqHost == "" || reqHost != apiHost {
		return
	}
	req.Header.Set(ParentExecutionHeader, ctx.ExecutionID)
}

// hostnameOf extracts the lowercase hostname portion of a URL,
// excluding the port. Returns the empty string for inputs the stdlib
// can't parse so the caller treats them as "no match".
func hostnameOf(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Hostname()), nil
}
