package mqtt

import (
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"
)

func conn(name, typ string, value interface{}) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: value}
}

func TestGetAuthDefaultsPortPerProtocol(t *testing.T) {
	cases := map[string]int64{"mqtt": 1883, "mqtts": 8883, "ws": 8083, "wss": 8084}

	for protocol, wantPort := range cases {
		inputs := []*core.Connection{
			conn("protocol", core.ConnectionTypeString, protocol),
			conn("host", core.ConnectionTypeString, "broker.example.com"),
		}

		auth, err := GetAuth(inputs)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", protocol, err)
		}
		if auth.Port != wantPort {
			t.Errorf("%s: port = %d, want %d", protocol, auth.Port, wantPort)
		}
	}
}

func TestGetAuthStripsPastedScheme(t *testing.T) {
	inputs := []*core.Connection{
		conn("protocol", core.ConnectionTypeString, "mqtt"),
		conn("host", core.ConnectionTypeString, "mqtt://broker.example.com/"),
	}

	auth, err := GetAuth(inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth.Host != "broker.example.com" {
		t.Errorf("host = %q, want broker.example.com", auth.Host)
	}
	if got := auth.BrokerURL(); got != "mqtt://broker.example.com:1883" {
		t.Errorf("BrokerURL() = %q", got)
	}
}

func TestBrokerURLCarriesPathOnlyForWebSockets(t *testing.T) {
	tcp := Auth{Protocol: "mqtt", Host: "h", Port: 1883, WSPath: "/mqtt"}
	if got := tcp.BrokerURL(); got != "mqtt://h:1883" {
		t.Errorf("tcp BrokerURL() = %q, want no path", got)
	}

	ws := Auth{Protocol: "ws", Host: "h", Port: 8083, WSPath: "/mqtt"}
	if got := ws.BrokerURL(); got != "ws://h:8083/mqtt" {
		t.Errorf("ws BrokerURL() = %q, want the endpoint path", got)
	}
}

func TestGetAuthRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name   string
		inputs []*core.Connection
	}{
		{"unknown protocol", []*core.Connection{
			conn("protocol", core.ConnectionTypeString, "amqp"),
			conn("host", core.ConnectionTypeString, "h"),
		}},
		{"missing host", []*core.Connection{
			conn("protocol", core.ConnectionTypeString, "mqtt"),
		}},
		{"port out of range", []*core.Connection{
			conn("protocol", core.ConnectionTypeString, "mqtt"),
			conn("host", core.ConnectionTypeString, "h"),
			conn("port", core.ConnectionTypeInteger, int64(70000)),
		}},
		{"client cert without key", []*core.Connection{
			conn("protocol", core.ConnectionTypeString, "mqtts"),
			conn("host", core.ConnectionTypeString, "h"),
			conn("client_certificate", core.ConnectionTypeSecret, "cert"),
		}},
	}

	for _, tc := range cases {
		if _, err := GetAuth(tc.inputs); err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
		}
	}
}

// An unset integer input is the shape that panics core's Number(), so the
// helper must never reach it.
func TestOptionalIntSurvivesUnsetAndVariableBoundInputs(t *testing.T) {
	inputs := []*core.Connection{
		conn("unset", core.ConnectionTypeInteger, nil),
		conn("empty", core.ConnectionTypeInteger, ""),
		conn("native", core.ConnectionTypeInteger, int64(8883)),
		conn("from_variable", core.ConnectionTypeInteger, "8883"),
	}

	if _, ok := OptionalInt("unset", inputs); ok {
		t.Error("unset integer reported as set")
	}
	if _, ok := OptionalInt("empty", inputs); ok {
		t.Error("empty integer reported as set")
	}
	if _, ok := OptionalInt("missing", inputs); ok {
		t.Error("absent integer reported as set")
	}
	if v, ok := OptionalInt("native", inputs); !ok || v != 8883 {
		t.Errorf("native = %d, %v", v, ok)
	}
	if v, ok := OptionalInt("from_variable", inputs); !ok || v != 8883 {
		t.Errorf("variable-bound = %d, %v — core's Number() drops the string form", v, ok)
	}
}

