// Shared fixtures for the azureservicebus action tests.
//
// azservicebus ships no fake and no test server: the client dials AMQP 1.0 at
// construction, so there is nothing an httptest server could stand in for.
// The package's Sender/Receiver/Admin interfaces exist for this file — these
// stubs record what each action asked the SDK to do, which is the only way to
// assert that (say) a dead-letter disposition actually dead-letters, or that
// the DLQ is addressed by SubQueue rather than by name.
package azureservicebus_test

import (
	"context"
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

// testConnString is a well-formed namespace-level connection string. The key
// is deliberately distinctive so the redaction tests can look for it.
const (
	testKey        = "AAAAsecretkeyAAAA="
	testConnString = "Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=" + testKey
)

// authInputs builds the credential block with the connection-string default.
func authInputs(extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "connection_string", Type: core.ConnectionTypeSecret, Value: testConnString},
	}, extra...)
}

// entityScopedAuthInputs builds a credential block whose connection string was
// copied from a queue's own policy, and so carries EntityPath.
func entityScopedAuthInputs(entity string, extra ...*core.Connection) []*core.Connection {
	return append([]*core.Connection{
		{Name: "connection_string", Type: core.ConnectionTypeSecret, Value: testConnString + ";EntityPath=" + entity},
	}, extra...)
}

func str(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func text(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: val}
}

func obj(name, jsonStr string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: jsonStr}
}

func boolean(name string, v bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: v}
}

func integer(name string, v int) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: v}
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

// wantSoftFailure asserts the (ErrorResult, nil) soft-failure contract.
func wantSoftFailure(t *testing.T, out map[string]interface{}, err error, contains string) {
	t.Helper()
	if err != nil {
		t.Fatalf("soft failure must return a nil error, got %v", err)
	}
	if out["success"] != false {
		t.Fatalf("success = %v, want false (out: %v)", out["success"], out)
	}
	msg, _ := out["error"].(string)
	if contains != "" && !strings.Contains(msg, contains) {
		t.Fatalf("error %q does not contain %q", msg, contains)
	}
	if summary, _ := out["tool_result"].(string); summary != msg {
		t.Fatalf("tool_result %q should carry the error %q", summary, msg)
	}
}

// wantSuccess asserts the happy-path contract and returns the result map.
func wantSuccess(t *testing.T, out map[string]interface{}, err error) map[string]interface{} {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	if out["error"] != "" {
		t.Fatalf("error = %q, want empty on success", out["error"])
	}
	result, _ := out["result"].(map[string]interface{})
	return result
}

// wantResults asserts a list/receive output and returns the items.
func wantResults(t *testing.T, out map[string]interface{}, err error, count int) []interface{} {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, want true (error: %v)", out["success"], out["error"])
	}
	items, ok := out["results"].([]interface{})
	if !ok {
		t.Fatalf("results is %T, want []interface{} — a nil slice would serialise as null and break a downstream Loop", out["results"])
	}
	if len(items) != count {
		t.Fatalf("got %d result(s), want %d", len(items), count)
	}
	if out["count"] != count {
		t.Fatalf("count = %v, want %d", out["count"], count)
	}
	return items
}

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubSender records every send. sendErr/batchErr/scheduleErr/cancelErr drive
// the failure paths.
type stubSender struct {
	entity string
	auth   sb.Auth

	sent       []*azservicebus.Message
	scheduled  []*azservicebus.Message
	scheduleAt time.Time
	cancelled  []int64
	closed     bool

	batch       *stubBatch
	sendErr     error
	newBatchErr error
	batchErr    error
	scheduleErr error
	scheduleSeq []int64
	cancelErr   error
}

func (s *stubSender) SendMessage(_ context.Context, m *azservicebus.Message, _ *azservicebus.SendMessageOptions) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, m)
	return nil
}

func (s *stubSender) NewMessageBatch(_ context.Context, _ *azservicebus.MessageBatchOptions) (sb.MessageBatch, error) {
	if s.newBatchErr != nil {
		return nil, s.newBatchErr
	}
	if s.batch == nil {
		s.batch = &stubBatch{}
	}
	return s.batch, nil
}

func (s *stubSender) SendMessageBatch(_ context.Context, batch sb.MessageBatch, _ *azservicebus.SendMessageBatchOptions) error {
	if s.batchErr != nil {
		return s.batchErr
	}
	b := batch.(*stubBatch)
	s.sent = append(s.sent, b.added...)
	return nil
}

