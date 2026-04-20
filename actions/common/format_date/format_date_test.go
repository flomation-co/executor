package format_date

import (
	"testing"
	"time"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestCurrentTime(t *testing.T) {
	RegisterTestingT(t)
	before := time.Now().Unix()
	out, err := Execute(nil, nil, []*core.Connection{
		conn("output_format", "unix"),
	})
	after := time.Now().Unix()
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	ts, ok := out["unix_timestamp"].(int64)
	Expect(ok).To(BeTrue())
	Expect(ts).To(BeNumerically(">=", before))
	Expect(ts).To(BeNumerically("<=", after))
}

func TestParseISO8601(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19T14:30:00Z"),
		conn("input_format", "iso8601"),
		conn("output_format", "friendly"),
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["result"]).To(Equal("19 Apr 2026, 14:30"))
	Expect(out["year"]).To(Equal(int64(2026)))
	Expect(out["month"]).To(Equal(int64(4)))
	Expect(out["day"]).To(Equal(int64(19)))
	Expect(out["hour"]).To(Equal(int64(14)))
	Expect(out["minute"]).To(Equal(int64(30)))
	Expect(out["day_of_week"]).To(Equal("Sunday"))
}

func TestDateOnly(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19"),
		conn("input_format", "date"),
		conn("output_format", "dmy"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("19/04/2026"))
}

func TestUnixTimestamp(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "1776556800"),
		conn("input_format", "unix"),
		conn("output_format", "date"),
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["result"]).To(Equal("2026-04-19"))
}

func TestTimezone(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19T12:00:00Z"),
		conn("input_format", "iso8601"),
		conn("output_format", "time"),
		conn("timezone", "America/New_York"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("08:00:00"))
}

func TestCustomFormat(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19T14:30:00Z"),
		conn("input_format", "iso8601"),
		conn("output_format", "custom"),
		conn("custom_output_format", "Monday, 02 January 2006"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("Sunday, 19 April 2026"))
}

func TestAutoDetect(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19 14:30:00"),
		conn("output_format", "iso8601"),
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeTrue())
	Expect(out["result"]).To(Equal("2026-04-19T14:30:00Z"))
}

func TestInvalidTimezone(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("timezone", "Not/A/Timezone"),
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("invalid timezone"))
}

func TestInvalidDatetime(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "not a date"),
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("failed to parse"))
}

func TestMDYOutput(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		conn("datetime", "2026-04-19"),
		conn("input_format", "date"),
		conn("output_format", "mdy"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("04/19/2026"))
}
