// Package mqtt provides the shared broker client behind every
// Message Brokers ▸ MQTT action.
//
// MQTT is a protocol, not a hosted product, so unlike the REST providers there
// is no base URL or bearer token: an action dials the broker over TCP (mqtt://),
// TLS (mqtts://) or a WebSocket (ws:// / wss://), optionally authenticates with
// a username/password, does one piece of work, and disconnects. Connections are
// per-execution and never pooled — the same lifecycle the Redis and SQL actions
// use.
//
// Three details of the protocol drive the shape of this file:
//
//   - Client IDs must be unique across everything connected to the broker. Two
//     connections sharing an ID cause the broker to kick the older one off, so
//     the default is a freshly generated ID per execution. A user-supplied
//     client_id is honoured but is only safe for a durable subscription (see
//     the trigger) or a flow that never runs concurrently.
//
//   - A publish is not delivered when Publish() returns. At QoS 1 and 2 the
//     broker acknowledges asynchronously, and disconnecting early drops the
//     message on the floor. Every publish here therefore waits for the token
//     before disconnecting.
//
//   - Retained messages are delivered the instant you subscribe, ahead of any
//     live traffic. That is what makes "read the current value of a topic"
//     possible at all, and it is also why an action that wants the *next* live
//     message has to filter them out.
package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

const (
	// ConnectTimeout bounds the initial dial + CONNACK.
	ConnectTimeout = 15 * time.Second
	// PublishTimeout bounds the wait for a PUBACK/PUBCOMP at QoS 1/2.
	PublishTimeout = 30 * time.Second
	// SubscribeTimeout bounds the wait for a SUBACK.
	SubscribeTimeout = 15 * time.Second
	// DisconnectQuiesce is the grace period (ms) given to in-flight packets.
	DisconnectQuiesce = 500

	// DefaultWaitSeconds is how long the waiting actions listen before giving up.
	DefaultWaitSeconds = 30
	// MaxWaitSeconds caps the wait so a misconfigured node can't pin an executor
	// slot indefinitely.
	MaxWaitSeconds = 300
	// RetainedWaitSeconds is the (short) window allowed for the broker to deliver
	// a retained message, which it does immediately on subscribe or not at all.
	RetainedWaitSeconds = 5

	// DefaultWSPath is the conventional WebSocket endpoint; EMQX, Mosquitto and
	// HiveMQ all default to /mqtt.
	DefaultWSPath = "/mqtt"

	// maxPayloadPreview caps how much of a payload is echoed into tool_result,
	// which is a human-facing summary, not a data channel.
	maxPayloadPreview = 256
)

// Auth is the broker connection config shared by every action. It is assembled
// from the auth block that each action re-declares in its Inputs (the manifest
// generator AST-parses those literals, so they cannot be factored out).
type Auth struct {
	Protocol   string
	Host       string
	Port       int64
	Username   string
	Password   string
	ClientID   string
	WSPath     string
	CACert     string
	ClientCert string
	ClientKey  string
	Insecure   bool
}

// AuthInputs documents the canonical connection block. Action packages
// re-declare their own literal Inputs arrays (the manifest generator AST-parses
// those and cannot follow a variable reference), so this exists for reference
// and to keep the labels/placeholders in one place when they change.
var AuthInputs = []core.Connection{
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: ProtocolOptions},
	{Name: "host", Type: core.ConnectionTypeString, Label: "Broker Host", Placeholder: "broker.example.com — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "1883 (MQTT), 8883 (MQTT over TLS), 8083 (WebSocket)"},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Leave empty for an anonymous broker"},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Broker password"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Leave empty to generate a unique ID per run (recommended)"},
	{Name: "ws_path", Type: core.ConnectionTypeString, Label: "WebSocket Path", Placeholder: "/mqtt", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"ws", "wss"}}},
	{Name: "ca_certificate", Type: core.ConnectionTypeSecret, Label: "CA Certificate", Placeholder: "PEM-encoded CA cert — only needed for a privately-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate", Placeholder: "PEM-encoded client cert — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key", Placeholder: "PEM-encoded client private key — only for mutual-TLS brokers", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip certificate verification — only for a self-signed broker", Visible: &core.VisibleWhen{Field: "protocol", Values: []string{"mqtts", "wss"}}},
}

// ProtocolOptions are the transports paho supports.
var ProtocolOptions = []core.ConnectionOption{
	{Name: "MQTT (TCP)", Value: "mqtt"},
	{Name: "MQTT over TLS", Value: "mqtts"},
	{Name: "MQTT over WebSocket", Value: "ws"},
	{Name: "MQTT over Secure WebSocket", Value: "wss"},
}

