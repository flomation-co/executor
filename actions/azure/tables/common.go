// Package tables holds the shared client construction, auth, entity shaping
// and error mapping used by every azure/tables/* action.
//
// Unlike azure/storage and azure/cosmosdb — which hand-roll REST — this
// package drives the official SDK, github.com/Azure/azure-sdk-for-go/sdk/data/
// aztables. The honest reason is NOT "Tables needs a second string-to-sign":
// aztables signs with SharedKeyLite (two lines: x-ms-date + canonicalized
// resource, and the canonicalized resource takes only ?comp=, not every sorted
// query param), which is a THIRD scheme distinct from the full SharedKey our
// Blob signer implements. Hand-rolling would mean a third signer variant, not
// a second, and reusing storage.canonicalizedResource would silently produce a
// wrong signature and a bare 403.
//
// The real reasons are that the dependency cost is nil (aztables pulls only
// azcore and friends, all already vendored) and that two pieces of the
// protocol are genuinely fiddly:
//
//   - Batch is multipart/mixed nesting a second multipart/mixed changeset,
//     each part a serialised HTTP request, demultiplexed back into per-op
//     results. That is where the bugs would live.
//   - The EDM type system: Edm.Int64 travels as a STRING with a sibling
//     "Prop@odata.type":"Edm.Int64" sidecar, and Edm.DateTime/Binary/Guid
//     marshal both ways. A plain JSON number silently becomes Edm.Int32 or
//     Edm.Double, so a big integer loses precision.
//
// Three SDK behaviours this package exists to contain:
//
//   - aztables PANICS on operator-supplied JSON. UpdateEntity/UpsertEntity do
//     an unchecked type assertion on the PartitionKey/RowKey it finds in the
//     entity map (client.go:333, `pk.(string)`), so a missing key or a numeric
//     one — {"PartitionKey":42} — crashes the executor rather than erroring.
//     The doc comment claims an error is returned; the code panics. Our entity
//     JSON comes straight from operator input and flow variables, so ParseEntity
//     validates presence AND string-ness before any call can reach the SDK.
//   - The SDK defaults IfMatch to "*" when nil, i.e. unconditional
//     last-write-wins. The safe-looking default is the unsafe one, so the
//     etag input's placeholder says what blank means.
//   - odata.* keys pollute every entity read, and odata.metadata echoes the
//     ENDPOINT URL into flow output. ShapeEntity lifts the etag out and drops
//     the rest.
//
// The auth block mirrors azure/storage's field names so an operator moving
// between the two sees the same fields, plus connection_string, which aztables
// makes cheap and which is the one-paste option the portal actually hands you.
package tables

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	// EntraScope is the client-credentials scope for the Table service. The
	// audience is https://storage.azure.com for every cloud, public and
	// sovereign alike (aztables hardcodes the same value) — only Cosmos table
	// endpoints differ, and those are out of scope here.
	EntraScope = "https://storage.azure.com/.default"

	// requestTimeout bounds a single Table service call.
	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound $top. The service caps a page at
	// 1000 entities regardless of what is asked for.
	DefaultPageLimit = 50
	MaxPageLimit     = 1000

	// maxListPages bounds a return_all continuation walk so a table with tens
	// of millions of rows can never spin unbounded requests. $top is PER PAGE,
	// not a total, so without this a return_all walk over a large table never
	// ends. At 1000 entities a page this still admits 200k rows.
	maxListPages = 200

	// MaxBatchEntities is the service's hard cap on one transaction.
	MaxBatchEntities = 100

	// MaxAccessPolicies is the service's cap on stored access policies per
	// table. aztables rejects a sixth locally; we say so first, by name.
	MaxAccessPolicies = 5
)

// nowFunc is the clock for SAS start/expiry defaults; a var so the SAS test
// can pin it.
var nowFunc = time.Now

// httpClient is shared across every Tables action so TLS connections to the
// account endpoint are pooled rather than re-dialled per call.
// insecureHTTPClient is the same but skips TLS verification, used only when
// the action opts in via allow_insecure — a separate client so the secure
// default cannot be weakened by a per-request tweak.
//
// Neither is ever handed to the Entra token exchange: azidentity owns its own
// pipeline (see actions/azure/common.go), which is what keeps an operator's
// allow_insecure checkbox from silently disabling certificate verification on
// the credential exchange to Microsoft.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

var insecureHTTPClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — opt-in only, for self-signed custom endpoints
	},
}

// Auth method values for the auth_method dropdown. Empty reads as shared_key
// so a fresh node with the dropdown untouched authenticates with the key the
// operator just pasted.
const (
	AuthSharedKey        = "shared_key"
	AuthConnectionString = "connection_string"
	AuthEntra            = "entra"
)

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals, so it cannot see through a shared variable), but every action puts
// these nine first, in this order — tables_inputs_drift_test.go enforces it.
//
// The names deliberately match azure/storage's: an operator who has already
// connected Blob Storage is looking at the same account, the same access keys
// and the same service principal, and should not have to re-learn the form.
// connection_string is the one addition — aztables parses it for free and the
// portal hands it over as a single string, which for a non-technical operator
// beats hunting down an account name and a key separately.
//
// The names are also reserved: core.FindConnection returns the FIRST input
// whose name matches, and the credential block is declared first, so a
// resource field reusing one of these names would silently read the credential.
var AuthInputs = []core.Connection{
	{
		Name:        "account_name",
		Type:        core.ConnectionTypeString,
		Label:       "Storage Account",
		Placeholder: "mystorageaccount",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthSharedKey, AuthEntra}},
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Shared Key", Value: AuthSharedKey},
			{Name: "Connection String", Value: AuthConnectionString},
			{Name: "Microsoft Entra (service principal)", Value: AuthEntra},
		},
	},
	{
		Name:        "account_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Account Key",
		Placeholder: "Base64 account key — Storage Account ▸ Access keys",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthSharedKey}},
	},
	{
		Name:        "connection_string",
		Type:        core.ConnectionTypeSecret,
		Label:       "Connection String",
		Placeholder: "DefaultEndpointsProtocol=https;AccountName=…;AccountKey=…;EndpointSuffix=core.windows.net — Storage Account ▸ Access keys ▸ Connection string",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthConnectionString}},
	},
	{
		Name:        "azure_tenant_id",
		Type:        core.ConnectionTypeString,
		Label:       "Tenant ID",
		Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "azure_client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Application (client) ID of the service principal",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "azure_client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "The app needs a Storage Table Data role on the account (RBAC)",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthEntra}},
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://myaccount.table.core.windows.net — leave blank to derive; Azurite: http://host:10002/devstoreaccount1",
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure TLS",
		Placeholder: "Skip TLS verification — only for custom endpoints with a self-signed certificate",
	},
}

// Auth is the resolved connection: the account name, the chosen method with
// its credentials, and the normalised service URL (scheme + host [+ account
// path for Azurite-style endpoints], no trailing slash, no table name).
type Auth struct {
	AccountName  string
	Method       string
	AccountKey   string // base64, as pasted or as parsed out of a connection string; empty under Entra
	TenantID     string
	ClientID     string
	ClientSecret string
	ServiceURL   string
	Insecure     bool

	// connString is retained solely for redaction — it embeds the account key.
	connString string
}

// accountNameRe is the Azure storage-account charset (lowercase alphanumeric,
// 3-24 chars — Azurite's devstoreaccount1 fits). Enforced because the name is
// interpolated into a host and signed into the canonicalized resource.
var accountNameRe = regexp.MustCompile(`^[a-z0-9]{3,24}$`)

