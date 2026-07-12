package mqtt

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
	paho "github.com/eclipse/paho.mqtt.golang"
)

// Second is re-exported so an action can express a wait in seconds without
// importing time purely for the multiplier.
const Second = time.Second

// RetainedWait is the window allowed for the broker to deliver a retained
// message. The broker sends it immediately on subscribe or not at all, so this
// only has to cover the round trip.
const RetainedWait = RetainedWaitSeconds * time.Second

// messageBuffer is how many messages a listener holds before it starts dropping
// them. The waiting actions only ever consume one, but a busy topic can deliver
// several while the filter rejects the earlier ones, and paho invokes the
// handler on its own goroutine — a blocking send there would stall the client's
// packet loop.
const messageBuffer = 32

// MaxPayloadBytes caps what a subscribing action will carry onto its outputs.
// Mirrors the trigger's cap in launch.
const MaxPayloadBytes = 256 * 1024

// Message is a broker message flattened out of paho's interface, so the action
// packages don't all have to import the driver.
type Message struct {
	Topic     string
	Payload   string
	QoS       byte
	Retained  bool
	Truncated bool
}

// Filter decides whether a message satisfies what the caller is waiting for.
// Returning false discards it and keeps listening.
type Filter func(Message) bool

// Listener is an active subscription draining into a buffered channel.
type Listener struct {
	topic string
	ch    chan Message
	once  sync.Once
}

// Listen subscribes to topic and starts buffering matching messages. The caller
// must Close it — the subscription outlives the function that created it
// otherwise, and paho keeps invoking the handler.
func Listen(client paho.Client, topic string, qos byte) (*Listener, error) {
	l := &Listener{topic: topic, ch: make(chan Message, messageBuffer)}

	handler := func(_ paho.Client, m paho.Message) {
		// A subscribed topic is not necessarily one the operator controls — a
		// shared or third-party broker can carry anything, and MQTT permits
		// payloads up to 256MB. Cap what is allowed onto a node's output and into
		// the run history, the same way the trigger does in launch.
		payload := m.Payload()
		truncated := false
		if len(payload) > MaxPayloadBytes {
			payload = payload[:MaxPayloadBytes]
			truncated = true
		}

		msg := Message{
			Topic:     m.Topic(),
			Payload:   string(payload),
			QoS:       m.Qos(),
			Retained:  m.Retained(),
			Truncated: truncated,
		}
		// Never block paho's packet loop: if the buffer is full the flow is not
		// keeping up and the extra messages are of no use to it anyway.
		select {
		case l.ch <- msg:
		default:
		}
	}

	if err := Subscribe(client, topic, qos, handler); err != nil {
		return nil, err
	}

	return l, nil
}

// Await blocks until a message passes the filter, the timeout expires, or the
// flow is cancelled. A nil message with a nil error means "nothing arrived" —
// a normal outcome for a wait, not a failure, so the caller reports it through a
// boolean output rather than the error port.
func (l *Listener) Await(flow *core.Flow, timeout time.Duration, filter Filter) (*Message, error) {
	ctx := gocontext.Context(gocontext.Background())
	if flow != nil {
		if c := flow.GoContext(); c != nil {
			ctx = c
		}
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case msg := <-l.ch:
			if filter != nil && !filter(msg) {
				continue
			}
			return &msg, nil

		case <-deadline.C:
			return nil, nil

		case <-ctx.Done():
			return nil, fmt.Errorf("the flow was cancelled while waiting for a message on %q", l.topic)
		}
	}
}

// Close unsubscribes. Errors are ignored: the connection is about to be torn
// down regardless, and a failed unsubscribe must not mask the action's result.
func (l *Listener) Close(client paho.Client) {
	l.once.Do(func() {
		if client != nil && client.IsConnected() {
			client.Unsubscribe(l.topic).WaitTimeout(SubscribeTimeout)
		}
	})
}

// AwaitMessage is the subscribe-wait-unsubscribe round trip the reading actions
// share, for the cases that don't need to publish in between.
func AwaitMessage(flow *core.Flow, client paho.Client, topic string, qos byte, timeout time.Duration, filter Filter) (*Message, error) {
	listener, err := Listen(client, topic, qos)
	if err != nil {
		return nil, err
	}
	defer listener.Close(client)

	return listener.Await(flow, timeout, filter)
}

// ValidatePublishTopic rejects the subscribe-side wildcards. A broker responds to
// a wildcard publish by dropping the connection with a bare protocol error, so
// catching it here is the difference between a clear message and a baffling one.
func ValidatePublishTopic(topic string) error {
	if strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("the topic %q contains a wildcard (+ or #) — wildcards can only be used when subscribing, not when publishing", topic)
	}
	return nil
}

// ParseTopicList reads the comma-separated topic list the trigger accepts, in
// which each entry may carry its own QoS after a colon ("sensors/#:1,alerts:2").
// This is the syntax MQTT tooling has settled on, so an operator moving a config
// across can paste it verbatim.
func ParseTopicList(raw string, defaultQoS byte) (map[string]byte, error) {
	topics := map[string]byte{}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		topic := entry
		qos := defaultQoS

		// Only split on the LAST colon: a colon is legal inside a topic name, so
		// "ns:sensors/temp" is one topic, not a topic with a QoS of "sensors/temp".
		//
		// A numeric suffix is always taken as a QoS and clamped, rather than only
		// accepting 0-2 literally. Otherwise a typo ("sensors/temp:3") is read as a
		// topic *named* "sensors/temp:3", which subscribes successfully, never
		// matches a real message, and reports nothing wrong.
		if i := strings.LastIndex(entry, ":"); i > 0 {
			if suffix := strings.TrimSpace(entry[i+1:]); isNumeric(suffix) {
				topic = strings.TrimSpace(entry[:i])
				qos = clampQoS(suffix)
			}
		}

		if topic == "" {
			continue
		}
		topics[topic] = qos
	}

	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics to subscribe to — provide at least one, separated by commas")
	}

	return topics, nil
}

// isNumeric reports whether s is a bare integer (a leading "-" allowed, so an
// out-of-range "-1" is still recognised as an attempted QoS and clamped).
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// clampQoS reads a QoS suffix, pinning anything outside 0-2 to 0.
func clampQoS(s string) byte {
	q, err := strconv.Atoi(s)
	if err != nil || q < 0 || q > 2 {
		return 0
	}
	return byte(q)
}