// QoSOptions are the three MQTT delivery guarantees, labelled in the terms an
// operator thinks in rather than the spec's numbers.
var QoSOptions = []core.ConnectionOption{
	{Name: "0 — At most once (fire and forget)", Value: "0"},
	{Name: "1 — At least once (acknowledged)", Value: "1"},
	{Name: "2 — Exactly once", Value: "2"},
}

// defaultPorts is the conventional port per transport, used when the operator
// leaves Port empty.
var defaultPorts = map[string]int64{
	"mqtt":  1883,
	"mqtts": 8883,
	"ws":    8083,
	"wss":   8084,
}

// GetAuth reads the connection block out of an action's inputs. A malformed
// connection is a hard error (the node cannot run at all), which is why it
// returns an error rather than an ErrorResult.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	a := Auth{
		Protocol:   strings.ToLower(strings.TrimSpace(OptionalString("protocol", inputs))),
		Host:       strings.TrimSpace(OptionalString("host", inputs)),
		Username:   OptionalString("username", inputs),
		Password:   OptionalString("password", inputs),
		ClientID:   strings.TrimSpace(OptionalString("client_id", inputs)),
		WSPath:     strings.TrimSpace(OptionalString("ws_path", inputs)),
		CACert:     OptionalString("ca_certificate", inputs),
		ClientCert: OptionalString("client_certificate", inputs),
		ClientKey:  OptionalString("client_key", inputs),
		Insecure:   OptionalBool("allow_insecure", inputs),
	}

	if a.Protocol == "" {
		a.Protocol = "mqtt"
	}
	if _, ok := defaultPorts[a.Protocol]; !ok {
		return Auth{}, fmt.Errorf("unsupported protocol %q — expected mqtt, mqtts, ws or wss", a.Protocol)
	}

	// Operators paste broker URLs out of habit; strip the scheme rather than
	// producing "mqtt://mqtt://host".
	a.Host = stripScheme(a.Host)
	if a.Host == "" {
		return Auth{}, fmt.Errorf("broker host is required")
	}

	if port, ok := OptionalInt("port", inputs); ok {
		if port < 1 || port > 65535 {
			return Auth{}, fmt.Errorf("port %d is out of range (1-65535)", port)
		}
		a.Port = port
	} else {
		a.Port = defaultPorts[a.Protocol]
	}

	if a.WSPath == "" {
		a.WSPath = DefaultWSPath
	}
	if !strings.HasPrefix(a.WSPath, "/") {
		a.WSPath = "/" + a.WSPath
	}

	// A cert without its key (or vice versa) is a misconfiguration that would
	// otherwise surface as an opaque TLS handshake failure.
	if (a.ClientCert == "") != (a.ClientKey == "") {
		return Auth{}, fmt.Errorf("mutual TLS needs both a client certificate and a client key")
	}

	return a, nil
}

// stripScheme tolerates a host pasted as a full broker URL.
func stripScheme(host string) string {
	for _, scheme := range []string{"mqtts://", "mqtt://", "wss://", "ws://", "tcp://", "ssl://"} {
		if strings.HasPrefix(strings.ToLower(host), scheme) {
			host = host[len(scheme):]
			break
		}
	}
	// A pasted URL may still carry a path or trailing slash.
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return strings.TrimSpace(host)
}

// BrokerURL renders the connection as the URL paho dials. The WebSocket
// transports carry the endpoint path; the TCP ones must not.
func (a Auth) BrokerURL() string {
	base := fmt.Sprintf("%s://%s:%d", a.Protocol, a.Host, a.Port)
	if a.Protocol == "ws" || a.Protocol == "wss" {
		return base + a.WSPath
	}
	return base
}

// usesTLS reports whether the transport needs a TLS config.
func (a Auth) usesTLS() bool {
	return a.Protocol == "mqtts" || a.Protocol == "wss"
}

