// Package azureservicebus holds what every messagebrokers/azureservicebus/*
// action shares: the credential block, the two clients Service Bus needs, the
// message shaping, and the error mapping.
//
// Five things shape this file.
//
//   - THERE ARE TWO CLIENTS, not one, and they speak different protocols.
//     azservicebus.Client is AMQP 1.0 (send/receive/peek/schedule);
//     admin.Client is ATOM/XML over HTTPS (create/list/delete entities). They
//     take the same credentials and nothing else. The admin plane is rate
//     limited far more aggressively than the data plane, which is why a live
//     dropdown backed by ListQueues must be cached by the caller rather than
//     called per keystroke.
//
//   - THE SDK IS NOT OPTIONAL. Service Bus's REST surface has no sessions, no
//     batch, no ReceiveDeferredMessages and no scheduled-cancel, and AMQP 1.0
//     is not something to hand-roll. What that costs us is testability: the
//     SDK ships no fake, so the concrete clients sit behind the Sender /
//     Receiver / Admin interfaces below and the factories are vars. Nothing
//     else in this package touches azservicebus.Client directly.
//
//   - SETTLEMENT CANNOT BE A DOWNSTREAM NODE. A lock token belongs to the AMQP
//     connection that took it: the SDK settles on the receiver's link, or on a
//     management link that it hands receiver.LinkName() to, and that link must
//     live on the same connection. Worse, when a receiver's link closes the
//     broker releases its unsettled peek-lock messages IMMEDIATELY rather than
//     holding them until LockedUntil — so by the time a second one-shot
//     executor action started, the message would already be someone else's.
//     Hence Disposition is a parameter of the receive, and the only sanctioned
//     cross-flow handoff is defer, whose sequence numbers are durable.
//
//   - ENTRA IS azidentity, NOT THE SHARED MINT. actions/azure's
//     ClientCredentialsToken returns a token string; azservicebus wants an
//     azcore.TokenCredential because it refreshes on its own schedule over a
//     link that outlives any one token. Wrapping a string in a static
//     credential would hand the SDK a token it cannot renew, which fails as a
//     mid-receive auth error rather than anything legible. The field NAMES
//     (azure_tenant_id / azure_client_id / azure_client_secret) are the part
//     that is shared, per the spec.
//
//   - EVERY ERROR IS REDACTED. The connection string carries SharedAccessKey
//     inline, so an unredacted SDK error is a key in the flow's error output.
//     Redact is applied at the one place errors become strings (Fail).
package azureservicebus

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

const (
	// DefaultMaxMessages / MaxMaxMessages bound the maxMessages argument to
	// ReceiveMessages. The SDK has no PrefetchCount (a real divergence from
	// .NET/Java — do not add a prefetch input, it does not exist here), so
	// this is the only flow-control knob there is.
	DefaultMaxMessages = 1
	MaxMaxMessages     = 100

	// DefaultMaxWaitSeconds / MaxMaxWaitSeconds bound how long a receive
	// blocks. ReceiveMessages is a long poll: it returns as soon as it has
	// ANY message, so this is a ceiling on an empty queue, not a delay.
	DefaultMaxWaitSeconds = 10
	MaxMaxWaitSeconds     = 300

	// DefaultPageLimit / MaxPageLimit bound an admin list.
	DefaultPageLimit = 50
	MaxPageLimit     = 100

	// MaxAllPages bounds a Return All walk so a namespace with thousands of
	// entities cannot spin unbounded requests against the rate-limited
	// management plane.
	MaxAllPages = 100

	// adminTimeout caps a management-plane call. The data plane is bounded by
	// the flow context and the caller's Max Wait instead.
	adminTimeout = 60 * time.Second

	// StandardMaxMessageBytes is the Standard-tier message cap. It covers
	// headers AND application properties AND body, not just the body, so a
	// 250KB payload with fat properties still fails. Premium raises it to
	// 100MB, which is why namespace_get reports the SKU. We do not enforce
	// this client-side — MessageBatch.AddMessage does it exactly, and a
	// single send gets a clear broker error — it is here for the messages.
	StandardMaxMessageBytes = 256 * 1024
)

// AuthInputs is the canonical credential block. Every action re-declares these
// six fields inline and first, in this order, because the manifest generator
// AST-parses the Inputs literal and cannot see through a package-level
// variable. This var is therefore documentation; the enforcement is
// azureservicebus_inputs_drift_test.go.
//
// Connection string leads because it is the one an operator can actually
// obtain: one paste from the portal's Shared access policies blade, no app
// registration and no consent. Entra is the least-privilege answer, not the
// first-run answer.
var AuthInputs = []core.Connection{
	{
		Name:        "connection_string",
		Type:        core.ConnectionTypeSecret,
		Label:       "Connection String",
		Placeholder: "Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=… — the NAMESPACE-level policy, not a queue's",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connection_string"}},
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Connection String", Value: "connection_string"},
			{Name: "Microsoft Entra (service principal)", Value: "entra"},
		},
	},
	{
		Name:        "namespace",
		Type:        core.ConnectionTypeString,
		Label:       "Namespace",
		Placeholder: "myns.servicebus.windows.net — the host only, no sb:// prefix",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}},
	},
	{
		Name:        "azure_tenant_id",
		Type:        core.ConnectionTypeString,
		Label:       "Tenant ID",
		Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}},
	},
	{
		Name:        "azure_client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Application (client) ID of the service principal",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}},
	},
	{
		Name:        "azure_client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "The app needs an Azure Service Bus Data role on the namespace — subscription Owner is not enough",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"entra"}},
	},
}

// Auth is the resolved credential plus what parsing the connection string told
// us. EntityPath is non-empty when the operator pasted an entity-scoped policy
// — see RequireNamespaceScope.
type Auth struct {
	Method           string
	ConnectionString string
	Namespace        string
	TenantID         string
	ClientID         string
	ClientSecret     string

	// SharedAccessKey is lifted out of the connection string so it can be
	// redacted on its own: the SDK's errors quote the key, not the whole
	// string it came from.
	SharedAccessKey string

	// EntityPath is the ;EntityPath=<name> suffix a queue- or topic-level
	// policy carries. Empty for a namespace-level policy.
	EntityPath string
}

// GetAuth resolves and validates the credential block, including the two
// mistakes an operator actually makes: pasting sb:// into the namespace field,
// and pasting an entity-scoped connection string.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	method := OptionalString("auth_method", inputs)
	if method == "" {
		method = "connection_string"
	}

	switch method {
	case "connection_string":
		cs := OptionalString("connection_string", inputs)
		if cs == "" {
			return Auth{}, fmt.Errorf("connection_string is required — Service Bus namespace ▸ Shared access policies ▸ RootManageSharedAccessKey ▸ Primary Connection String")
		}
		a := Auth{Method: method, ConnectionString: cs}
		props, err := parseConnectionString(cs)
		if err != nil {
			return Auth{}, err
		}
		a.Namespace = props.namespace
		a.SharedAccessKey = props.sharedAccessKey
		a.EntityPath = props.entityPath
		return a, nil

	case "entra":
		ns, err := normaliseNamespace(OptionalString("namespace", inputs))
		if err != nil {
			return Auth{}, err
		}
		a := Auth{Method: method, Namespace: ns}
		if a.TenantID = OptionalString("azure_tenant_id", inputs); a.TenantID == "" {
			return Auth{}, fmt.Errorf("azure_tenant_id is required for Microsoft Entra authentication")
		}
		if a.ClientID = OptionalString("azure_client_id", inputs); a.ClientID == "" {
			return Auth{}, fmt.Errorf("azure_client_id is required for Microsoft Entra authentication")
		}
		if a.ClientSecret = OptionalString("azure_client_secret", inputs); a.ClientSecret == "" {
			return Auth{}, fmt.Errorf("azure_client_secret is required for Microsoft Entra authentication")
		}
		return a, nil

	default:
		return Auth{}, fmt.Errorf("auth_method must be connection_string or entra, got %q", method)
	}
}