// GetAuth resolves the credential block. Errors are operator-configuration
// problems, so callers surface them as soft failures (ErrorResult).
//
// All three methods converge on the same two facts — a service URL and a
// credential — rather than branching into three client constructors. In
// particular the connection string is parsed here instead of being handed to
// aztables.NewServiceClientFromConnectionString, so that the `endpoint`
// override means exactly the same thing under every method. Letting the SDK
// derive its own URL for one method and not the others is the kind of
// inconsistency that only shows up against a custom endpoint, i.e. Azurite.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	a := Auth{
		Method:   OptionalString("auth_method", inputs),
		Insecure: OptionalBool("allow_insecure", inputs),
	}
	if a.Method == "" {
		a.Method = AuthSharedKey
	}

	// endpointFromConnString is only consulted when the operator did not set
	// an explicit endpoint.
	endpointFromConnString := ""

	switch a.Method {
	case AuthSharedKey:
		account, err := requiredAccountName(inputs)
		if err != nil {
			return Auth{}, err
		}
		a.AccountName = account
		if a.AccountKey, err = RequiredString("account_key", inputs); err != nil {
			return Auth{}, err
		}
	case AuthConnectionString:
		raw, err := RequiredString("connection_string", inputs)
		if err != nil {
			return Auth{}, err
		}
		a.connString = raw
		parsed, err := parseConnectionString(raw)
		if err != nil {
			return Auth{}, err
		}
		a.AccountName, a.AccountKey, endpointFromConnString = parsed.account, parsed.key, parsed.tableEndpoint
	case AuthEntra:
		account, err := requiredAccountName(inputs)
		if err != nil {
			return Auth{}, err
		}
		a.AccountName = account
		if a.TenantID, err = RequiredString("azure_tenant_id", inputs); err != nil {
			return Auth{}, err
		}
		if a.ClientID, err = RequiredString("azure_client_id", inputs); err != nil {
			return Auth{}, err
		}
		if a.ClientSecret, err = RequiredString("azure_client_secret", inputs); err != nil {
			return Auth{}, err
		}
	default:
		return Auth{}, fmt.Errorf("auth_method %q is not supported (use shared_key, connection_string or entra)", a.Method)
	}

	endpoint := OptionalString("endpoint", inputs)
	if endpoint == "" {
		endpoint = endpointFromConnString
	}
	switch {
	case endpoint != "":
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Auth{}, fmt.Errorf("endpoint must be an http(s) URL, e.g. https://myaccount.table.core.windows.net")
		}
		a.ServiceURL = strings.TrimRight(endpoint, "/")
	default:
		a.ServiceURL = "https://" + a.AccountName + ".table.core.windows.net"
	}
	return a, nil
}

func requiredAccountName(inputs []*core.Connection) (string, error) {
	account, err := RequiredString("account_name", inputs)
	if err != nil {
		return "", err
	}
	account = strings.ToLower(account)
	if !accountNameRe.MatchString(account) {
		return "", fmt.Errorf("account_name %q is not a valid storage account name (3-24 lowercase letters and digits)", account)
	}
	return account, nil
}

// connStringParts is what a Table connection string is worth to us: who we
// are, how we sign, and (for the emulator) where to send it.
type connStringParts struct {
	account       string
	key           string
	tableEndpoint string
}