func (s *stubSender) ScheduleMessages(_ context.Context, msgs []*azservicebus.Message, at time.Time, _ *azservicebus.ScheduleMessagesOptions) ([]int64, error) {
	if s.scheduleErr != nil {
		return nil, s.scheduleErr
	}
	s.scheduled = append(s.scheduled, msgs...)
	s.scheduleAt = at
	return s.scheduleSeq, nil
}

func (s *stubSender) CancelScheduledMessages(_ context.Context, seqs []int64, _ *azservicebus.CancelScheduledMessagesOptions) error {
	if s.cancelErr != nil {
		return s.cancelErr
	}
	s.cancelled = append(s.cancelled, seqs...)
	return nil
}

func (s *stubSender) Close(context.Context) error {
	s.closed = true
	return nil
}

// stubBatch is the size-aware batch. maxMessages simulates the point at which
// the real AddMessage would report ErrMessageTooLarge.
type stubBatch struct {
	added       []*azservicebus.Message
	maxMessages int
}

func (b *stubBatch) AddMessage(m *azservicebus.Message, _ *azservicebus.AddMessageOptions) error {
	if b.maxMessages > 0 && len(b.added) >= b.maxMessages {
		return azservicebus.ErrMessageTooLarge
	}
	b.added = append(b.added, m)
	return nil
}

func (b *stubBatch) NumMessages() int32 { return int32(len(b.added)) }

// settlement records what a receive did with each message, which is the only
// observable difference between the four dispositions.
type settlement struct {
	action  string
	message *azservicebus.ReceivedMessage
	reason  string
	descr   string
}

// stubReceiver records the receive and the settlements.
type stubReceiver struct {
	spec sb.ReceiverSpec
	auth sb.Auth

	messages    []*azservicebus.ReceivedMessage
	deferredFor []int64
	peekOpts    *azservicebus.PeekMessagesOptions
	maxAsked    int
	settled     []settlement
	closed      bool

	receiveErr  error
	peekErr     error
	deferredErr error
	settleErr   error
}

func (r *stubReceiver) ReceiveMessages(_ context.Context, maxMessages int, _ *azservicebus.ReceiveMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	r.maxAsked = maxMessages
	if r.receiveErr != nil {
		return nil, r.receiveErr
	}
	return r.messages, nil
}

func (r *stubReceiver) PeekMessages(_ context.Context, maxMessageCount int, options *azservicebus.PeekMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	r.maxAsked = maxMessageCount
	r.peekOpts = options
	if r.peekErr != nil {
		return nil, r.peekErr
	}
	return r.messages, nil
}

func (r *stubReceiver) ReceiveDeferredMessages(_ context.Context, seqs []int64, _ *azservicebus.ReceiveDeferredMessagesOptions) ([]*azservicebus.ReceivedMessage, error) {
	r.deferredFor = seqs
	if r.deferredErr != nil {
		return nil, r.deferredErr
	}
	return r.messages, nil
}

func (r *stubReceiver) CompleteMessage(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.CompleteMessageOptions) error {
	return r.record("complete", m, "", "")
}

func (r *stubReceiver) AbandonMessage(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.AbandonMessageOptions) error {
	return r.record("abandon", m, "", "")
}

func (r *stubReceiver) DeferMessage(_ context.Context, m *azservicebus.ReceivedMessage, _ *azservicebus.DeferMessageOptions) error {
	return r.record("defer", m, "", "")
}

func (r *stubReceiver) DeadLetterMessage(_ context.Context, m *azservicebus.ReceivedMessage, options *azservicebus.DeadLetterOptions) error {
	var reason, descr string
	if options != nil {
		if options.Reason != nil {
			reason = *options.Reason
		}
		if options.ErrorDescription != nil {
			descr = *options.ErrorDescription
		}
	}
	return r.record("dead_letter", m, reason, descr)
}

func (r *stubReceiver) record(action string, m *azservicebus.ReceivedMessage, reason, descr string) error {
	if r.settleErr != nil {
		return r.settleErr
	}
	r.settled = append(r.settled, settlement{action: action, message: m, reason: reason, descr: descr})
	return nil
}

func (r *stubReceiver) Close(context.Context) error {
	r.closed = true
	return nil
}

// stubSessionReceiver adds the session surface.
type stubSessionReceiver struct {
	stubReceiver
	sessionID   string
	lockedUntil time.Time
	state       []byte
	setState    []byte
	stateErr    error
	setStateErr error
}