type connStringProps struct {
	namespace       string
	sharedAccessKey string
	entityPath      string
}

// parseConnectionString reads the portal's connection string. We parse it
// ourselves as well as handing it to the SDK because two of the fields decide
// whether an action can run at all (EntityPath) and one must be redactable on
// its own (SharedAccessKey) — the SDK parses into an internal type we cannot
// see.
func parseConnectionString(cs string) (connStringProps, error) {
	var out connStringProps
	var endpoint string
	for _, part := range strings.Split(cs, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		// SharedAccessKey is base64 and routinely contains '=' padding, so
		// only the FIRST '=' separates key from value.
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch {
		case strings.EqualFold(k, "Endpoint"):
			endpoint = v
		case strings.EqualFold(k, "SharedAccessKey"):
			out.sharedAccessKey = v
		case strings.EqualFold(k, "EntityPath"):
			out.entityPath = v
		}
	}
	if endpoint == "" {
		return connStringProps{}, fmt.Errorf("connection_string has no Endpoint= — it should look like Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=…;SharedAccessKey=…")
	}
	ns, err := normaliseNamespace(endpoint)
	if err != nil {
		return connStringProps{}, fmt.Errorf("connection_string Endpoint is not a namespace host: %w", err)
	}
	out.namespace = ns
	return out, nil
}

// normaliseNamespace reduces a namespace to the bare host azservicebus.NewClient
// wants. Passing sb:// (or https://) is the single most common Entra
// misconfiguration and the SDK reports it as an opaque dial failure, so strip
// it rather than fail: the operator copied it from the portal, where it is
// shown with the scheme.
func normaliseNamespace(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("namespace is required for Microsoft Entra authentication, e.g. myns.servicebus.windows.net")
	}
	for _, scheme := range []string{"sb://", "amqps://", "https://", "http://"} {
		s = strings.TrimPrefix(s, scheme)
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" || strings.ContainsAny(s, "/ ") {
		return "", fmt.Errorf("namespace must be the host only, e.g. myns.servicebus.windows.net (no scheme, no path)")
	}
	return s, nil
}

// RequireNamespaceScope rejects an entity-scoped connection string for the
// management plane, where it cannot work at all.
//
// A policy copied from a queue's or topic's own Shared access policies blade
// carries ;EntityPath=<name> and is pinned to that entity: admin.Client has no
// entity to be pinned to, and the failure downstream is an unrelated-looking
// 401. Every admin action calls this first.
func (a Auth) RequireNamespaceScope() error {
	if a.EntityPath == "" {
		return nil
	}
	return fmt.Errorf("this connection string is scoped to the entity %q (it carries ;EntityPath=%s), and the Service Bus management API cannot use it — paste the NAMESPACE-level policy instead: Service Bus namespace ▸ Shared access policies ▸ RootManageSharedAccessKey", a.EntityPath, a.EntityPath)
}

// RequireEntityScope checks a data-plane action's target against an
// entity-scoped connection string. Such a string can send to and receive from
// its own entity and nothing else, so naming a different one is a config
// error, not a runtime one — and the runtime version is an unauthorized-access
// error that reads like a bad key.
func (a Auth) RequireEntityScope(entity string) error {
	if a.EntityPath == "" || strings.EqualFold(a.EntityPath, entity) {
		return nil
	}
	return fmt.Errorf("this connection string is scoped to the entity %q (it carries ;EntityPath=%s) and cannot reach %q — either target %s, or paste the namespace-level policy (Shared access policies ▸ RootManageSharedAccessKey)", a.EntityPath, a.EntityPath, entity, a.EntityPath)
}

// ---------------------------------------------------------------------------
// SDK seams
//
// azservicebus ships no fake, and these interfaces are the whole reason the
// actions are testable. They are exported so the external test package (which
// must be external — the action packages import this one) can build stubs.
// ---------------------------------------------------------------------------

// MessageBatch is the size-aware batch. It is an interface only so a stub can
// report ErrMessageTooLarge deterministically; the real one is
// *azservicebus.MessageBatch, and its AddMessage is what actually enforces the
// 256KB envelope.
type MessageBatch interface {
	AddMessage(m *azservicebus.Message, options *azservicebus.AddMessageOptions) error
	NumMessages() int32
}

// Sender is the slice of *azservicebus.Sender the actions use.
type Sender interface {
	SendMessage(ctx context.Context, message *azservicebus.Message, options *azservicebus.SendMessageOptions) error
	NewMessageBatch(ctx context.Context, options *azservicebus.MessageBatchOptions) (MessageBatch, error)
	SendMessageBatch(ctx context.Context, batch MessageBatch, options *azservicebus.SendMessageBatchOptions) error
	ScheduleMessages(ctx context.Context, messages []*azservicebus.Message, scheduledEnqueueTime time.Time, options *azservicebus.ScheduleMessagesOptions) ([]int64, error)
	CancelScheduledMessages(ctx context.Context, sequenceNumbers []int64, options *azservicebus.CancelScheduledMessagesOptions) error
	Close(ctx context.Context) error
}

// Receiver is the slice of *azservicebus.Receiver the actions use.
// *azservicebus.SessionReceiver satisfies it too — the session type is a
// different type with the same receive/settle surface.
type Receiver interface {
	ReceiveMessages(ctx context.Context, maxMessages int, options *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	PeekMessages(ctx context.Context, maxMessageCount int, options *azservicebus.PeekMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	ReceiveDeferredMessages(ctx context.Context, sequenceNumbers []int64, options *azservicebus.ReceiveDeferredMessagesOptions) ([]*azservicebus.ReceivedMessage, error)
	CompleteMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.CompleteMessageOptions) error
	AbandonMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.AbandonMessageOptions) error
	DeferMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.DeferMessageOptions) error
	DeadLetterMessage(ctx context.Context, message *azservicebus.ReceivedMessage, options *azservicebus.DeadLetterOptions) error
	Close(ctx context.Context) error
}

// SessionReceiver adds the session-level surface to Receiver. A session has its
// own lock, its own state, and FIFO ordering; it is a second receiver type
// rather than a flag on the first.
type SessionReceiver interface {
	Receiver
	SessionID() string
	LockedUntil() time.Time
	GetSessionState(ctx context.Context, options *azservicebus.GetSessionStateOptions) ([]byte, error)
	SetSessionState(ctx context.Context, state []byte, options *azservicebus.SetSessionStateOptions) error
	RenewSessionLock(ctx context.Context, options *azservicebus.RenewSessionLockOptions) error
}