// parseConnectionString reads the portal's key=value;… string.
//
// Only the AccountKey form is supported. A SharedAccessSignature connection
// string would need the no-credential client and cannot generate a SAS or be
// account-key-redacted the same way; the portal's own "Connection string"
// field always carries AccountKey, so the SAS form is refused by name rather
// than half-supported.
func parseConnectionString(raw string) (connStringParts, error) {
	fields := map[string]string{}
	for _, part := range strings.Split(strings.TrimRight(strings.TrimSpace(raw), ";"), ";") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return connStringParts{}, fmt.Errorf("connection_string is malformed — expected key=value pairs separated by semicolons, as shown under Storage Account ▸ Access keys")
		}
		fields[strings.TrimSpace(kv[0])] = kv[1]
	}

	p := connStringParts{
		account:       strings.ToLower(strings.TrimSpace(fields["AccountName"])),
		key:           strings.TrimSpace(fields["AccountKey"]),
		tableEndpoint: strings.TrimRight(strings.TrimSpace(fields["TableEndpoint"]), "/"),
	}
	if p.account == "" {
		return connStringParts{}, fmt.Errorf("connection_string has no AccountName — paste the whole string from Storage Account ▸ Access keys ▸ Connection string")
	}
	if p.key == "" {
		if fields["SharedAccessSignature"] != "" {
			return connStringParts{}, fmt.Errorf("connection_string is a SAS connection string; this node needs one containing an AccountKey (Storage Account ▸ Access keys ▸ Connection string)")
		}
		return connStringParts{}, fmt.Errorf("connection_string has no AccountKey — paste the whole string from Storage Account ▸ Access keys ▸ Connection string")
	}
	if !accountNameRe.MatchString(p.account) {
		return connStringParts{}, fmt.Errorf("connection_string's AccountName %q is not a valid storage account name", p.account)
	}
	if p.tableEndpoint == "" {
		// The dev/Azurite string carries TableEndpoint explicitly; a public
		// cloud one carries a suffix instead.
		if suffix := strings.TrimSpace(fields["EndpointSuffix"]); suffix != "" {
			scheme := strings.TrimSpace(fields["DefaultEndpointsProtocol"])
			if scheme == "" {
				scheme = "https"
			}
			p.tableEndpoint = scheme + "://" + p.account + ".table." + suffix
		}
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// Clients
// ---------------------------------------------------------------------------

// entraCredential adapts the shared azidentity-backed token mint
// (actions/azure/common.go) to the azcore.TokenCredential interface aztables
// wants. The mint exports a token STRING, not a credential object, and this
// package must not reach past it into azidentity — the whole point of that
// file is that the credential exchange owns its own pipeline and cannot be
// handed an insecure HTTP client.
//
// ExpiresOn is deliberately pessimistic. The mint does not report the real
// expiry, so claiming one would be a guess that could hand aztables a token
// already dead. A one-minute floor makes azcore re-ask often, which costs
// nothing: ClientCredentialsToken lands in azidentity's own per-scope token
// cache, so a re-ask is a map lookup, not a round trip to Microsoft.
type entraCredential struct {
	tenantID     string
	clientID     string
	clientSecret string
}

const entraTokenFloor = time.Minute

func (c entraCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	scope := EntraScope
	if len(opts.Scopes) > 0 && opts.Scopes[0] != "" {
		scope = opts.Scopes[0]
	}
	token, err := azure.ClientCredentialsToken(ctx, c.tenantID, c.clientID, c.clientSecret, scope)
	if err != nil {
		// ClientCredentialsToken already redacts the secret; Auth.redact
		// scrubs again on the way into ErrorResult.
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{Token: token, ExpiresOn: nowFunc().Add(entraTokenFloor)}, nil
}

func transportFor(a Auth) policy.Transporter {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
}

func clientOptions(a Auth) *aztables.ClientOptions {
	return &aztables.ClientOptions{ClientOptions: azcore.ClientOptions{Transport: transportFor(a)}}
}

// ServiceClient builds the account-level client used by the table_* actions.
func ServiceClient(a Auth) (*aztables.ServiceClient, error) {
	switch a.Method {
	case AuthEntra:
		cred := entraCredential{tenantID: a.TenantID, clientID: a.ClientID, clientSecret: a.ClientSecret}
		client, err := aztables.NewServiceClient(a.ServiceURL, cred, clientOptions(a))
		if err != nil {
			return nil, fmt.Errorf("failed to build the Table service client: %s", a.redact(err.Error()))
		}
		return client, nil
	default:
		cred, err := a.SharedKeyCredential()
		if err != nil {
			return nil, err
		}
		client, err := aztables.NewServiceClientWithSharedKey(a.ServiceURL, cred, clientOptions(a))
		if err != nil {
			return nil, fmt.Errorf("failed to build the Table service client: %s", a.redact(err.Error()))
		}
		return client, nil
	}
}

// TableClient builds a client affinitised to one table, validating the name
// first so a bad one fails by rule rather than as an opaque 400.
//
// It goes through ServiceClient.NewClient rather than aztables.NewClient: that
// constructor splits the table name off the END of the URL path, which is
// ambiguous against the Azurite path style (http://host:10002/devstoreaccount1),
// where the account is already a path segment.
func TableClient(a Auth, table string) (*aztables.Client, error) {
	if err := ValidateTableName(table); err != nil {
		return nil, err
	}
	svc, err := ServiceClient(a)
	if err != nil {
		return nil, err
	}
	return svc.NewClient(table), nil
}

// SharedKeyCredential returns the signing credential, or an error naming the
// reason there isn't one. SAS generation and every non-Entra request need it.
func (a Auth) SharedKeyCredential() (*aztables.SharedKeyCredential, error) {
	if a.AccountKey == "" {
		return nil, fmt.Errorf("this operation needs the account key — set Authentication to Shared Key or Connection String (a Microsoft Entra service principal cannot sign it)")
	}
	cred, err := aztables.NewSharedKeyCredential(a.AccountName, a.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("account_key is not valid base64 — paste the key exactly as shown under Access keys")
	}
	return cred, nil
}

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// Context is the request context every action passes to the SDK.
func Context(flow *core.Flow) context.Context { return reqContext(flow) }

// ---------------------------------------------------------------------------
// Redaction
// ---------------------------------------------------------------------------

// sasSigRe matches a SAS signature query value so a URL echoed into an error
// never leaks it. azcore's ResponseError already strips the query string from
// the URL it prints — this covers everything else that might carry one.
var sasSigRe = regexp.MustCompile(`(sig=)[^&\s"']+`)

// redact scrubs this connection's own secrets plus the generic patterns. Every
// error string that could carry transport detail passes through here before it
// reaches an output.
//
// The connection string is masked as a whole AND via its parsed key: the whole
// string is what an operator pasted and what a parse error would echo back,
// while the key is what the SDK might surface on its own.
func (a Auth) redact(msg string) string {
	if a.connString != "" {
		msg = azure.RedactSecret(msg, a.connString)
	}
	if a.AccountKey != "" {
		msg = azure.RedactSecret(msg, a.AccountKey)
	}
	if a.ClientSecret != "" {
		msg = azure.RedactSecret(msg, a.ClientSecret)
	}
	return sasSigRe.ReplaceAllString(msg, "${1}REDACTED")
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ErrorCode returns the Table service error code carried by an SDK error, or
// "" when the failure was not an HTTP response (transport, context, local
// validation). azcore populates it from the x-ms-error-code header when the
// service sends one and otherwise unwraps the {"odata.error":{"code":…}} body.
func ErrorCode(err error) string {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.ErrorCode
	}
	return ""
}

// StatusCode returns the HTTP status carried by an SDK error, or 0.
func StatusCode(err error) int {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode
	}
	return 0
}

// Errorf renders an SDK error into one operator-facing line: the service's own
// code plus a plain-language hint for the failures an operator will actually
// hit, redacted, and truncated so a raw azcore dump (which restates the whole
// request and response) cannot flood the run record.
func (a Auth) Errorf(err error) string {
	code, status := ErrorCode(err), StatusCode(err)
	if code == "" {
		msg := a.redact(err.Error())
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Sprintf("Azure Table Storage request failed: %s", msg)
	}
	return fmt.Sprintf("Azure Table Storage error (%d %s): %s", status, code, friendlyError(code))
}

// friendlyError maps the service's error codes to what an operator should do
// about them. The codes not listed here are self-explanatory in context; these
// are the ones whose raw text explains nothing, or explains the wrong thing.
func friendlyError(code string) string {
	// A failed transaction reports its code prefixed with the index of the
	// operation that broke it ("1:EntityAlreadyExists"). Strip the index —
	// BatchErrorf says which change that was, in rows rather than offsets —
	// so the code itself still maps.
	if _, rest, split := strings.Cut(code, ":"); split {
		code = rest
	}
	switch code {
	case "TableAlreadyExists":
		return "a table with that name already exists in this account"
	case "TableNotFound":
		return "no table with that name exists in this account — table names are case-preserving but case-insensitive for uniqueness"
	case "TableBeingDeleted":
		return "that table was just deleted and the name stays reserved while the service reclaims it (up to about 40 seconds) — wait, then retry"
	case "EntityAlreadyExists":
		return "an entity with that PartitionKey and RowKey already exists — use Upsert Row to insert-or-update instead"
	case "ResourceNotFound", "EntityNotFound":
		return "no entity with that PartitionKey and RowKey exists in this table"
	case "UpdateConditionNotSatisfied":
		return "the entity was modified since it was read — the ETag no longer matches. Re-read the entity and retry, or clear the ETag field to overwrite unconditionally"
	case "InvalidInput":
		return "the service rejected the entity — entities are FLAT (no nested objects or arrays; JSON-stringify instead), max 255 properties, 1 MB per entity, 32 KB per string property"
	case "PropertiesNeedValue":
		return "an entity property has no value — Table Storage cannot store a null; omit the property instead"
	case "AuthenticationFailed":
		return "the account name or key was rejected — check they are from the same storage account and the key is the base64 Value, not its name"
	case "AuthorizationPermissionMismatch":
		return "the service principal authenticated but has no data-plane role on this account — grant Storage Table Data Contributor (or Reader for read-only). Subscription Owner is NOT enough"
	case "InvalidAuthenticationInfo":
		return "the request signature was rejected — if a custom endpoint is set, check it points at the Table service (port 10002 on Azurite, not 10000)"
	case "OutOfRangeInput":
		return "a value is out of range — PartitionKey/RowKey are 1 KB each at most, and table names are 3-63 alphanumeric characters starting with a letter"
	}
	return code
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// tableNameRe is the Table service's name rule: alphanumeric ONLY (no hyphens
// — unlike blob container names, which is the mistake an operator arriving
// from Blob Storage will make), 3-63 characters, not starting with a digit.
var tableNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{2,62}$`)

// ValidateTableName mirrors the service rule client-side so the error names it
// rather than arriving as a bare 400.
func ValidateTableName(name string) error {
	if !tableNameRe.MatchString(name) {
		return fmt.Errorf("table name %q is invalid — 3-63 letters and digits only, starting with a letter (no hyphens or underscores, unlike blob containers)", name)
	}
	return nil
}

// ValidateKey enforces the PartitionKey/RowKey rules: 1 KB max, and none of
// the characters the service reserves for its own URL routing.
func ValidateKey(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty — PartitionKey and RowKey together are the entity's identity", field)
	}
	if len(value) > 1024 {
		return fmt.Errorf("%s must be 1 KB or less (got %d bytes)", field, len(value))
	}
	if strings.ContainsAny(value, `/\#?`) {
		return fmt.Errorf(`%s must not contain / \ # or ?`, field)
	}
	for _, r := range value {
		if r < 0x20 || (r >= 0x7F && r <= 0x9F) {
			return fmt.Errorf("%s must not contain control characters", field)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Entities
// ---------------------------------------------------------------------------

// ParseEntity reads an entity-object input and enforces every rule the SDK
// does not.
//
// This is the panic guard. aztables' UpdateEntity/UpsertEntity assert
// mapEntity["PartitionKey"].(string) with no comma-ok, so an entity missing
// either key, or carrying a numeric one, takes down the executor process
// rather than returning an error. Every action that hands an entity to the SDK
// goes through here first.
//
// The returned bytes are the re-marshalled map, not the operator's original
// text: the SDK parses the bytes itself, and re-marshalling guarantees what we
// validated is what it sees.
func ParseEntity(inputs []*core.Connection, name string) ([]byte, map[string]interface{}, error) {
	value, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, nil, err
	}
	if value == nil {
		return nil, nil, fmt.Errorf("%s is required", name)
	}
	entity, ok := value.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf(`%s must be a JSON object, e.g. {"PartitionKey":"orders","RowKey":"1001","Total":42}`, name)
	}
	if _, _, err := EntityKeys(entity); err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, nil, fmt.Errorf("%s could not be encoded: %w", name, err)
	}
	return raw, entity, nil
}