func (s *stubSessionReceiver) SessionID() string      { return s.sessionID }
func (s *stubSessionReceiver) LockedUntil() time.Time { return s.lockedUntil }

func (s *stubSessionReceiver) GetSessionState(context.Context, *azservicebus.GetSessionStateOptions) ([]byte, error) {
	return s.state, s.stateErr
}

func (s *stubSessionReceiver) SetSessionState(_ context.Context, state []byte, _ *azservicebus.SetSessionStateOptions) error {
	if s.setStateErr != nil {
		return s.setStateErr
	}
	s.setState = state
	s.state = state
	return nil
}

func (s *stubSessionReceiver) RenewSessionLock(context.Context, *azservicebus.RenewSessionLockOptions) error {
	return nil
}

// stubAdmin records the management calls. Each op has an err field so the
// failure path of every admin action can be driven independently.
type stubAdmin struct {
	auth sb.Auth
	err  error

	namespace admin.NamespaceProperties

	queues       []admin.QueueItem
	queue        admin.QueueProperties
	queueRuntime admin.QueueRuntimeProperties
	createdQueue *admin.QueueProperties
	updatedQueue *admin.QueueProperties
	deletedQueue string
	listedLimit  int

	topics       []admin.TopicItem
	createdTopic *admin.TopicProperties
	deletedTopic string

	// topicMissing / subMissing drive the existence probes the list actions
	// make when a list comes back empty. They default to false — i.e. the
	// parent exists — so the stub keeps its old meaning for every test that
	// does not care.
	topicMissing bool
	subMissing   bool
	existsErr    error

	subscriptions   []admin.SubscriptionPropertiesItem
	subscription    admin.SubscriptionProperties
	subRuntime      admin.SubscriptionRuntimeProperties
	createdSub      *admin.SubscriptionProperties
	deletedSub      string
	listedSubsTopic string

	rules       []admin.RuleProperties
	createdRule *admin.CreateRuleOptions
	deletedRule string
}

func (a *stubAdmin) GetNamespaceProperties(context.Context) (admin.NamespaceProperties, error) {
	return a.namespace, a.err
}

func (a *stubAdmin) CreateQueue(_ context.Context, _ string, props *admin.QueueProperties) (admin.QueueProperties, error) {
	if a.err != nil {
		return admin.QueueProperties{}, a.err
	}
	a.createdQueue = props
	if props == nil {
		return admin.QueueProperties{}, nil
	}
	return *props, nil
}

func (a *stubAdmin) UpdateQueue(_ context.Context, _ string, props admin.QueueProperties) (admin.QueueProperties, error) {
	if a.err != nil {
		return admin.QueueProperties{}, a.err
	}
	a.updatedQueue = &props
	return props, nil
}

func (a *stubAdmin) GetQueue(context.Context, string) (admin.QueueProperties, error) {
	return a.queue, a.err
}

func (a *stubAdmin) GetQueueRuntimeProperties(context.Context, string) (admin.QueueRuntimeProperties, error) {
	return a.queueRuntime, a.err
}

func (a *stubAdmin) DeleteQueue(_ context.Context, name string) error {
	a.deletedQueue = name
	return a.err
}

func (a *stubAdmin) ListQueues(_ context.Context, limit int) ([]admin.QueueItem, error) {
	a.listedLimit = limit
	return a.queues, a.err
}

func (a *stubAdmin) CreateTopic(_ context.Context, _ string, props *admin.TopicProperties) (admin.TopicProperties, error) {
	if a.err != nil {
		return admin.TopicProperties{}, a.err
	}
	a.createdTopic = props
	if props == nil {
		return admin.TopicProperties{}, nil
	}
	return *props, nil
}

func (a *stubAdmin) DeleteTopic(_ context.Context, name string) error {
	a.deletedTopic = name
	return a.err
}

func (a *stubAdmin) ListTopics(_ context.Context, limit int) ([]admin.TopicItem, error) {
	a.listedLimit = limit
	return a.topics, a.err
}

func (a *stubAdmin) TopicExists(context.Context, string) (bool, error) {
	return !a.topicMissing, a.existsErr
}

func (a *stubAdmin) SubscriptionExists(context.Context, string, string) (bool, error) {
	return !a.subMissing, a.existsErr
}