// Admin is the management plane. The SDK's pagers are collapsed into plain
// slices here: every caller wants the whole (bounded) list, and a
// runtime.Pager cannot be stubbed without reimplementing azcore's paging.
type Admin interface {
	GetNamespaceProperties(ctx context.Context) (admin.NamespaceProperties, error)

	CreateQueue(ctx context.Context, name string, props *admin.QueueProperties) (admin.QueueProperties, error)
	UpdateQueue(ctx context.Context, name string, props admin.QueueProperties) (admin.QueueProperties, error)
	GetQueue(ctx context.Context, name string) (admin.QueueProperties, error)
	GetQueueRuntimeProperties(ctx context.Context, name string) (admin.QueueRuntimeProperties, error)
	DeleteQueue(ctx context.Context, name string) error
	ListQueues(ctx context.Context, limit int) ([]admin.QueueItem, error)

	CreateTopic(ctx context.Context, name string, props *admin.TopicProperties) (admin.TopicProperties, error)
	DeleteTopic(ctx context.Context, name string) error
	ListTopics(ctx context.Context, limit int) ([]admin.TopicItem, error)

	// TopicExists / SubscriptionExists answer the question a LIST cannot.
	//
	// Listing the subscriptions of a topic that does not exist returns an EMPTY
	// FEED and no error — not a 404 — and the same is true of listing the rules
	// of a subscription that does not exist. So "no results" and "no such
	// parent" are the same response, and a flow that fans out over the list of
	// a mistyped topic reads it as "nobody is subscribed" instead of "that
	// topic is not there". These make the two distinguishable; the list actions
	// only call them when the list came back empty, which is the only ambiguous
	// case, so the common path costs no extra round trip on the heavily
	// rate-limited management plane.
	TopicExists(ctx context.Context, topic string) (bool, error)
	SubscriptionExists(ctx context.Context, topic, sub string) (bool, error)

	CreateSubscription(ctx context.Context, topic, sub string, props *admin.SubscriptionProperties) (admin.SubscriptionProperties, error)
	DeleteSubscription(ctx context.Context, topic, sub string) error
	ListSubscriptions(ctx context.Context, topic string, limit int) ([]admin.SubscriptionPropertiesItem, error)
	GetSubscriptionRuntimeProperties(ctx context.Context, topic, sub string) (admin.SubscriptionRuntimeProperties, error)

	CreateRule(ctx context.Context, topic, sub string, opts *admin.CreateRuleOptions) (admin.RuleProperties, error)
	DeleteRule(ctx context.Context, topic, sub, rule string) error
	ListRules(ctx context.Context, topic, sub string, limit int) ([]admin.RuleProperties, error)
}

// ReceiverSpec names what to receive from. Exactly one of Queue or
// (Topic, Subscription) is set; SubQueue selects the dead-letter or transfer
// sub-queue, which is NOT a name you can type — see NewReceiver.
type ReceiverSpec struct {
	Queue        string
	Topic        string
	Subscription string
	SubQueue     azservicebus.SubQueue
	Mode         azservicebus.ReceiveMode
}

// Entity is the human name of the spec's target, for error text.
func (s ReceiverSpec) Entity() string {
	if s.Queue != "" {
		return s.Queue
	}
	return s.Topic + "/" + s.Subscription
}

// EntitySpec resolves either a queue or a topic subscription from the
// entity_type switch that the actions serving both shapes share.
//
// Messages live on the SUBSCRIPTION, never on the topic: there is no receiving
// from a topic and no sending to a subscription, which is why the two are a
// switch rather than one free-text "entity" field.
func EntitySpec(inputs []*core.Connection) (ReceiverSpec, error) {
	var spec ReceiverSpec
	switch OptionalString("entity_type", inputs) {
	case "", "queue":
		queue, err := RequiredString("queue", inputs)
		if err != nil {
			return spec, err
		}
		spec.Queue = queue
	case "subscription":
		topic, err := RequiredString("topic", inputs)
		if err != nil {
			return spec, err
		}
		subscription, err := RequiredString("subscription", inputs)
		if err != nil {
			return spec, err
		}
		spec.Topic, spec.Subscription = topic, subscription
	default:
		return spec, fmt.Errorf("entity_type must be queue or subscription")
	}
	return spec, nil
}

// DeadLetterSpec is EntitySpec plus the sub-queue selector.
//
// The dead-letter queue is a SUB-QUEUE, not a name: passing
// "<queue>/$deadletterqueue" as an entity name is accepted and then reported
// as not found, so it must be selected through ReceiverOptions.SubQueue
// instead. SubQueueTransfer is the other, separate place messages hide — the
// transfer DLQ holds messages that failed auto-forwarding.
func DeadLetterSpec(inputs []*core.Connection) (ReceiverSpec, error) {
	spec, err := EntitySpec(inputs)
	if err != nil {
		return spec, err
	}
	spec.SubQueue = azservicebus.SubQueueDeadLetter
	if OptionalString("sub_queue", inputs) == "transfer" {
		spec.SubQueue = azservicebus.SubQueueTransfer
	}
	return spec, nil
}

// The factories are vars so tests can stub the SDK. Production never reassigns
// them; SetSenderFactoryForTest and friends are the only writers.
var (
	newSender = func(a Auth, entity string) (Sender, error) {
		client, err := a.dataClient()
		if err != nil {
			return nil, err
		}
		s, err := client.NewSender(entity, nil)
		if err != nil {
			return nil, err
		}
		return &sdkSender{Sender: s, client: client}, nil
	}

	newReceiver = func(a Auth, spec ReceiverSpec) (Receiver, error) {
		client, err := a.dataClient()
		if err != nil {
			return nil, err
		}
		opts := &azservicebus.ReceiverOptions{ReceiveMode: spec.Mode, SubQueue: spec.SubQueue}
		var r *azservicebus.Receiver
		if spec.Queue != "" {
			r, err = client.NewReceiverForQueue(spec.Queue, opts)
		} else {
			r, err = client.NewReceiverForSubscription(spec.Topic, spec.Subscription, opts)
		}
		if err != nil {
			return nil, err
		}
		return &sdkReceiver{Receiver: r, client: client}, nil
	}

	newSessionReceiver = func(ctx context.Context, a Auth, spec ReceiverSpec, sessionID string) (SessionReceiver, error) {
		client, err := a.dataClient()
		if err != nil {
			return nil, err
		}
		opts := &azservicebus.SessionReceiverOptions{ReceiveMode: spec.Mode}
		var sr *azservicebus.SessionReceiver
		switch {
		case spec.Queue != "" && sessionID != "":
			sr, err = client.AcceptSessionForQueue(ctx, spec.Queue, sessionID, opts)
		case spec.Queue != "":
			sr, err = client.AcceptNextSessionForQueue(ctx, spec.Queue, opts)
		case sessionID != "":
			sr, err = client.AcceptSessionForSubscription(ctx, spec.Topic, spec.Subscription, sessionID, opts)
		default:
			sr, err = client.AcceptNextSessionForSubscription(ctx, spec.Topic, spec.Subscription, opts)
		}
		if err != nil {
			return nil, err
		}
		return &sdkSessionReceiver{SessionReceiver: sr, client: client}, nil
	}

	newAdmin = func(a Auth) (Admin, error) {
		if a.Method == "entra" {
			cred, err := a.credential()
			if err != nil {
				return nil, err
			}
			c, err := admin.NewClient(a.Namespace, cred, nil)
			if err != nil {
				return nil, err
			}
			return &sdkAdmin{c: c}, nil
		}
		c, err := admin.NewClientFromConnectionString(a.ConnectionString, nil)
		if err != nil {
			return nil, err
		}
		return &sdkAdmin{c: c}, nil
	}
)

// SetSenderFactoryForTest / SetReceiverFactoryForTest /
// SetSessionFactoryForTest / SetAdminFactoryForTest swap the SDK out for a
// stub and return a restore func. Test-only.
func SetSenderFactoryForTest(fn func(Auth, string) (Sender, error)) func() {
	prev := newSender
	newSender = fn
	return func() { newSender = prev }
}

func SetReceiverFactoryForTest(fn func(Auth, ReceiverSpec) (Receiver, error)) func() {
	prev := newReceiver
	newReceiver = fn
	return func() { newReceiver = prev }
}

func SetSessionFactoryForTest(fn func(context.Context, Auth, ReceiverSpec, string) (SessionReceiver, error)) func() {
	prev := newSessionReceiver
	newSessionReceiver = fn
	return func() { newSessionReceiver = prev }
}