// tlsConfig builds the TLS settings for the secure transports. The insecure
// path is strictly opt-in: it is only reachable when the operator ticks
// allow_insecure, so the default can never be silently weakened.
func (a Auth) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: a.Host,
	}

	if a.CACert != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(FormatPEM(a.CACert))) {
			return nil, fmt.Errorf("the CA certificate could not be parsed — expected PEM-encoded text beginning with -----BEGIN CERTIFICATE-----")
		}
		cfg.RootCAs = pool
	}

	if a.ClientCert != "" && a.ClientKey != "" {
		pair, err := tls.X509KeyPair([]byte(FormatPEM(a.ClientCert)), []byte(FormatPEM(a.ClientKey)))
		if err != nil {
			return nil, fmt.Errorf("the client certificate and key could not be loaded: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}

	if a.Insecure {
		// #nosec G402 — opt-in only, gated behind the allow_insecure input so a
		// self-signed broker can be used deliberately. The default config above
		// always verifies.
		cfg.InsecureSkipVerify = true
	}

	return cfg, nil
}

// ClientOptions builds the paho options for this connection. clientIDSuffix
// distinguishes the generated IDs of different actions in broker logs; when the
// operator has pinned a client_id it is used verbatim.
//
// cleanSession=false asks the broker to remember the client's subscriptions and
// queue its QoS 1/2 messages while it is away — the property the trigger relies
// on to survive a reconnect without dropping messages. The one-shot actions pass
// true: they have no subscription worth remembering, and a durable session for a
// throwaway client ID would leak state on the broker.
func (a Auth) ClientOptions(clientIDSuffix string, cleanSession bool) (*paho.ClientOptions, error) {
	opts := paho.NewClientOptions()
	opts.AddBroker(a.BrokerURL())

	if a.ClientID != "" {
		opts.SetClientID(a.ClientID)
	} else {
		// MQTT 3.1.1 allows 65535 bytes of client ID, but brokers in the wild
		// still enforce the 23-byte 3.1 limit, so keep it short.
		opts.SetClientID(fmt.Sprintf("flo-%s-%s", clientIDSuffix, uuid.NewString()[:8]))
	}

	if a.Username != "" {
		opts.SetUsername(a.Username)
	}
	if a.Password != "" {
		opts.SetPassword(a.Password)
	}

	if a.usesTLS() {
		cfg, err := a.tlsConfig()
		if err != nil {
			return nil, err
		}
		opts.SetTLSConfig(cfg)
	}

	opts.SetCleanSession(cleanSession)
	opts.SetConnectTimeout(ConnectTimeout)
	opts.SetOrderMatters(false)
	// The one-shot actions manage their own lifecycle and must surface a dial
	// failure to the operator rather than retrying invisibly behind a timeout.
	opts.SetAutoReconnect(false)
	opts.SetConnectRetry(false)

	return opts, nil
}

// Connect dials the broker and blocks until the CONNACK arrives. The returned
// client is always the caller's to Disconnect.
func Connect(a Auth, clientIDSuffix string) (paho.Client, error) {
	opts, err := a.ClientOptions(clientIDSuffix, true)
	if err != nil {
		return nil, err
	}

	client := paho.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(ConnectTimeout) {
		client.Disconnect(0)
		return nil, fmt.Errorf("timed out after %s connecting to %s", ConnectTimeout, a.BrokerURL())
	}
	if err := token.Error(); err != nil {
		client.Disconnect(0)
		return nil, fmt.Errorf("could not connect to %s: %s", a.BrokerURL(), Redact(a, err.Error()))
	}

	return client, nil
}

// Disconnect closes the client, allowing in-flight packets to drain.
func Disconnect(client paho.Client) {
	if client != nil && client.IsConnected() {
		client.Disconnect(DisconnectQuiesce)
	}
}

// Publish sends one message and waits for the broker to acknowledge it. The
// wait is what makes QoS 1 and 2 mean anything: without it, Disconnect races the
// PUBACK and the message is silently lost.
func Publish(client paho.Client, topic string, qos byte, retain bool, payload string) error {
	token := client.Publish(topic, qos, retain, []byte(payload))
	if !token.WaitTimeout(PublishTimeout) {
		return fmt.Errorf("timed out after %s waiting for the broker to acknowledge the message on %q — the broker may not permit QoS %d on this topic", PublishTimeout, topic, qos)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish to %q failed: %s", topic, err.Error())
	}
	return nil
}

// Subscribe registers handler for topic and waits for the SUBACK, so a rejected
// subscription (an ACL denial, typically) is reported rather than hanging.
func Subscribe(client paho.Client, topic string, qos byte, handler paho.MessageHandler) error {
	token := client.Subscribe(topic, qos, handler)
	if !token.WaitTimeout(SubscribeTimeout) {
		return fmt.Errorf("timed out after %s subscribing to %q", SubscribeTimeout, topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("could not subscribe to %q: %s — the broker may not grant this client access to the topic", topic, err.Error())
	}
	return nil
}

// Redact strips the broker password out of a message before it can reach a log
// line or an error output.
func Redact(a Auth, msg string) string {
	if a.Password == "" {
		return msg
	}
	return strings.ReplaceAll(msg, a.Password, "********")
}

// ParseQoS reads a qos input, defaulting to 0. An out-of-range value is clamped
// rather than rejected, matching how brokers treat it.
func ParseQoS(name string, inputs []*core.Connection) byte {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return 0
	}
	q, err := strconv.Atoi(raw)
	if err != nil || q < 0 || q > 2 {
		return 0
	}
	return byte(q)
}

// WaitSeconds reads a timeout input, applying the default and the ceiling.
func WaitSeconds(name string, inputs []*core.Connection, fallback int64) time.Duration {
	secs := fallback
	if v, ok := OptionalInt(name, inputs); ok && v > 0 {
		secs = v
	}
	if secs > MaxWaitSeconds {
		secs = MaxWaitSeconds
	}
	return time.Duration(secs) * time.Second
}

// -- input helpers -----------------------------------------------------------
//
// These wrap the core accessors rather than calling them directly. Number() and
// Boolean() are strictly type-gated and Number() will panic on an unset integer
// input, so every read goes through a nil-guard here.

// OptionalString returns an input's value, or "" when it is absent or empty.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return ""
	}
	v := conn.String()
	if v == nil {
		return ""
	}
	return *v
}

