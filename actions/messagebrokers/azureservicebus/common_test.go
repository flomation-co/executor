package azureservicebus_test

import (
	"context"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestGetAuthConnectionString(t *testing.T) {
	auth, err := sb.GetAuth(authInputs())
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.Method != "connection_string" {
		t.Errorf("Method = %q, want connection_string — it is the default, and the field an operator can actually fill in", auth.Method)
	}
	if auth.Namespace != "myns.servicebus.windows.net" {
		t.Errorf("Namespace = %q, want the bare host lifted out of Endpoint=", auth.Namespace)
	}
	if auth.SharedAccessKey != testKey {
		t.Errorf("SharedAccessKey = %q, want %q — it must be lifted out on its own so it can be redacted", auth.SharedAccessKey, testKey)
	}
	if auth.EntityPath != "" {
		t.Errorf("EntityPath = %q, want empty for a namespace-level policy", auth.EntityPath)
	}
}

// A SharedAccessKey is base64 and routinely ends in '='. Splitting on every
// '=' rather than the first would truncate the key and produce an auth failure
// that looks like a wrong key.
func TestGetAuthKeepsBase64PaddingInTheKey(t *testing.T) {
	auth, err := sb.GetAuth([]*core.Connection{
		{Name: "connection_string", Type: core.ConnectionTypeSecret,
			Value: "Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=Root;SharedAccessKey=abc/def+ghi=="},
	})
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.SharedAccessKey != "abc/def+ghi==" {
		t.Errorf("SharedAccessKey = %q, want the whole base64 value including its = padding", auth.SharedAccessKey)
	}
}

func TestGetAuthRejectsBlankAndMalformedConnectionStrings(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		contains string
	}{
		{"blank", "", "connection_string is required"},
		{"no endpoint", "SharedAccessKeyName=Root;SharedAccessKey=abc", "no Endpoint="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sb.GetAuth([]*core.Connection{
				{Name: "connection_string", Type: core.ConnectionTypeSecret, Value: tc.value},
			})
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("err = %v, want one containing %q", err, tc.contains)
			}
		})
	}
}

// Pasting the namespace with its scheme is the most common Entra
// misconfiguration and the SDK reports it as an opaque dial failure, so it is
// stripped rather than rejected — the portal shows it with the scheme.
func TestGetAuthStripsTheNamespaceScheme(t *testing.T) {
	for _, raw := range []string{
		"myns.servicebus.windows.net",
		"sb://myns.servicebus.windows.net",
		"sb://myns.servicebus.windows.net/",
		"  https://myns.servicebus.windows.net/  ",
		"amqps://myns.servicebus.windows.net",
	} {
		auth, err := sb.GetAuth([]*core.Connection{
			str("auth_method", "entra"),
			str("namespace", raw),
			str("azure_tenant_id", "tenant"),
			str("azure_client_id", "client"),
			{Name: "azure_client_secret", Type: core.ConnectionTypeSecret, Value: "s3cret"},
		})
		if err != nil {
			t.Fatalf("GetAuth(%q): %v", raw, err)
		}
		if auth.Namespace != "myns.servicebus.windows.net" {
			t.Errorf("GetAuth(%q).Namespace = %q, want the bare host", raw, auth.Namespace)
		}
	}
}

func TestGetAuthEntraRequiresEveryCredential(t *testing.T) {
	cases := []struct {
		name     string
		inputs   []*core.Connection
		contains string
	}{
		{"no namespace", []*core.Connection{str("auth_method", "entra")}, "namespace is required"},
		{"namespace with a path", []*core.Connection{
			str("auth_method", "entra"), str("namespace", "myns.servicebus.windows.net/orders"),
		}, "host only"},
		{"no tenant", []*core.Connection{
			str("auth_method", "entra"), str("namespace", "myns.servicebus.windows.net"),
		}, "azure_tenant_id is required"},
		{"no client id", []*core.Connection{
			str("auth_method", "entra"), str("namespace", "myns.servicebus.windows.net"), str("azure_tenant_id", "t"),
		}, "azure_client_id is required"},
		{"no secret", []*core.Connection{
			str("auth_method", "entra"), str("namespace", "myns.servicebus.windows.net"),
			str("azure_tenant_id", "t"), str("azure_client_id", "c"),
		}, "azure_client_secret is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := sb.GetAuth(tc.inputs); err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("err = %v, want one containing %q", err, tc.contains)
			}
		})
	}
}