func SetAdminFactoryForTest(fn func(Auth) (Admin, error)) func() {
	prev := newAdmin
	newAdmin = fn
	return func() { newAdmin = prev }
}

// NewSender opens a sender for a queue or topic, after checking the target
// against an entity-scoped connection string.
func NewSender(a Auth, entity string) (Sender, error) {
	if err := a.RequireEntityScope(entity); err != nil {
		return nil, err
	}
	return newSender(a, entity)
}

// NewReceiver opens a receiver for the spec. The dead-letter queue is reached
// with SubQueue, never by naming "<queue>/$deadletterqueue" — that string is
// accepted as an entity name and then reported as not found, which is how the
// mistake survives review.
func NewReceiver(a Auth, spec ReceiverSpec) (Receiver, error) {
	entity := spec.Queue
	if entity == "" {
		entity = spec.Topic
	}
	if err := a.RequireEntityScope(entity); err != nil {
		return nil, err
	}
	return newReceiver(a, spec)
}

// NewSessionReceiver accepts a specific session, or the next available one when
// sessionID is empty.
func NewSessionReceiver(ctx context.Context, a Auth, spec ReceiverSpec, sessionID string) (SessionReceiver, error) {
	entity := spec.Queue
	if entity == "" {
		entity = spec.Topic
	}
	if err := a.RequireEntityScope(entity); err != nil {
		return nil, err
	}
	return newSessionReceiver(ctx, a, spec, sessionID)
}

// NewAdmin opens the management client, refusing an entity-scoped connection
// string up front.
func NewAdmin(a Auth) (Admin, error) {
	if err := a.RequireNamespaceScope(); err != nil {
		return nil, err
	}
	return newAdmin(a)
}

// credential mints the service-principal credential. Managed identity is
// deliberately absent: the executor is not necessarily Azure-hosted, and
// DefaultAzureCredential's fallback chain fails slowly and opaquely with no
// IMDS endpoint to find.
func (a Auth) credential() (*azidentity.ClientSecretCredential, error) {
	cred, err := azidentity.NewClientSecretCredential(a.TenantID, a.ClientID, a.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build the Entra credential: %s", Redact(a, err.Error()))
	}
	return cred, nil
}

func (a Auth) dataClient() (*azservicebus.Client, error) {
	if a.Method == "entra" {
		cred, err := a.credential()
		if err != nil {
			return nil, err
		}
		return azservicebus.NewClient(a.Namespace, cred, nil)
	}
	return azservicebus.NewClientFromConnectionString(a.ConnectionString, nil)
}

// sdkSender adapts *azservicebus.Sender to Sender: the batch methods need the
// interface/concrete conversion, and Close must also close the client the
// factory opened, or the AMQP connection leaks for the life of the process.
type sdkSender struct {
	*azservicebus.Sender
	client *azservicebus.Client
}

func (s *sdkSender) NewMessageBatch(ctx context.Context, options *azservicebus.MessageBatchOptions) (MessageBatch, error) {
	return s.Sender.NewMessageBatch(ctx, options)
}

func (s *sdkSender) SendMessageBatch(ctx context.Context, batch MessageBatch, options *azservicebus.SendMessageBatchOptions) error {
	real, ok := batch.(*azservicebus.MessageBatch)
	if !ok {
		return fmt.Errorf("internal error: batch was not created by this sender")
	}
	return s.Sender.SendMessageBatch(ctx, real, options)
}

func (s *sdkSender) Close(ctx context.Context) error {
	_ = s.Sender.Close(ctx)
	return s.client.Close(ctx)
}

type sdkReceiver struct {
	*azservicebus.Receiver
	client *azservicebus.Client
}

func (r *sdkReceiver) Close(ctx context.Context) error {
	_ = r.Receiver.Close(ctx)
	return r.client.Close(ctx)
}

type sdkSessionReceiver struct {
	*azservicebus.SessionReceiver
	client *azservicebus.Client
}

func (r *sdkSessionReceiver) Close(ctx context.Context) error {
	_ = r.SessionReceiver.Close(ctx)
	return r.client.Close(ctx)
}

// sdkAdmin flattens the SDK's pagers and response wrappers into the Admin
// interface's plain values.
type sdkAdmin struct {
	c *admin.Client
}

func (s *sdkAdmin) GetNamespaceProperties(ctx context.Context) (admin.NamespaceProperties, error) {
	resp, err := s.c.GetNamespaceProperties(ctx, nil)
	if err != nil {
		return admin.NamespaceProperties{}, err
	}
	return resp.NamespaceProperties, nil
}

func (s *sdkAdmin) CreateQueue(ctx context.Context, name string, props *admin.QueueProperties) (admin.QueueProperties, error) {
	resp, err := s.c.CreateQueue(ctx, name, &admin.CreateQueueOptions{Properties: props})
	if err != nil {
		return admin.QueueProperties{}, err
	}
	return resp.QueueProperties, nil
}

func (s *sdkAdmin) UpdateQueue(ctx context.Context, name string, props admin.QueueProperties) (admin.QueueProperties, error) {
	resp, err := s.c.UpdateQueue(ctx, name, props, nil)
	if err != nil {
		return admin.QueueProperties{}, err
	}
	return resp.QueueProperties, nil
}

func (s *sdkAdmin) GetQueue(ctx context.Context, name string) (admin.QueueProperties, error) {
	resp, err := s.c.GetQueue(ctx, name, nil)
	if err != nil {
		return admin.QueueProperties{}, err
	}
	if resp == nil {
		return admin.QueueProperties{}, errNotFound(name)
	}
	return resp.QueueProperties, nil
}

func (s *sdkAdmin) GetQueueRuntimeProperties(ctx context.Context, name string) (admin.QueueRuntimeProperties, error) {
	resp, err := s.c.GetQueueRuntimeProperties(ctx, name, nil)
	if err != nil {
		return admin.QueueRuntimeProperties{}, err
	}
	if resp == nil {
		return admin.QueueRuntimeProperties{}, errNotFound(name)
	}
	return resp.QueueRuntimeProperties, nil
}

func (s *sdkAdmin) DeleteQueue(ctx context.Context, name string) error {
	_, err := s.c.DeleteQueue(ctx, name, nil)
	return err
}

func (s *sdkAdmin) ListQueues(ctx context.Context, limit int) ([]admin.QueueItem, error) {
	pager := s.c.NewListQueuesPager(&admin.ListQueuesOptions{MaxPageSize: int32(limit)})
	out := []admin.QueueItem{}
	for pages := 0; pager.More() && pages < MaxAllPages; pages++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Queues...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
	}
	return out, nil
}

func (s *sdkAdmin) CreateTopic(ctx context.Context, name string, props *admin.TopicProperties) (admin.TopicProperties, error) {
	resp, err := s.c.CreateTopic(ctx, name, &admin.CreateTopicOptions{Properties: props})
	if err != nil {
		return admin.TopicProperties{}, err
	}
	return resp.TopicProperties, nil
}

func (s *sdkAdmin) DeleteTopic(ctx context.Context, name string) error {
	_, err := s.c.DeleteTopic(ctx, name, nil)
	return err
}

func (s *sdkAdmin) ListTopics(ctx context.Context, limit int) ([]admin.TopicItem, error) {
	pager := s.c.NewListTopicsPager(&admin.ListTopicsOptions{MaxPageSize: int32(limit)})
	out := []admin.TopicItem{}
	for pages := 0; pager.More() && pages < MaxAllPages; pages++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Topics...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
	}
	return out, nil
}