// EntityKeys extracts and validates the composite identity from an entity map.
func EntityKeys(entity map[string]interface{}) (partitionKey, rowKey string, err error) {
	partitionKey, err = entityKey(entity, "PartitionKey")
	if err != nil {
		return "", "", err
	}
	rowKey, err = entityKey(entity, "RowKey")
	if err != nil {
		return "", "", err
	}
	return partitionKey, rowKey, nil
}

func entityKey(entity map[string]interface{}, field string) (string, error) {
	raw, present := entity[field]
	if !present {
		return "", fmt.Errorf("the entity has no %s — PartitionKey and RowKey together are the entity's identity, and both are required", field)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("the entity's %s must be a string, not %s — Table Storage keys are always strings (quote it: %q)",
			field, jsonKind(raw), fmt.Sprintf("%v", raw))
	}
	if err := ValidateKey(field, s); err != nil {
		return "", err
	}
	return s, nil
}

// jsonKind names what an operator actually passed, for an error that tells
// them which field to quote.
func jsonKind(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case []interface{}:
		return "an array"
	case map[string]interface{}:
		return "an object"
	}
	return "a non-string value"
}

// odataNoise is the metadata the service inlines with the operator's own
// properties. odata.metadata is the sharp one: it echoes the ENDPOINT URL into
// flow output, where it is persisted in the run record and forwarded to every
// downstream node.
var odataNoise = map[string]bool{
	"odata.metadata": true,
	"odata.id":       true,
	"odata.type":     true,
	"odata.editLink": true,
}