func TestGetAuthRejectsAnUnknownMethod(t *testing.T) {
	_, err := sb.GetAuth([]*core.Connection{str("auth_method", "managed_identity")})
	if err == nil || !strings.Contains(err.Error(), "auth_method must be") {
		t.Fatalf("err = %v, want a named auth_method failure — managed identity is deliberately unsupported", err)
	}
}

// The EntityPath gotcha, config-time half: a queue-scoped connection string
// cannot drive the management API at all, and the runtime failure is an
// unrelated-looking 401.
func TestEntityScopedConnectionStringIsRejectedForManagement(t *testing.T) {
	auth, err := sb.GetAuth(entityScopedAuthInputs("orders"))
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if auth.EntityPath != "orders" {
		t.Fatalf("EntityPath = %q, want orders", auth.EntityPath)
	}
	err = auth.RequireNamespaceScope()
	if err == nil {
		t.Fatal("RequireNamespaceScope accepted an entity-scoped connection string")
	}
	for _, want := range []string{"orders", "EntityPath", "RootManageSharedAccessKey"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q — the operator needs to know which entity and what to paste instead", err, want)
		}
	}
}

func TestEntityScopedConnectionStringReachesOnlyItsOwnEntity(t *testing.T) {
	auth, err := sb.GetAuth(entityScopedAuthInputs("orders"))
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if err := auth.RequireEntityScope("orders"); err != nil {
		t.Errorf("RequireEntityScope(orders) = %v, want nil — the string is scoped to exactly this entity", err)
	}
	if err := auth.RequireEntityScope("ORDERS"); err != nil {
		t.Errorf("RequireEntityScope is case-sensitive; Service Bus entity names are not")
	}
	err = auth.RequireEntityScope("invoices")
	if err == nil || !strings.Contains(err.Error(), "invoices") || !strings.Contains(err.Error(), "orders") {
		t.Fatalf("err = %v, want one naming both the target and the scope", err)
	}
}

func TestNamespaceScopedConnectionStringReachesEverything(t *testing.T) {
	auth, err := sb.GetAuth(authInputs())
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if err := auth.RequireNamespaceScope(); err != nil {
		t.Errorf("RequireNamespaceScope = %v, want nil", err)
	}
	if err := auth.RequireEntityScope("anything"); err != nil {
		t.Errorf("RequireEntityScope = %v, want nil", err)
	}
}

// The connection string carries the key inline, so an unredacted SDK error is
// a credential in the flow's error output.
func TestRedactScrubsEveryCredential(t *testing.T) {
	auth := sb.Auth{
		ConnectionString: testConnString,
		SharedAccessKey:  testKey,
		ClientSecret:     "s3cret",
	}
	msg := sb.Redact(auth, "failed to dial "+testConnString+" with key "+testKey+" or secret s3cret")
	for _, leaked := range []string{testConnString, testKey, "s3cret"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("Redact left %q in %q", leaked, msg)
		}
	}
	if !strings.Contains(msg, "********") {
		t.Errorf("Redact produced %q, expected the redaction marker", msg)
	}
}

// A correct Entra credential with no data-plane role fails as
// CodeUnauthorizedAccess and looks exactly like a bad secret — operators
// rotate a working secret over this. The role names have to be in the text.
func TestFriendlyErrorNamesTheServiceBusDataRoles(t *testing.T) {
	auth := sb.Auth{Method: "entra"}
	msg := sb.FriendlyError(auth, serviceBusError(azservicebus.CodeUnauthorizedAccess))
	for _, want := range []string{
		"Azure Service Bus Data Sender",
		"Azure Service Bus Data Receiver",
		"Azure Service Bus Data Owner",
		"Subscription Owner or Contributor is NOT enough",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("FriendlyError does not mention %q:\n%s", want, msg)
		}
	}
}

// Under a shared access policy the same code means something else entirely —
// there are no RBAC roles to assign, so naming them would be a wild goose chase.
func TestFriendlyErrorTalksAboutPolicyClaimsForAConnectionString(t *testing.T) {
	auth := sb.Auth{Method: "connection_string"}
	msg := sb.FriendlyError(auth, serviceBusError(azservicebus.CodeUnauthorizedAccess))
	if strings.Contains(msg, "Azure Service Bus Data Sender") {
		t.Errorf("FriendlyError offered RBAC advice to a shared-access-key user:\n%s", msg)
	}
	if !strings.Contains(msg, "Send/Listen/Manage") {
		t.Errorf("FriendlyError does not mention the policy claims:\n%s", msg)
	}
}