// TopicExists reports whether the topic is there. The SDK's "nil response, nil
// error" shape for a missing entity is the whole subtlety: an err of nil does
// NOT mean it was found.
func (s *sdkAdmin) TopicExists(ctx context.Context, topic string) (bool, error) {
	resp, err := s.c.GetTopic(ctx, topic, nil)
	if err != nil {
		return false, err
	}
	return resp != nil, nil
}

// SubscriptionExists reports whether the subscription is there. A missing TOPIC
// surfaces here as an error rather than a nil response, and either way the
// answer is the same: not found.
func (s *sdkAdmin) SubscriptionExists(ctx context.Context, topic, sub string) (bool, error) {
	resp, err := s.c.GetSubscription(ctx, topic, sub, nil)
	if err != nil {
		return false, err
	}
	return resp != nil, nil
}

func (s *sdkAdmin) CreateSubscription(ctx context.Context, topic, sub string, props *admin.SubscriptionProperties) (admin.SubscriptionProperties, error) {
	resp, err := s.c.CreateSubscription(ctx, topic, sub, &admin.CreateSubscriptionOptions{Properties: props})
	if err != nil {
		return admin.SubscriptionProperties{}, err
	}
	return resp.SubscriptionProperties, nil
}

func (s *sdkAdmin) DeleteSubscription(ctx context.Context, topic, sub string) error {
	_, err := s.c.DeleteSubscription(ctx, topic, sub, nil)
	return err
}

func (s *sdkAdmin) ListSubscriptions(ctx context.Context, topic string, limit int) ([]admin.SubscriptionPropertiesItem, error) {
	pager := s.c.NewListSubscriptionsPager(topic, &admin.ListSubscriptionsOptions{MaxPageSize: int32(limit)})
	out := []admin.SubscriptionPropertiesItem{}
	for pages := 0; pager.More() && pages < MaxAllPages; pages++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Subscriptions...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
	}
	return out, nil
}

func (s *sdkAdmin) GetSubscriptionRuntimeProperties(ctx context.Context, topic, sub string) (admin.SubscriptionRuntimeProperties, error) {
	resp, err := s.c.GetSubscriptionRuntimeProperties(ctx, topic, sub, nil)
	if err != nil {
		return admin.SubscriptionRuntimeProperties{}, err
	}
	if resp == nil {
		return admin.SubscriptionRuntimeProperties{}, errNotFound(topic + "/" + sub)
	}
	return resp.SubscriptionRuntimeProperties, nil
}

func (s *sdkAdmin) CreateRule(ctx context.Context, topic, sub string, opts *admin.CreateRuleOptions) (admin.RuleProperties, error) {
	resp, err := s.c.CreateRule(ctx, topic, sub, opts)
	if err != nil {
		return admin.RuleProperties{}, err
	}
	return resp.RuleProperties, nil
}

func (s *sdkAdmin) DeleteRule(ctx context.Context, topic, sub, rule string) error {
	_, err := s.c.DeleteRule(ctx, topic, sub, rule, nil)
	return err
}

func (s *sdkAdmin) ListRules(ctx context.Context, topic, sub string, limit int) ([]admin.RuleProperties, error) {
	pager := s.c.NewListRulesPager(topic, sub, &admin.ListRulesOptions{MaxPageSize: int32(limit)})
	out := []admin.RuleProperties{}
	for pages := 0; pager.More() && pages < MaxAllPages; pages++ {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Rules...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
	}
	return out, nil
}

// errNotFound covers the SDK's documented "nil response, nil error" shape for
// a missing entity on the Get* calls, which would otherwise surface as an
// empty properties object reported as success.
func errNotFound(name string) error {
	return fmt.Errorf("no entity named %q exists in this namespace", name)
}