// RequiredString returns an input's value, or an error naming the empty field.
func RequiredString(name, label string, inputs []*core.Connection) (string, error) {
	v := strings.TrimSpace(OptionalString(name, inputs))
	if v == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return v, nil
}

// OptionalInt returns an integer input and whether it was set. The nil-guard on
// Value is load-bearing: core's Number() type-asserts Value to a string on its
// final fallback and panics when the input was never filled in.
func OptionalInt(name string, inputs []*core.Connection) (int64, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return 0, false
	}
	if s, ok := conn.Value.(string); ok {
		if strings.TrimSpace(s) == "" {
			return 0, false
		}
		// A variable-bound integer input arrives as a string.
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	v := conn.Number()
	if v == nil {
		return 0, false
	}
	return *v, true
}

// OptionalBool returns a boolean input, defaulting to false. Like OptionalInt it
// handles the string form a variable-bound checkbox produces, which core's
// Boolean() rejects outright.
func OptionalBool(name string, inputs []*core.Connection) bool {
	return BoolWithDefault(name, inputs, false)
}

// BoolWithDefault returns a boolean input, or fallback when it is unset. The
// distinction matters for the options that default to true — an unticked box and
// an untouched one are the same value on the wire, so the default has to come
// from the caller.
func BoolWithDefault(name string, inputs []*core.Connection, fallback bool) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return fallback
	}
	if b, ok := conn.Value.(bool); ok {
		return b
	}
	if s, ok := conn.Value.(string); ok {
		if strings.TrimSpace(s) == "" {
			return fallback
		}
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return fallback
		}
		return b
	}
	if b := conn.Boolean(); b != nil {
		return *b
	}
	return fallback
}

// -- result shaping ----------------------------------------------------------

// ErrorResult is the soft-failure shape: a populated map and a nil error, so the
// engine routes it down the node's error port as data instead of failing the run.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// MessageResult shapes a received message into the outputs every reading action
// shares. parseJSON additionally decodes the payload into payload_json so a
// downstream node can address individual fields without a JSON node in between.
func MessageResult(topic, payload string, qos byte, retained bool, parseJSON bool) map[string]interface{} {
	out := map[string]interface{}{
		"topic":       topic,
		"payload":     payload,
		"qos":         int64(qos),
		"retained":    retained,
		"received_at": time.Now().UTC().Format(time.RFC3339),
	}

	if parseJSON {
		var decoded interface{}
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			out["payload_json"] = decoded
		}
	}

	return out
}

// pemBlock matches a PEM whose newlines have been flattened away.
var pemBlock = regexp.MustCompile(`(-----BEGIN [A-Z0-9 ]+-----)\s*(.*?)\s*(-----END [A-Z0-9 ]+-----)`)

// FormatPEM repairs a certificate whose line breaks were lost. Pasting a cert
// through a single-line form field or carrying it in an environment variable
// flattens it to one line, which every PEM parser rejects — with an error that
// says nothing about newlines. Rebuilding the 64-character body lines turns a
// baffling TLS failure into a working connection.
func FormatPEM(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "\n") {
		// Already multi-line: leave a valid PEM exactly as it is.
		return s
	}

	return pemBlock.ReplaceAllStringFunc(s, func(block string) string {
		parts := pemBlock.FindStringSubmatch(block)
		if len(parts) != 4 {
			return block
		}

		header, body, footer := parts[1], strings.ReplaceAll(parts[2], " ", ""), parts[3]

		var b strings.Builder
		b.WriteString(header)
		b.WriteString("\n")
		for i := 0; i < len(body); i += 64 {
			end := i + 64
			if end > len(body) {
				end = len(body)
			}
			b.WriteString(body[i:end])
			b.WriteString("\n")
		}
		b.WriteString(footer)

		return b.String()
	})
}

// Preview truncates a payload for the human-facing tool_result summary.
func Preview(payload string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return "(empty)"
	}
	if len(payload) > maxPayloadPreview {
		return payload[:maxPayloadPreview] + "…"
	}
	return payload
}
