package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestConnectionTypeString(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: "abcdef12345",
	}

	Expect(c.String()).To(Not(BeNil()))

	Expect(c.Number()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
}

func TestConnectionTypeBadString(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeString,
		Value: 1234,
	}

	Expect(c.String()).To(BeNil())
	Expect(c.Number()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
}

func TestConnectionTypeNumber(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: 1234,
	}

	Expect(c.Number()).To(Not(BeNil()))

	Expect(c.String()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())
}

func TestConnectionTypeBadNumber(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: "1234",
	}

	Expect(c.Number()).To(Not(BeNil()))

	Expect(c.String()).To(BeNil())
	Expect(c.Boolean()).To(BeNil())

	c2 := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: "abcd",
	}

	Expect(c2.Number()).To(BeNil())
	Expect(c2.String()).To(BeNil())
	Expect(c2.Boolean()).To(BeNil())

	c3 := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeInteger,
		Value: 12.3,
	}

	Expect(c3.Number()).To(Not(BeNil()))

	Expect(c3.String()).To(BeNil())
	Expect(c3.Boolean()).To(BeNil())
}

func TestConnectionTypeBoolean(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeBoolean,
		Value: false,
	}

	Expect(c.Boolean()).To(Not(BeNil()))

	Expect(c.String()).To(BeNil())
	Expect(c.Number()).To(BeNil())
}

func TestConnectionTypeBadBoolean(t *testing.T) {
	RegisterTestingT(t)

	c := Connection{
		Name:  "test-connection",
		Type:  ConnectionTypeBoolean,
		Value: "abc",
	}

	Expect(c.Boolean()).To(BeNil())
	Expect(c.String()).To(BeNil())
	Expect(c.Number()).To(BeNil())
}

func TestFindConnection(t *testing.T) {
	RegisterTestingT(t)

	connections := []*Connection{
		&Connection{
			Name:  "connection1",
			Type:  ConnectionTypeString,
			Value: "value",
		},
	}

	result := FindConnection("connection1", connections)
	Expect(result).To(Not(BeNil()))
	Expect(result).To(Equal(connections[0]))

	bad := FindConnection("missing-connection-name", connections)
	Expect(bad).To(BeNil())
}