// CheckListParent turns an ambiguous empty list into either a genuine empty
// list (nil) or a not-found error.
//
// The management API answers "list the children of X" with an empty feed
// whether X has no children or X does not exist, so an empty result on its own
// says nothing. Only the empty case pays for the extra call.
func CheckListParent(ctx context.Context, count int, exists func(context.Context) (bool, error), missing string) error {
	if count > 0 {
		return nil
	}
	ok, err := exists(ctx)
	if err != nil {
		// The existence probe is a diagnostic, not the operation. If it cannot
		// answer, report the empty list rather than invent a failure the
		// operator did not actually hit.
		return nil
	}
	if ok {
		return nil
	}
	return errors.New(missing)
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

// Context returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func Context(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// AdminContext bounds a management-plane call, which has no other timeout.
func AdminContext(flow *core.Flow) (context.Context, context.CancelFunc) {
	return context.WithTimeout(Context(flow), adminTimeout)
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// Redact scrubs every credential out of a message. The connection string is
// the dangerous one: it carries SharedAccessKey inline, and the SDK quotes
// both the whole string and the bare key depending on where it failed.
func Redact(a Auth, msg string) string {
	for _, secret := range []string{a.ConnectionString, a.SharedAccessKey, a.ClientSecret} {
		if secret != "" {
			msg = strings.ReplaceAll(msg, secret, "********")
		}
	}
	return msg
}

// Fail is the single place an error becomes operator-facing text: it maps the
// SDK's error codes to something actionable and redacts what is left.
func Fail(a Auth, action string, err error) map[string]interface{} {
	return ErrorResult(fmt.Sprintf("%s: %s", action, FriendlyError(a, err)))
}

// FriendlyError decorates the Service Bus error codes whose raw text does not
// tell an operator what to do next.
func FriendlyError(a Auth, err error) string {
	if err == nil {
		return ""
	}
	msg := Redact(a, err.Error())

	var sbErr *azservicebus.Error
	if !asServiceBusError(err, &sbErr) {
		return msg
	}
	switch sbErr.Code {
	case azservicebus.CodeUnauthorizedAccess:
		if a.Method == "entra" {
			// The credential is usually FINE here. Service Bus's data-plane
			// roles are separate from the control plane, so an operator who is
			// subscription Owner still gets this — and reads it as a bad
			// secret, and rotates a working secret.
			return msg + " — the credential authenticated but is not authorised for this entity. Service Bus data access needs an RBAC role assignment on the namespace or entity: Azure Service Bus Data Sender (to send), Azure Service Bus Data Receiver (to receive), or Azure Service Bus Data Owner (both). Subscription Owner or Contributor is NOT enough."
		}
		return msg + " — the shared access policy does not grant this operation on this entity. Check the policy's Send/Listen/Manage claims, and that it is the namespace-level policy if the action manages entities."
	case azservicebus.CodeNotFound:
		return msg + " — no queue, topic or subscription with that name exists in this namespace. Note the dead-letter queue is not a name you can type: use the dead-letter actions instead of \"<queue>/$deadletterqueue\"."
	case azservicebus.CodeLockLost:
		return msg + " — the message lock expired before the message could be settled. LockDuration defaults to 60s and cannot exceed 5 minutes; the message will be redelivered (and dead-lettered once it exceeds MaxDeliveryCount). Receive fewer messages at a time, or shorten the work between receive and settle."
	case azservicebus.CodeTimeout:
		return msg + " — the service timed out. For a session receive this usually means no session was available to accept."
	default:
		return msg
	}
}

// asServiceBusError is errors.As, isolated so the SDK's error type is named in
// exactly one place.
func asServiceBusError(err error, target **azservicebus.Error) bool {
	for err != nil {
		if sb, ok := err.(*azservicebus.Error); ok {
			*target = sb
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
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

// OptionalRawString is OptionalString without the trim — message bodies are
// the operator's bytes, and trimming them silently corrupts a payload whose
// whitespace matters.
func OptionalRawString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
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

// BoolIfSet returns the boolean and whether the checkbox was ever touched. The
// tri-state matters on admin properties: not sending a field is not the same
// as sending false.
func BoolIfSet(name string, inputs []*core.Connection) (bool, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
}

// OptionalJSON parses an object/text input into an arbitrary value. Returns
// (nil, nil) when absent/blank.
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

// OptionalPropertyMap parses an application-properties style JSON object.
func OptionalPropertyMap(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil || v == nil {
		return nil, err
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON object, e.g. {"tenant":"acme"}`, name)
	}
	return m, nil
}

// ClampMaxMessages bounds the maxMessages argument. Note this is "up to N",
// never "wait for N": asking for 10 and getting 1 is normal.
func ClampMaxMessages(inputs []*core.Connection) int {
	n, set := OptionalInt("max_messages", inputs)
	if !set || n <= 0 {
		return DefaultMaxMessages
	}
	if n > MaxMaxMessages {
		return MaxMaxMessages
	}
	return n
}

// ClampMaxWait bounds the receive window.
func ClampMaxWait(inputs []*core.Connection) time.Duration {
	n, set := OptionalInt("max_wait_seconds", inputs)
	if !set || n <= 0 {
		n = DefaultMaxWaitSeconds
	}
	if n > MaxMaxWaitSeconds {
		n = MaxMaxWaitSeconds
	}
	return time.Duration(n) * time.Second
}

// ClampLimit bounds an admin list page.
func ClampLimit(inputs []*core.Connection) int {
	n, set := OptionalInt("limit", inputs)
	if !set || n <= 0 {
		return DefaultPageLimit
	}
	if n > MaxPageLimit {
		return MaxPageLimit
	}
	return n
}

// ReceiveModeFrom reads the receive_mode input. PeekLock is the default and
// must stay so: ReceiveAndDelete destroys the message before the flow sees it,
// with no redelivery and no dead-letter if the flow then fails.
func ReceiveModeFrom(inputs []*core.Connection) (azservicebus.ReceiveMode, error) {
	switch v := OptionalString("receive_mode", inputs); v {
	case "", "peek_lock":
		return azservicebus.ReceiveModePeekLock, nil
	case "receive_and_delete":
		return azservicebus.ReceiveModeReceiveAndDelete, nil
	default:
		return 0, fmt.Errorf("receive_mode must be peek_lock or receive_and_delete, got %q", v)
	}
}

// Disposition is what the action does with each message before it returns.
type Disposition string

const (
	DispositionComplete   Disposition = "complete"
	DispositionAbandon    Disposition = "abandon"
	DispositionDeadLetter Disposition = "dead_letter"
	DispositionDefer      Disposition = "defer"
)

// DispositionFrom reads the disposition input.
func DispositionFrom(inputs []*core.Connection) (Disposition, error) {
	switch v := Disposition(OptionalString("disposition", inputs)); v {
	case "", DispositionComplete:
		return DispositionComplete, nil
	case DispositionAbandon, DispositionDeadLetter, DispositionDefer:
		return v, nil
	default:
		return "", fmt.Errorf("disposition must be complete, abandon, dead_letter or defer, got %q", v)
	}
}

// ParseSequenceNumbers reads a sequence-number list, accepting either a JSON
// array or a comma-separated list — schedule outputs feed this straight
// through, and an operator retyping them by hand uses commas.
func ParseSequenceNumbers(raw string) ([]int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("sequence_numbers is required — it is the value queue_schedule returned; without it a scheduled message cannot be identified")
	}
	if strings.HasPrefix(s, "[") {
		var nums []int64
		if err := json.Unmarshal([]byte(s), &nums); err != nil {
			return nil, fmt.Errorf("sequence_numbers must be a JSON array of numbers or a comma-separated list: %w", err)
		}
		if len(nums) == 0 {
			return nil, fmt.Errorf("sequence_numbers is empty")
		}
		return nums, nil
	}
	out := []int64{}
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("sequence_numbers contains %q, which is not a sequence number", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("sequence_numbers is empty")
	}
	return out, nil
}

// ParseTime reads an RFC3339 timestamp input.
func ParseTime(name, raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp, e.g. 2026-07-17T09:30:00Z", name)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Message shaping
// ---------------------------------------------------------------------------

// BuildMessage assembles an outgoing message from the standard message input
// block that queue_send, topic_send, queue_send_batch and queue_schedule share.
func BuildMessage(inputs []*core.Connection) (*azservicebus.Message, error) {
	body := OptionalRawString("body", inputs)
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("body is required")
	}
	msg := &azservicebus.Message{Body: []byte(body)}

	setPtr := func(name string, dst **string) {
		if v := OptionalString(name, inputs); v != "" {
			s := v
			*dst = &s
		}
	}
	setPtr("message_id", &msg.MessageID)
	setPtr("session_id", &msg.SessionID)
	setPtr("correlation_id", &msg.CorrelationID)
	setPtr("subject", &msg.Subject)
	setPtr("content_type", &msg.ContentType)
	setPtr("reply_to", &msg.ReplyTo)
	setPtr("to", &msg.To)
	setPtr("partition_key", &msg.PartitionKey)

	if secs, set := OptionalInt("time_to_live_seconds", inputs); set && secs > 0 {
		ttl := time.Duration(secs) * time.Second
		msg.TimeToLive = &ttl
	}
	if raw := OptionalString("scheduled_enqueue_time", inputs); raw != "" {
		t, err := ParseTime("scheduled_enqueue_time", raw)
		if err != nil {
			return nil, err
		}
		msg.ScheduledEnqueueTime = &t
	}
	props, err := OptionalPropertyMap("application_properties", inputs)
	if err != nil {
		return nil, err
	}
	if props != nil {
		msg.ApplicationProperties = props
	}
	return msg, nil
}

// MessageFromJSON builds one message of a send-batch from its JSON object. The
// field names mirror the single-send inputs so an operator can move between
// the two without a translation table.
func MessageFromJSON(index int, raw interface{}) (*azservicebus.Message, error) {
	obj, ok := raw.(map[string]interface{})
	if !ok {
		// A bare string is a body — the common case for a batch built by a
		// Loop, and rejecting it would be pedantry.
		if s, isStr := raw.(string); isStr {
			return &azservicebus.Message{Body: []byte(s)}, nil
		}
		return nil, fmt.Errorf(`messages[%d] must be a JSON object like {"body":"…"} or a plain string body`, index)
	}

	msg := &azservicebus.Message{}
	switch b := obj["body"].(type) {
	case string:
		msg.Body = []byte(b)
	case nil:
		return nil, fmt.Errorf("messages[%d] has no body", index)
	default:
		// An object body is what a flow carrying structured data produces;
		// re-marshalling it is friendlier than making the operator stringify.
		encoded, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("messages[%d].body could not be encoded: %w", index, err)
		}
		msg.Body = encoded
	}

	strField := func(key string, dst **string) error {
		v, present := obj[key]
		if !present || v == nil {
			return nil
		}
		s, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("messages[%d].%s must be a string", index, key)
		}
		*dst = &s
		return nil
	}
	for key, dst := range map[string]**string{
		"message_id":     &msg.MessageID,
		"session_id":     &msg.SessionID,
		"correlation_id": &msg.CorrelationID,
		"subject":        &msg.Subject,
		"content_type":   &msg.ContentType,
		"reply_to":       &msg.ReplyTo,
		"to":             &msg.To,
		"partition_key":  &msg.PartitionKey,
	} {
		if err := strField(key, dst); err != nil {
			return nil, err
		}
	}
	if props, present := obj["application_properties"]; present && props != nil {
		m, isMap := props.(map[string]interface{})
		if !isMap {
			return nil, fmt.Errorf("messages[%d].application_properties must be a JSON object", index)
		}
		msg.ApplicationProperties = m
	}
	return msg, nil
}

// MessageEcho describes a message that was sent. The broker returns nothing on
// a send, so the output echoes what went out rather than inventing a result.
func MessageEcho(msg *azservicebus.Message, entity string) map[string]interface{} {
	out := map[string]interface{}{
		"entity":     entity,
		"body_bytes": len(msg.Body),
	}
	if msg.MessageID != nil {
		out["message_id"] = *msg.MessageID
	}
	if msg.SessionID != nil {
		out["session_id"] = *msg.SessionID
	}
	if msg.Subject != nil {
		out["subject"] = *msg.Subject
	}
	if msg.ScheduledEnqueueTime != nil {
		out["scheduled_enqueue_time"] = msg.ScheduledEnqueueTime.UTC().Format(time.RFC3339)
	}
	return out
}

// MessageOutput shapes a received message. Everything an operator needs to
// reason about redelivery is surfaced deliberately: DeliveryCount is how a
// flow detects a poison message, LockedUntil is its settlement budget, and the
// dead-letter reason/description are the whole point of the DLQ actions.
//
// parseJSON additionally decodes the body into body_json when it parses, which
// is what a downstream node wants; the raw body always stays in body.
func MessageOutput(m *azservicebus.ReceivedMessage, parseJSON bool) map[string]interface{} {
	out := map[string]interface{}{
		"message_id":     m.MessageID,
		"body":           string(m.Body),
		"delivery_count": int(m.DeliveryCount),
		"lock_token":     hex.EncodeToString(m.LockToken[:]),
		"state":          messageState(m.State),
	}
	if parseJSON {
		var decoded interface{}
		if err := json.Unmarshal(m.Body, &decoded); err == nil {
			out["body_json"] = decoded
		}
	}
	if m.SequenceNumber != nil {
		out["sequence_number"] = *m.SequenceNumber
	}
	if m.EnqueuedSequenceNumber != nil {
		out["enqueued_sequence_number"] = *m.EnqueuedSequenceNumber
	}
	setStr := func(key string, v *string) {
		if v != nil {
			out[key] = *v
		}
	}
	setStr("content_type", m.ContentType)
	setStr("correlation_id", m.CorrelationID)
	setStr("subject", m.Subject)
	setStr("reply_to", m.ReplyTo)
	setStr("reply_to_session_id", m.ReplyToSessionID)
	setStr("session_id", m.SessionID)
	setStr("to", m.To)
	setStr("partition_key", m.PartitionKey)
	setStr("dead_letter_reason", m.DeadLetterReason)
	setStr("dead_letter_error_description", m.DeadLetterErrorDescription)
	setStr("dead_letter_source", m.DeadLetterSource)
	setTime := func(key string, v *time.Time) {
		if v != nil {
			out[key] = v.UTC().Format(time.RFC3339)
		}
	}
	setTime("enqueued_time", m.EnqueuedTime)
	setTime("expires_at", m.ExpiresAt)
	setTime("locked_until", m.LockedUntil)
	if m.TimeToLive != nil {
		out["time_to_live_seconds"] = int(m.TimeToLive.Seconds())
	}
	if len(m.ApplicationProperties) > 0 {
		out["application_properties"] = m.ApplicationProperties
	}
	return out
}

func messageState(s azservicebus.MessageState) string {
	switch s {
	case azservicebus.MessageStateDeferred:
		return "deferred"
	case azservicebus.MessageStateScheduled:
		return "scheduled"
	default:
		return "active"
	}
}

// MessagesOutput shapes a batch of received messages.
func MessagesOutput(msgs []*azservicebus.ReceivedMessage, parseJSON bool) []interface{} {
	out := make([]interface{}, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, MessageOutput(m, parseJSON))
	}
	return out
}

// ReceiveWithin receives up to maxMessages, giving up after wait.
//
// The subtlety it exists for: when the wait elapses with nothing to show,
// ReceiveMessages does NOT return an empty slice — it returns the cancellation
// error, because the SDK only swallows errors when it has messages to hand
// back. An idle queue would therefore report "context deadline exceeded" on
// the error port on every poll. That is the ordinary state of a quiet queue,
// so it becomes an empty result here. A cancelled FLOW still surfaces: the
// parent context's own error distinguishes the two.
func ReceiveWithin(ctx context.Context, r Receiver, maxMessages int, wait time.Duration) ([]*azservicebus.ReceivedMessage, error) {
	recvCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	msgs, err := r.ReceiveMessages(recvCtx, maxMessages, nil)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, nil
		}
		return nil, err
	}
	return msgs, nil
}

// Settle applies the disposition to every message, in this action, on the
// connection that locked them. It is deliberately not a separate node: see the
// package comment.
//
// A ReceiveAndDelete message is not settleable at all (the SDK says so
// explicitly), so settlement is skipped rather than reported as a failure —
// the message is already gone, which is what the operator asked for.
func Settle(ctx context.Context, r Receiver, msgs []*azservicebus.ReceivedMessage, mode azservicebus.ReceiveMode, d Disposition, reason, description string) error {
	if mode == azservicebus.ReceiveModeReceiveAndDelete {
		return nil
	}
	for _, m := range msgs {
		var err error
		switch d {
		case DispositionAbandon:
			err = r.AbandonMessage(ctx, m, nil)
		case DispositionDefer:
			err = r.DeferMessage(ctx, m, nil)
		case DispositionDeadLetter:
			opts := &azservicebus.DeadLetterOptions{}
			if reason != "" {
				opts.Reason = &reason
			}
			if description != "" {
				opts.ErrorDescription = &description
			}
			err = r.DeadLetterMessage(ctx, m, opts)
		default:
			err = r.CompleteMessage(ctx, m, nil)
		}
		if err != nil {
			return fmt.Errorf("received %d message(s) but could not %s message %s: %w", len(msgs), d, m.MessageID, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Admin property helpers
// ---------------------------------------------------------------------------

// ISODuration renders seconds as the ISO-8601 duration the management plane
// takes (LockDuration, DefaultMessageTimeToLive, AutoDeleteOnIdle are all
// *string, not time.Duration).
func ISODuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	days := int(d.Hours()) / 24
	rem := d - time.Duration(days)*24*time.Hour
	hours := int(rem.Hours())
	rem -= time.Duration(hours) * time.Hour
	mins := int(rem.Minutes())
	rem -= time.Duration(mins) * time.Minute
	secs := int(rem.Seconds())

	out := "P"
	if days > 0 {
		out += strconv.Itoa(days) + "D"
	}
	if hours > 0 || mins > 0 || secs > 0 || days == 0 {
		out += "T"
		if hours > 0 {
			out += strconv.Itoa(hours) + "H"
		}
		if mins > 0 {
			out += strconv.Itoa(mins) + "M"
		}
		if secs > 0 || (hours == 0 && mins == 0) {
			out += strconv.Itoa(secs) + "S"
		}
	}
	return out
}

// SetISODurationIfSet writes an ISO-8601 duration property from a seconds
// input, leaving it nil (⇒ the service default) when unset.
func SetISODurationIfSet(dst **string, name string, inputs []*core.Connection) {
	if secs, set := OptionalInt(name, inputs); set && secs > 0 {
		s := ISODuration(secs)
		*dst = &s
	}
}

// SetInt32IfSet writes an *int32 property only when its input was provided.
func SetInt32IfSet(dst **int32, name string, inputs []*core.Connection) {
	if v, set := OptionalInt(name, inputs); set {
		n := int32(v)
		*dst = &n
	}
}

// SetBoolIfSet writes a *bool property only when the checkbox was touched, so
// the tri-state nil survives as "use the service default".
func SetBoolIfSet(dst **bool, name string, inputs []*core.Connection) {
	if v, set := BoolIfSet(name, inputs); set {
		b := v
		*dst = &b
	}
}

// SetStringIfPresent writes a *string property only when the input is non-blank.
func SetStringIfPresent(dst **string, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		s := v
		*dst = &s
	}
}

// QueueOutput flattens QueueProperties for the action output. The SDK's
// pointers become present-or-absent keys, which is what a flow can branch on.
func QueueOutput(name string, p admin.QueueProperties) map[string]interface{} {
	out := map[string]interface{}{"name": name}
	putStr(out, "lock_duration", p.LockDuration)
	putStr(out, "default_message_time_to_live", p.DefaultMessageTimeToLive)
	putStr(out, "duplicate_detection_history_time_window", p.DuplicateDetectionHistoryTimeWindow)
	putStr(out, "auto_delete_on_idle", p.AutoDeleteOnIdle)
	putStr(out, "forward_to", p.ForwardTo)
	putStr(out, "forward_dead_lettered_messages_to", p.ForwardDeadLetteredMessagesTo)
	putStr(out, "user_metadata", p.UserMetadata)
	putInt32(out, "max_size_megabytes", p.MaxSizeInMegabytes)
	putInt32(out, "max_delivery_count", p.MaxDeliveryCount)
	putBool(out, "requires_session", p.RequiresSession)
	putBool(out, "requires_duplicate_detection", p.RequiresDuplicateDetection)
	putBool(out, "dead_lettering_on_message_expiration", p.DeadLetteringOnMessageExpiration)
	putBool(out, "enable_batched_operations", p.EnableBatchedOperations)
	putBool(out, "enable_partitioning", p.EnablePartitioning)
	if p.Status != nil {
		out["status"] = string(*p.Status)
	}
	if p.MaxMessageSizeInKilobytes != nil {
		out["max_message_size_kilobytes"] = *p.MaxMessageSizeInKilobytes
	}
	return out
}

// TopicOutput flattens TopicProperties.
func TopicOutput(name string, p admin.TopicProperties) map[string]interface{} {
	out := map[string]interface{}{"name": name}
	putStr(out, "default_message_time_to_live", p.DefaultMessageTimeToLive)
	putStr(out, "duplicate_detection_history_time_window", p.DuplicateDetectionHistoryTimeWindow)
	putStr(out, "auto_delete_on_idle", p.AutoDeleteOnIdle)
	putStr(out, "user_metadata", p.UserMetadata)
	putInt32(out, "max_size_megabytes", p.MaxSizeInMegabytes)
	putBool(out, "requires_duplicate_detection", p.RequiresDuplicateDetection)
	putBool(out, "enable_batched_operations", p.EnableBatchedOperations)
	putBool(out, "enable_partitioning", p.EnablePartitioning)
	putBool(out, "support_ordering", p.SupportOrdering)
	if p.Status != nil {
		out["status"] = string(*p.Status)
	}
	if p.MaxMessageSizeInKilobytes != nil {
		out["max_message_size_kilobytes"] = *p.MaxMessageSizeInKilobytes
	}
	return out
}

// SubscriptionOutput flattens SubscriptionProperties.
func SubscriptionOutput(topic, name string, p admin.SubscriptionProperties) map[string]interface{} {
	out := map[string]interface{}{"name": name, "topic": topic}
	putStr(out, "lock_duration", p.LockDuration)
	putStr(out, "default_message_time_to_live", p.DefaultMessageTimeToLive)
	putStr(out, "auto_delete_on_idle", p.AutoDeleteOnIdle)
	putStr(out, "forward_to", p.ForwardTo)
	putStr(out, "forward_dead_lettered_messages_to", p.ForwardDeadLetteredMessagesTo)
	putStr(out, "user_metadata", p.UserMetadata)
	putInt32(out, "max_delivery_count", p.MaxDeliveryCount)
	putBool(out, "requires_session", p.RequiresSession)
	putBool(out, "dead_lettering_on_message_expiration", p.DeadLetteringOnMessageExpiration)
	putBool(out, "dead_lettering_on_filter_evaluation_exceptions", p.EnableDeadLetteringOnFilterEvaluationExceptions)
	putBool(out, "enable_batched_operations", p.EnableBatchedOperations)
	if p.Status != nil {
		out["status"] = string(*p.Status)
	}
	return out
}

// RuleOutput flattens a rule, naming the filter type so an operator can see at
// a glance which rules are real filters and which is the $Default TrueFilter
// that ships with every new subscription and matches everything.
func RuleOutput(p admin.RuleProperties) map[string]interface{} {
	out := map[string]interface{}{"name": p.Name}
	switch f := p.Filter.(type) {
	case *admin.SQLFilter:
		out["filter_type"] = "sql"
		out["filter_expression"] = f.Expression
		if len(f.Parameters) > 0 {
			out["filter_parameters"] = f.Parameters
		}
	case *admin.CorrelationFilter:
		out["filter_type"] = "correlation"
		out["filter"] = correlationOutput(f)
	case *admin.TrueFilter:
		out["filter_type"] = "true"
	case *admin.FalseFilter:
		out["filter_type"] = "false"
	case *admin.UnknownRuleFilter:
		out["filter_type"] = f.Type
	}
	if a, ok := p.Action.(*admin.SQLAction); ok {
		out["action_expression"] = a.Expression
		if len(a.Parameters) > 0 {
			out["action_parameters"] = a.Parameters
		}
	}
	return out
}

func correlationOutput(f *admin.CorrelationFilter) map[string]interface{} {
	out := map[string]interface{}{}
	putStr(out, "correlation_id", f.CorrelationID)
	putStr(out, "message_id", f.MessageID)
	putStr(out, "subject", f.Subject)
	putStr(out, "reply_to", f.ReplyTo)
	putStr(out, "reply_to_session_id", f.ReplyToSessionID)
	putStr(out, "session_id", f.SessionID)
	putStr(out, "content_type", f.ContentType)
	putStr(out, "to", f.To)
	if len(f.ApplicationProperties) > 0 {
		out["application_properties"] = f.ApplicationProperties
	}
	return out
}

func putStr(m map[string]interface{}, key string, v *string) {
	if v != nil {
		m[key] = *v
	}
}

func putInt32(m map[string]interface{}, key string, v *int32) {
	if v != nil {
		m[key] = int(*v)
	}
}

func putBool(m map[string]interface{}, key string, v *bool) {
	if v != nil {
		m[key] = *v
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
func ResourceResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection into the standard list output. A non-nil
// empty slice serialises as [] not null (get-many feeds Loop nodes).
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

// MessagesResult shapes a receive/peek into the standard list output plus the
// received flag.
//
// An empty result is NOT a failure: ReceiveMessages returns as soon as it has
// anything and returns nothing at all if the wait elapsed, so "no messages" is
// the ordinary state of a quiet queue. Reporting it on the error port would
// make every polling flow look broken, so it is a boolean output — the same
// shape as the MQTT node's wait.
func MessagesResult(items []interface{}, verb, entity string) map[string]interface{} {
	out := ListResult(items, "")
	out["received"] = len(items) > 0
	if len(items) == 0 {
		out["tool_result"] = fmt.Sprintf("No messages available on %s", entity)
		return out
	}
	out["tool_result"] = fmt.Sprintf("%s %d message(s) from %s", verb, len(items), entity)
	return out
}

// ListSummary phrases the standard admin list tool_result.
func ListSummary(noun string, count int) string {
	return fmt.Sprintf("Found %d %s(s)", count, noun)
}