// ShapeEntity turns a raw entity body into the operator-facing object: the
// etag lifted to a plain "etag" key (it is what entity_update/entity_delete
// take for optimistic concurrency, so it must be reachable from a flow
// expression), the odata.* noise dropped, and everything else — including the
// service-managed Timestamp — left alone.
//
// The "Prop@odata.type" sidecars are deliberately KEPT. They are how an
// Edm.Int64/DateTime/Binary/Guid round-trips: strip them and an entity read by
// entity_get and written back by entity_upsert silently changes type.
func ShapeEntity(raw []byte, etag string) (map[string]interface{}, error) {
	entity := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entity); err != nil {
			return nil, fmt.Errorf("failed to parse the Table service entity response: %w", err)
		}
	}
	if etag == "" {
		if v, ok := entity["odata.etag"].(string); ok {
			etag = v
		}
	}
	delete(entity, "odata.etag")
	for key := range entity {
		if odataNoise[key] {
			delete(entity, key)
		}
	}
	if etag != "" {
		entity["etag"] = etag
	}
	return entity, nil
}

// EntityID is the flow-facing identity of an entity: the two keys joined, so a
// downstream node has one value to carry rather than two.
func EntityID(partitionKey, rowKey string) string {
	return partitionKey + "/" + rowKey
}

