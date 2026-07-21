package aws_vpc_create_route

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

// The required-field guards run before any AWS call, so they're testable without
// credentials. create_route needs route_table_id + destination_cidr_block +
// target_type + target_id.
func TestRequiresRouteFields(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("route_table_id"))

	_, err = Execute(nil, nil, []*core.Connection{
		str("route_table_id", "rtb-1"),
		str("destination_cidr_block", "0.0.0.0/0"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("target_type"))

	_, err = Execute(nil, nil, []*core.Connection{
		str("route_table_id", "rtb-1"),
		str("destination_cidr_block", "0.0.0.0/0"),
		str("target_type", "internet_gateway"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("target_id"))
}