func TestFriendlyErrorExplainsTheOtherCodes(t *testing.T) {
	auth := sb.Auth{Method: "connection_string"}
	cases := []struct {
		code     azservicebus.Code
		contains string
	}{
		{azservicebus.CodeNotFound, "$deadletterqueue"},
		{azservicebus.CodeLockLost, "cannot exceed 5 minutes"},
		{azservicebus.CodeTimeout, "no session was available"},
	}
	for _, tc := range cases {
		msg := sb.FriendlyError(auth, serviceBusError(tc.code))
		if !strings.Contains(msg, tc.contains) {
			t.Errorf("FriendlyError(%s) does not mention %q:\n%s", tc.code, tc.contains, msg)
		}
	}
}

func TestFriendlyErrorPassesThroughAPlainError(t *testing.T) {
	auth := sb.Auth{Method: "connection_string", SharedAccessKey: testKey}
	msg := sb.FriendlyError(auth, context.DeadlineExceeded)
	if msg != context.DeadlineExceeded.Error() {
		t.Errorf("FriendlyError mangled a non-SDK error: %q", msg)
	}
	if sb.FriendlyError(auth, nil) != "" {
		t.Error("FriendlyError(nil) should be empty")
	}
}

// The one that matters most: an elapsed Max Wait on a quiet queue arrives as
// the context deadline, because the SDK only swallows errors when it has
// messages to hand back. Reporting it would make every polling flow look broken.
func TestReceiveWithinTreatsAnElapsedWaitAsNoMessages(t *testing.T) {
	r := &stubReceiver{receiveErr: context.DeadlineExceeded}
	msgs, err := sb.ReceiveWithin(context.Background(), r, 1, time.Millisecond)
	if err != nil {
		t.Fatalf("ReceiveWithin = %v, want a nil error — an idle queue is not a failure", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("got %d messages, want none", len(msgs))
	}
}

// ...but a cancelled FLOW is a real cancellation and must surface, or a
// stopped run would silently look like an empty queue.
func TestReceiveWithinSurfacesACancelledFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &stubReceiver{receiveErr: context.DeadlineExceeded}
	if _, err := sb.ReceiveWithin(ctx, r, 1, time.Minute); err == nil {
		t.Fatal("ReceiveWithin swallowed a cancelled flow's error")
	}
}

func TestReceiveWithinSurfacesARealError(t *testing.T) {
	r := &stubReceiver{receiveErr: serviceBusError(azservicebus.CodeUnauthorizedAccess)}
	if _, err := sb.ReceiveWithin(context.Background(), r, 1, time.Minute); err == nil {
		t.Fatal("ReceiveWithin swallowed an authorisation failure")
	}
}

// ReceiveAndDelete messages have no lock, and the SDK rejects settling them
// outright — so settlement is skipped, not attempted and reported.
func TestSettleSkipsReceiveAndDeleteMessages(t *testing.T) {
	r := &stubReceiver{}
	msgs := []*azservicebus.ReceivedMessage{receivedMessage("m1", "hi")}
	err := sb.Settle(context.Background(), r, msgs, azservicebus.ReceiveModeReceiveAndDelete, sb.DispositionComplete, "", "")
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if len(r.settled) != 0 {
		t.Fatalf("Settle called %s on a receive-and-delete message, which the SDK rejects", r.settled[0].action)
	}
}

func TestSettleAppliesEveryDisposition(t *testing.T) {
	cases := []struct {
		disposition sb.Disposition
		want        string
	}{
		{sb.DispositionComplete, "complete"},
		{sb.DispositionAbandon, "abandon"},
		{sb.DispositionDefer, "defer"},
		{sb.DispositionDeadLetter, "dead_letter"},
	}
	for _, tc := range cases {
		t.Run(string(tc.disposition), func(t *testing.T) {
			r := &stubReceiver{}
			msgs := []*azservicebus.ReceivedMessage{receivedMessage("m1", "hi"), receivedMessage("m2", "there")}
			if err := sb.Settle(context.Background(), r, msgs, azservicebus.ReceiveModePeekLock, tc.disposition, "why", "detail"); err != nil {
				t.Fatalf("Settle: %v", err)
			}
			if len(r.settled) != 2 {
				t.Fatalf("settled %d message(s), want 2 — every message received must be settled", len(r.settled))
			}
			for _, s := range r.settled {
				if s.action != tc.want {
					t.Errorf("settled with %q, want %q", s.action, tc.want)
				}
			}
			if tc.disposition == sb.DispositionDeadLetter {
				if r.settled[0].reason != "why" || r.settled[0].descr != "detail" {
					t.Errorf("dead-letter reason/description = %q/%q, want why/detail — they are the diagnostic payload", r.settled[0].reason, r.settled[0].descr)
				}
			}
		})
	}
}

