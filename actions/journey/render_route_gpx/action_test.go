package journey_render_route_gpx

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, []*core.Connection{
		conn("polyline", "_p~iF~ps|U_ulLnnqC_mqNvxq`@"),
		conn("name", "Test route"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["point_count"]).To(Equal("3"))

	xml := out["gpx_xml"].(string)
	Expect(xml).To(ContainSubstring("<?xml"))
	Expect(xml).To(ContainSubstring("<gpx"))
	Expect(xml).To(ContainSubstring("creator=\"Flomation\""))
	Expect(xml).To(ContainSubstring("<name>Test route</name>"))
	Expect(strings.Count(xml, "<trkpt")).To(Equal(3))
	Expect(out["gpx_base64"]).ToNot(BeEmpty())
}

func TestExecuteEmptyPolyline(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		conn("polyline", "  "),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("polyline"))
}

func TestExecuteInvalidPolyline(t *testing.T) {
	RegisterTestingT(t)

	// A single garbage character won't decode into any complete coordinate.
	out, err := Execute(nil, nil, []*core.Connection{
		conn("polyline", "X"),
	})
	Expect(err).To(HaveOccurred())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("zero points"))
}