// A checkbox bound to a variable arrives as a string, which core's Boolean()
// rejects outright.
func TestBoolWithDefaultHandlesStringAndDefault(t *testing.T) {
	inputs := []*core.Connection{
		conn("native_true", core.ConnectionTypeBoolean, true),
		conn("string_false", core.ConnectionTypeBoolean, "false"),
		conn("string_true", core.ConnectionTypeBoolean, "true"),
		conn("unset", core.ConnectionTypeBoolean, nil),
	}

	if !BoolWithDefault("native_true", inputs, false) {
		t.Error("native true not read")
	}
	if BoolWithDefault("string_false", inputs, true) {
		t.Error(`"false" from a variable not read`)
	}
	if !BoolWithDefault("string_true", inputs, false) {
		t.Error(`"true" from a variable not read`)
	}
	if !BoolWithDefault("unset", inputs, true) {
		t.Error("unset input did not fall back to the default")
	}
	if !BoolWithDefault("absent", inputs, true) {
		t.Error("absent input did not fall back to the default")
	}
}

func TestParseQoSDefaultsAndClamps(t *testing.T) {
	inputs := []*core.Connection{
		conn("zero", core.ConnectionTypeString, "0"),
		conn("one", core.ConnectionTypeString, "1"),
		conn("two", core.ConnectionTypeString, "2"),
		conn("silly", core.ConnectionTypeString, "9"),
		conn("junk", core.ConnectionTypeString, "high"),
		conn("empty", core.ConnectionTypeString, ""),
	}

	for name, want := range map[string]byte{
		"zero": 0, "one": 1, "two": 2,
		"silly": 0, "junk": 0, "empty": 0, "absent": 0,
	} {
		if got := ParseQoS(name, inputs); got != want {
			t.Errorf("ParseQoS(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestWaitSecondsAppliesDefaultAndCeiling(t *testing.T) {
	inputs := []*core.Connection{
		conn("set", core.ConnectionTypeInteger, int64(45)),
		conn("huge", core.ConnectionTypeInteger, int64(99999)),
		conn("zero", core.ConnectionTypeInteger, int64(0)),
	}

	if got := WaitSeconds("set", inputs, 30); got != 45*time.Second {
		t.Errorf("set = %s", got)
	}
	if got := WaitSeconds("huge", inputs, 30); got != MaxWaitSeconds*time.Second {
		t.Errorf("huge = %s, want the %ds ceiling", got, MaxWaitSeconds)
	}
	if got := WaitSeconds("zero", inputs, 30); got != 30*time.Second {
		t.Errorf("zero = %s, want the default", got)
	}
	if got := WaitSeconds("absent", inputs, 30); got != 30*time.Second {
		t.Errorf("absent = %s, want the default", got)
	}
}

func TestValidatePublishTopicRejectsWildcards(t *testing.T) {
	if err := ValidatePublishTopic("sensors/kitchen/temp"); err != nil {
		t.Errorf("a concrete topic was rejected: %v", err)
	}
	for _, topic := range []string{"sensors/+/temp", "sensors/#", "#"} {
		if err := ValidatePublishTopic(topic); err == nil {
			t.Errorf("wildcard topic %q was accepted for publishing", topic)
		}
	}
}

func TestParseTopicList(t *testing.T) {
	topics, err := ParseTopicList("sensors/#:1, alerts:2 ,plain", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]byte{"sensors/#": 1, "alerts": 2, "plain": 0}
	if len(topics) != len(want) {
		t.Fatalf("got %d topics, want %d: %v", len(topics), len(want), topics)
	}
	for topic, qos := range want {
		if topics[topic] != qos {
			t.Errorf("%s: qos = %d, want %d", topic, topics[topic], qos)
		}
	}
}

func TestParseTopicListAppliesDefaultQoS(t *testing.T) {
	topics, err := ParseTopicList("a,b:2", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topics["a"] != 1 {
		t.Errorf("a inherited qos %d, want the default 1", topics["a"])
	}
	if topics["b"] != 2 {
		t.Errorf("b: qos = %d, want its own 2", topics["b"])
	}
}

// A colon is legal inside a topic name, so only a trailing :0-2 is a QoS.
func TestParseTopicListOnlyTreatsTrailingDigitAsQoS(t *testing.T) {
	topics, err := ParseTopicList("ns:sensors/temp", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := topics["ns:sensors/temp"]; !ok {
		t.Errorf("a colon inside the topic name was mistaken for a QoS suffix: %v", topics)
	}
}

func TestParseTopicListRejectsEmpty(t *testing.T) {
	if _, err := ParseTopicList("  , ,", 0); err == nil {
		t.Error("expected an error for a list with no topics")
	}
}

func TestRedactHidesThePassword(t *testing.T) {
	a := Auth{Password: "hunter2"}
	got := Redact(a, "auth failed for user dave with password hunter2")
	if got != "auth failed for user dave with password ********" {
		t.Errorf("password survived redaction: %q", got)
	}

	// An empty password must not turn every message into asterisks.
	if got := Redact(Auth{}, "connection refused"); got != "connection refused" {
		t.Errorf("empty password mangled the message: %q", got)
	}
}

func TestMessageResultParsesJSONOnlyWhenAsked(t *testing.T) {
	out := MessageResult("t", `{"temp":21.5}`, 1, false, true)
	decoded, ok := out["payload_json"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload_json missing or wrong type: %#v", out["payload_json"])
	}
	if decoded["temp"] != 21.5 {
		t.Errorf("temp = %v", decoded["temp"])
	}

	plain := MessageResult("t", `{"temp":21.5}`, 1, false, false)
	if _, exists := plain["payload_json"]; exists {
		t.Error("payload_json produced without parse_json set")
	}

	// Unparseable payloads must not fail the action, just skip the decode.
	bad := MessageResult("t", "not json", 0, false, true)
	if _, exists := bad["payload_json"]; exists {
		t.Error("payload_json produced for a non-JSON payload")
	}
	if bad["payload"] != "not json" {
		t.Errorf("raw payload lost: %v", bad["payload"])
	}
}

func TestTLSConfigRequiresParseableCA(t *testing.T) {
	a := Auth{Protocol: "mqtts", Host: "h", CACert: "clearly not a certificate"}
	if _, err := a.tlsConfig(); err == nil {
		t.Error("a junk CA certificate was accepted")
	}
}

func TestTLSConfigVerifiesByDefault(t *testing.T) {
	a := Auth{Protocol: "mqtts", Host: "broker.example.com"}
	cfg, err := a.tlsConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("TLS verification was off without allow_insecure being set")
	}
	if cfg.ServerName != "broker.example.com" {
		t.Errorf("ServerName = %q", cfg.ServerName)
	}

	insecure := Auth{Protocol: "mqtts", Host: "h", Insecure: true}
	cfg, err = insecure.tlsConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Error("allow_insecure did not disable verification")
	}
}

// A typo in the QoS suffix must clamp, not silently create a topic literally
// named "sensors/temp:3" that subscribes fine and never matches anything.
func TestParseTopicListClampsOutOfRangeQoS(t *testing.T) {
	topics, err := ParseTopicList("sensors/temp:3,alerts:-1,ok:2", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, dead := topics["sensors/temp:3"]; dead {
		t.Error(`"sensors/temp:3" was taken as a topic NAME — the subscription would never fire`)
	}
	if q, ok := topics["sensors/temp"]; !ok || q != 0 {
		t.Errorf("sensors/temp: qos = %d, present = %v; want it clamped to 0", q, ok)
	}
	if q, ok := topics["alerts"]; !ok || q != 0 {
		t.Errorf("alerts: qos = %d, present = %v; want a negative QoS clamped to 0", q, ok)
	}
	if topics["ok"] != 2 {
		t.Errorf("a valid QoS was disturbed: %d", topics["ok"])
	}
}

func TestFormatPEMRepairsAFlattenedCertificate(t *testing.T) {
	flat := "-----BEGIN CERTIFICATE----- QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVphYmNkZWZnaGlqa2xtbm9wcXJzdHV2d3h5ejAxMjM0NTY3ODk= -----END CERTIFICATE-----"

	got := FormatPEM(flat)
	if !strings.HasPrefix(got, "-----BEGIN CERTIFICATE-----\n") {
		t.Errorf("no newline after the header:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n-----END CERTIFICATE-----") {
		t.Errorf("no newline before the footer:\n%s", got)
	}

	valid := "-----BEGIN CERTIFICATE-----\nQUJDRA==\n-----END CERTIFICATE-----"
	if FormatPEM(valid) != valid {
		t.Error("a valid PEM was mangled")
	}
}

// The subscribing actions must not carry an unbounded payload onto a node's
// outputs — a subscribed topic is not necessarily one the operator controls.
func TestMaxPayloadBytesIsEnforcedOnTheMessageShape(t *testing.T) {
	if MaxPayloadBytes <= 0 || MaxPayloadBytes > 1<<20 {
		t.Errorf("MaxPayloadBytes = %d — expected a sane cap", MaxPayloadBytes)
	}
}
