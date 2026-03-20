package conditional_if

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func Test_Equals_True(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "equals"),
		strConn("value_b", "hello"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))
}

func Test_Equals_False(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "equals"),
		strConn("value_b", "world"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(false))
}

func Test_NotEquals(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "not_equals"),
		strConn("value_b", "world"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))
}

func Test_Contains_True(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello world"),
		strConn("operator", "contains"),
		strConn("value_b", "world"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))
}

func Test_Contains_False(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello world"),
		strConn("operator", "contains"),
		strConn("value_b", "xyz"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(false))
}

func Test_GreaterThan(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "10"),
		strConn("operator", "greater_than"),
		strConn("value_b", "5"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))

	result, err = Execute(nil, nil, []*core.Connection{
		strConn("value_a", "3"),
		strConn("operator", "greater_than"),
		strConn("value_b", "5"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(false))
}

func Test_LessThan(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "3"),
		strConn("operator", "less_than"),
		strConn("value_b", "5"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))
}

func Test_IsEmpty(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", ""),
		strConn("operator", "is_empty"),
		strConn("value_b", ""),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))

	result, err = Execute(nil, nil, []*core.Connection{
		strConn("value_a", "   "),
		strConn("operator", "is_empty"),
		strConn("value_b", ""),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))

	result, err = Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "is_empty"),
		strConn("value_b", ""),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(false))
}

func Test_IsNotEmpty(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "is_not_empty"),
		strConn("value_b", ""),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(true))
}

func Test_UnknownOperator(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("operator", "invalid"),
		strConn("value_b", "world"),
	})
	Expect(err).ToNot(BeNil())
}

func Test_MissingOperator(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "hello"),
		strConn("value_b", "world"),
	})
	Expect(err).ToNot(BeNil())
}

func Test_GreaterThan_NonNumeric(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("value_a", "abc"),
		strConn("operator", "greater_than"),
		strConn("value_b", "5"),
	})
	Expect(err).To(BeNil())
	Expect(result["result"]).To(Equal(false))
}