func TestSettleReportsWhichMessageFailed(t *testing.T) {
	r := &stubReceiver{settleErr: serviceBusError(azservicebus.CodeLockLost)}
	msgs := []*azservicebus.ReceivedMessage{receivedMessage("m1", "hi")}
	err := sb.Settle(context.Background(), r, msgs, azservicebus.ReceiveModePeekLock, sb.DispositionComplete, "", "")
	if err == nil || !strings.Contains(err.Error(), "m1") {
		t.Fatalf("err = %v, want one naming the message that could not be settled", err)
	}
}

func TestParseSequenceNumbers(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []int64
	}{
		{"json array as scheduled outputs it", "[12,13]", []int64{12, 13}},
		{"comma separated as a human types it", "12, 13", []int64{12, 13}},
		{"single", "12", []int64{12}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sb.ParseSequenceNumbers(tc.raw)
			if err != nil {
				t.Fatalf("ParseSequenceNumbers(%q): %v", tc.raw, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestParseSequenceNumbersRejectsRubbish(t *testing.T) {
	for _, raw := range []string{"", "   ", "abc", "[1,\"two\"]", "[]", ","} {
		if _, err := sb.ParseSequenceNumbers(raw); err == nil {
			t.Errorf("ParseSequenceNumbers(%q) accepted it", raw)
		}
	}
}

// The management plane takes ISO-8601 durations as strings; an operator gives
// us seconds.
func TestISODuration(t *testing.T) {
	cases := []struct {
		seconds int
		want    string
	}{
		{0, "PT0S"},
		{30, "PT30S"},
		{60, "PT1M"},
		{90, "PT1M30S"},
		{3600, "PT1H"},
		{3661, "PT1H1M1S"},
		{86400, "P1D"},
		{90061, "P1DT1H1M1S"},
	}
	for _, tc := range cases {
		if got := sb.ISODuration(tc.seconds); got != tc.want {
			t.Errorf("ISODuration(%d) = %q, want %q", tc.seconds, got, tc.want)
		}
	}
}

func TestClampMaxMessages(t *testing.T) {
	cases := []struct {
		name   string
		inputs []*core.Connection
		want   int
	}{
		{"unset", nil, 1},
		{"zero", []*core.Connection{integer("max_messages", 0)}, 1},
		{"negative", []*core.Connection{integer("max_messages", -5)}, 1},
		{"in range", []*core.Connection{integer("max_messages", 10)}, 10},
		{"over the cap", []*core.Connection{integer("max_messages", 5000)}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sb.ClampMaxMessages(tc.inputs); got != tc.want {
				t.Errorf("ClampMaxMessages = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestClampMaxWait(t *testing.T) {
	if got := sb.ClampMaxWait(nil); got != 10*time.Second {
		t.Errorf("ClampMaxWait(unset) = %v, want 10s", got)
	}
	if got := sb.ClampMaxWait([]*core.Connection{integer("max_wait_seconds", 9999)}); got != 300*time.Second {
		t.Errorf("ClampMaxWait(9999) = %v, want the 300s cap", got)
	}
}

func TestReceiveModeFrom(t *testing.T) {
	// Blank must mean peek-lock: receive-and-delete destroys the message
	// before the flow sees it, so it can never be the accidental default.
	if got, _ := sb.ReceiveModeFrom(nil); got != azservicebus.ReceiveModePeekLock {
		t.Error("an unset receive_mode must default to peek-lock, not the destructive mode")
	}
	if got, _ := sb.ReceiveModeFrom([]*core.Connection{str("receive_mode", "receive_and_delete")}); got != azservicebus.ReceiveModeReceiveAndDelete {
		t.Error("receive_and_delete did not select ReceiveModeReceiveAndDelete")
	}
	if _, err := sb.ReceiveModeFrom([]*core.Connection{str("receive_mode", "nonsense")}); err == nil {
		t.Error("an unknown receive_mode was accepted")
	}
}

func TestDispositionFrom(t *testing.T) {
	if got, _ := sb.DispositionFrom(nil); got != sb.DispositionComplete {
		t.Errorf("an unset disposition = %q, want complete", got)
	}
	for _, v := range []string{"complete", "abandon", "dead_letter", "defer"} {
		if got, err := sb.DispositionFrom([]*core.Connection{str("disposition", v)}); err != nil || string(got) != v {
			t.Errorf("DispositionFrom(%q) = %q, %v", v, got, err)
		}
	}
	if _, err := sb.DispositionFrom([]*core.Connection{str("disposition", "reject")}); err == nil {
		t.Error("an unknown disposition was accepted")
	}
}

func TestBuildMessage(t *testing.T) {
	msg, err := sb.BuildMessage([]*core.Connection{
		text("body", `{"order":1}`),
		str("message_id", "m-1"),
		str("session_id", "s-1"),
		str("correlation_id", "c-1"),
		str("subject", "order.created"),
		str("content_type", "application/json"),
		str("reply_to", "replies"),
		str("to", "somewhere"),
		str("partition_key", "p-1"),
		integer("time_to_live_seconds", 120),
		str("scheduled_enqueue_time", "2026-07-17T09:30:00Z"),
		obj("application_properties", `{"tenant":"acme"}`),
	})
	if err != nil {
		t.Fatalf("BuildMessage: %v", err)
	}
	if string(msg.Body) != `{"order":1}` {
		t.Errorf("Body = %q", msg.Body)
	}
	if *msg.MessageID != "m-1" || *msg.SessionID != "s-1" || *msg.Subject != "order.created" {
		t.Errorf("broker properties were not carried through: %+v", msg)
	}
	if *msg.TimeToLive != 2*time.Minute {
		t.Errorf("TimeToLive = %v, want 2m", *msg.TimeToLive)
	}
	if msg.ScheduledEnqueueTime.UTC().Format(time.RFC3339) != "2026-07-17T09:30:00Z" {
		t.Errorf("ScheduledEnqueueTime = %v", msg.ScheduledEnqueueTime)
	}
	if msg.ApplicationProperties["tenant"] != "acme" {
		t.Errorf("ApplicationProperties = %v", msg.ApplicationProperties)
	}
}

// A body is the operator's bytes. Trimming it would silently corrupt a payload
// whose whitespace matters, so only the blank check trims.
func TestBuildMessageDoesNotTrimTheBody(t *testing.T) {
	msg, err := sb.BuildMessage([]*core.Connection{text("body", "  padded  ")})
	if err != nil {
		t.Fatalf("BuildMessage: %v", err)
	}
	if string(msg.Body) != "  padded  " {
		t.Errorf("Body = %q, want the bytes as given", msg.Body)
	}
}

func TestBuildMessageRejectsAnEmptyBody(t *testing.T) {
	for _, body := range []string{"", "   "} {
		if _, err := sb.BuildMessage([]*core.Connection{text("body", body)}); err == nil {
			t.Errorf("BuildMessage accepted body %q", body)
		}
	}
}

func TestBuildMessageRejectsBadOptionalInputs(t *testing.T) {
	cases := []struct {
		name     string
		inputs   []*core.Connection
		contains string
	}{
		{"bad timestamp", []*core.Connection{text("body", "x"), str("scheduled_enqueue_time", "next tuesday")}, "RFC3339"},
		{"bad json", []*core.Connection{text("body", "x"), obj("application_properties", "{oops")}, "valid JSON"},
		{"non-object properties", []*core.Connection{text("body", "x"), obj("application_properties", `["a"]`)}, "JSON object"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := sb.BuildMessage(tc.inputs); err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("err = %v, want one containing %q", err, tc.contains)
			}
		})
	}
}

func TestMessageFromJSON(t *testing.T) {
	msg, err := sb.MessageFromJSON(0, map[string]interface{}{
		"body":                   "hello",
		"message_id":             "m-1",
		"subject":                "greeting",
		"application_properties": map[string]interface{}{"k": "v"},
	})
	if err != nil {
		t.Fatalf("MessageFromJSON: %v", err)
	}
	if string(msg.Body) != "hello" || *msg.MessageID != "m-1" || *msg.Subject != "greeting" {
		t.Errorf("message = %+v", msg)
	}
	if msg.ApplicationProperties["k"] != "v" {
		t.Errorf("ApplicationProperties = %v", msg.ApplicationProperties)
	}
}

// A bare string is what a Loop building a batch produces, and an object body
// is what a flow carrying structured data produces. Both are friendlier to
// accept than to reject.
func TestMessageFromJSONAcceptsAStringOrAnObjectBody(t *testing.T) {
	msg, err := sb.MessageFromJSON(0, "just a body")
	if err != nil {
		t.Fatalf("MessageFromJSON(string): %v", err)
	}
	if string(msg.Body) != "just a body" {
		t.Errorf("Body = %q", msg.Body)
	}

	msg, err = sb.MessageFromJSON(0, map[string]interface{}{"body": map[string]interface{}{"order": float64(7)}})
	if err != nil {
		t.Fatalf("MessageFromJSON(object body): %v", err)
	}
	if string(msg.Body) != `{"order":7}` {
		t.Errorf("Body = %q, want the object re-encoded", msg.Body)
	}
}

func TestMessageFromJSONNamesTheOffendingIndex(t *testing.T) {
	cases := []interface{}{
		map[string]interface{}{"subject": "no body here"},
		map[string]interface{}{"body": "x", "message_id": 7},
		map[string]interface{}{"body": "x", "application_properties": "not an object"},
		42,
	}
	for _, raw := range cases {
		_, err := sb.MessageFromJSON(3, raw)
		if err == nil {
			t.Fatalf("MessageFromJSON accepted %v", raw)
		}
		if !strings.Contains(err.Error(), "messages[3]") {
			t.Errorf("error %q does not name the index — the operator would have to bisect the array by hand", err)
		}
	}
}

func TestMessageOutputSurfacesTheRedeliveryFacts(t *testing.T) {
	m := receivedMessage("m-1", `{"order":1}`)
	m.DeliveryCount = 4
	m.DeadLetterReason = ptr("MaxDeliveryCountExceeded")
	m.DeadLetterErrorDescription = ptr("gave up after 10 tries")
	m.State = azservicebus.MessageStateDeferred

	out := sb.MessageOutput(m, true)

	// DeliveryCount is how a flow detects a poison message; LockedUntil is its
	// settlement budget; the dead-letter fields are the whole point of the DLQ
	// actions.
	if out["delivery_count"] != 4 {
		t.Errorf("delivery_count = %v, want 4", out["delivery_count"])
	}
	if out["locked_until"] != "2026-07-17T09:01:00Z" {
		t.Errorf("locked_until = %v", out["locked_until"])
	}
	if out["dead_letter_reason"] != "MaxDeliveryCountExceeded" || out["dead_letter_error_description"] != "gave up after 10 tries" {
		t.Errorf("dead-letter diagnostics missing: %v", out)
	}
	if out["state"] != "deferred" {
		t.Errorf("state = %v, want deferred", out["state"])
	}
	if out["sequence_number"] != int64(42) {
		t.Errorf("sequence_number = %v — without it a deferred message can never be retrieved", out["sequence_number"])
	}
	if out["body"] != `{"order":1}` {
		t.Errorf("body = %v, want the raw bytes", out["body"])
	}
	body, ok := out["body_json"].(map[string]interface{})
	if !ok || body["order"] != float64(1) {
		t.Errorf("body_json = %v, want the decoded body", out["body_json"])
	}
}

func TestMessageOutputLeavesUnparseableBodiesAlone(t *testing.T) {
	out := sb.MessageOutput(receivedMessage("m-1", "not json"), true)
	if _, present := out["body_json"]; present {
		t.Error("body_json was set for a body that is not JSON — parse_json is best-effort, not a validator")
	}
	if out["body"] != "not json" {
		t.Errorf("body = %v", out["body"])
	}

	out = sb.MessageOutput(receivedMessage("m-1", `{"a":1}`), false)
	if _, present := out["body_json"]; present {
		t.Error("body_json was set with parse_json off")
	}
}

// An empty receive must be data, not an error: a quiet queue is the ordinary
// state of a queue, and an error port would make every polling flow look broken.
func TestMessagesResultReportsEmptyAsData(t *testing.T) {
	out := sb.MessagesResult(nil, "Received", "queue orders")
	if out["success"] != true {
		t.Error("an empty receive was reported as a failure")
	}
	if out["received"] != false {
		t.Errorf("received = %v, want false", out["received"])
	}
	if out["count"] != 0 {
		t.Errorf("count = %v, want 0", out["count"])
	}
	if items, ok := out["results"].([]interface{}); !ok || items == nil {
		t.Error("results must be a non-nil empty slice — nil serialises as null and breaks a downstream Loop")
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "No messages") {
		t.Errorf("tool_result = %q, want it to say plainly that nothing arrived", summary)
	}

	out = sb.MessagesResult([]interface{}{map[string]interface{}{}}, "Received", "queue orders")
	if out["received"] != true {
		t.Errorf("received = %v, want true", out["received"])
	}
}