func (a *stubAdmin) CreateSubscription(_ context.Context, _, _ string, props *admin.SubscriptionProperties) (admin.SubscriptionProperties, error) {
	if a.err != nil {
		return admin.SubscriptionProperties{}, a.err
	}
	a.createdSub = props
	if props == nil {
		return admin.SubscriptionProperties{}, nil
	}
	return *props, nil
}

func (a *stubAdmin) DeleteSubscription(_ context.Context, _, sub string) error {
	a.deletedSub = sub
	return a.err
}

func (a *stubAdmin) ListSubscriptions(_ context.Context, topic string, limit int) ([]admin.SubscriptionPropertiesItem, error) {
	a.listedSubsTopic, a.listedLimit = topic, limit
	return a.subscriptions, a.err
}

func (a *stubAdmin) GetSubscriptionRuntimeProperties(context.Context, string, string) (admin.SubscriptionRuntimeProperties, error) {
	return a.subRuntime, a.err
}

func (a *stubAdmin) CreateRule(_ context.Context, _, _ string, opts *admin.CreateRuleOptions) (admin.RuleProperties, error) {
	if a.err != nil {
		return admin.RuleProperties{}, a.err
	}
	a.createdRule = opts
	out := admin.RuleProperties{}
	if opts != nil {
		if opts.Name != nil {
			out.Name = *opts.Name
		}
		out.Filter, out.Action = opts.Filter, opts.Action
	}
	return out, nil
}

func (a *stubAdmin) DeleteRule(_ context.Context, _, _, rule string) error {
	a.deletedRule = rule
	return a.err
}

func (a *stubAdmin) ListRules(context.Context, string, string, int) ([]admin.RuleProperties, error) {
	return a.rules, a.err
}

// ---------------------------------------------------------------------------
// Factory installers
// ---------------------------------------------------------------------------

func withSender(t *testing.T, s *stubSender) {
	t.Helper()
	restore := sb.SetSenderFactoryForTest(func(a sb.Auth, entity string) (sb.Sender, error) {
		s.auth, s.entity = a, entity
		return s, nil
	})
	t.Cleanup(restore)
}

func withSenderError(t *testing.T, err error) {
	t.Helper()
	restore := sb.SetSenderFactoryForTest(func(sb.Auth, string) (sb.Sender, error) { return nil, err })
	t.Cleanup(restore)
}

func withReceiver(t *testing.T, r *stubReceiver) {
	t.Helper()
	restore := sb.SetReceiverFactoryForTest(func(a sb.Auth, spec sb.ReceiverSpec) (sb.Receiver, error) {
		r.auth, r.spec = a, spec
		return r, nil
	})
	t.Cleanup(restore)
}

func withSessionReceiver(t *testing.T, r *stubSessionReceiver, acceptErr error) *string {
	t.Helper()
	accepted := new(string)
	restore := sb.SetSessionFactoryForTest(func(_ context.Context, a sb.Auth, spec sb.ReceiverSpec, sessionID string) (sb.SessionReceiver, error) {
		if acceptErr != nil {
			return nil, acceptErr
		}
		r.auth, r.spec, *accepted = a, spec, sessionID
		return r, nil
	})
	t.Cleanup(restore)
	return accepted
}

func withAdmin(t *testing.T, a *stubAdmin) {
	t.Helper()
	restore := sb.SetAdminFactoryForTest(func(auth sb.Auth) (sb.Admin, error) {
		a.auth = auth
		return a, nil
	})
	t.Cleanup(restore)
}

// ---------------------------------------------------------------------------
// Message fixtures
// ---------------------------------------------------------------------------

func ptr[T any](v T) *T { return &v }

// receivedMessage builds a ReceivedMessage the way the broker would hand one
// back.
func receivedMessage(id, body string) *azservicebus.ReceivedMessage {
	seq := int64(42)
	enqueued := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	locked := enqueued.Add(time.Minute)
	return &azservicebus.ReceivedMessage{
		MessageID:      id,
		Body:           []byte(body),
		DeliveryCount:  1,
		SequenceNumber: &seq,
		EnqueuedTime:   &enqueued,
		LockedUntil:    &locked,
	}
}

// serviceBusError builds the SDK's error type with a given code, so the
// friendly-error mapping can be exercised without a broker. The inner error is
// unexported and unreachable, so the message is always the SDK's "(code):
// unknown error" — the code is the part we map on anyway.
func serviceBusError(code azservicebus.Code) error {
	return &azservicebus.Error{Code: code}
}