// ETagOption converts an optional operator-supplied etag into the SDK's
// IfMatch.
//
// nil means "*" to the SDK, i.e. unconditional last-write-wins — so a blank
// field is the LAST-WRITE-WINS default, not the safe one. That is the SDK's
// choice and we keep it (failing every unconditional write would be worse),
// but it is why the etag input's placeholder spells out what blank means.
func ETagOption(inputs []*core.Connection, name string) *azcore.ETag {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	etag := azcore.ETag(raw)
	return &etag
}

// UpdateModeFor reads the merge/replace dropdown.
//
// Merge is the default and the blank value maps to it. Replace DELETES every
// property the supplied entity does not mention, silently and with no warning
// — upserting {PartitionKey, RowKey, Qty} over an entity with four other
// properties leaves only Qty. It is the single easiest way to lose data here,
// which is why the option labels say what each one does rather than naming the
// mode, and why blank cannot mean replace.
func UpdateModeFor(inputs []*core.Connection) (aztables.UpdateMode, error) {
	switch OptionalString("update_mode", inputs) {
	case "", string(aztables.UpdateModeMerge):
		return aztables.UpdateModeMerge, nil
	case string(aztables.UpdateModeReplace):
		return aztables.UpdateModeReplace, nil
	default:
		return "", fmt.Errorf("update_mode must be merge or replace")
	}
}

// UpdateModeInput is the shared merge/replace dropdown. Actions re-declare it
// literally (the manifest generator AST-parses the Inputs literal); the drift
// test compares every copy against this one.
var UpdateModeInput = core.Connection{
	Name:  "update_mode",
	Type:  core.ConnectionTypeString,
	Label: "Update Mode",
	Options: []core.ConnectionOption{
		{Name: "Merge — only change the fields you supply", Value: "merge"},
		{Name: "Replace — delete any field you do not supply", Value: "replace"},
	},
}

// ---------------------------------------------------------------------------
// Paging
// ---------------------------------------------------------------------------

// PageLimit reads the limit input, clamped to the service's 1-1000 page cap.
func PageLimit(inputs []*core.Connection) int32 {
	limit, set := OptionalInt("limit", inputs)
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return int32(limit)
}

// ListSummary phrases the standard list tool_result. capped means a return_all
// walk stopped at the maxListPages safety cap with pages still remaining.
func ListSummary(noun string, count int, returnAll, capped bool) string {
	if returnAll && capped {
		return fmt.Sprintf("Fetched %d %s(s); stopped at the %d-page safety cap — narrow the filter to get the rest", count, noun, maxListPages)
	}
	if returnAll {
		return fmt.Sprintf("Fetched all %d %s(s)", count, noun)
	}
	return fmt.Sprintf("Found %d %s(s)", count, noun)
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object-typed input into an arbitrary value. Returns
// (nil, nil) when absent/blank, (nil, err) on malformed JSON.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	switch v := conn.Value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("%s must be valid JSON: %w", name, err)
		}
		return out, nil
	default:
		return conn.Value, nil
	}
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-object response into the standard output.
func ResourceResult(id string, result map[string]interface{}, summary string) map[string]interface{} {
	if result == nil {
		result = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      result,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output. A
// non-nil empty slice serialises as [] not null (get-many feeds Loop nodes).
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}
