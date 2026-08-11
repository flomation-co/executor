package download

import (
	"net"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsDisallowedIP(t *testing.T) {
	RegisterTestingT(t)

	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"10.1.2.3",        // private
		"192.168.0.5",     // private
		"172.16.9.9",      // private
		"169.254.169.254", // link-local (cloud metadata)
		"0.0.0.0",         // unspecified
		"224.0.0.1",       // multicast
		"fe80::1",         // link-local v6
	}
	for _, s := range blocked {
		Expect(isDisallowedIP(net.ParseIP(s))).To(BeTrue(), "expected %s to be blocked", s)
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1::1"}
	for _, s := range allowed {
		Expect(isDisallowedIP(net.ParseIP(s))).To(BeFalse(), "expected %s to be allowed", s)
	}
}

func TestDeriveFilename(t *testing.T) {
	RegisterTestingT(t)

	mustURL := func(s string) *url.URL { u, _ := url.Parse(s); return u }

	// Explicit filename wins.
	Expect(deriveFilename("demo.mp4", mustURL("https://x/y/z"), "video/mp4")).To(Equal("demo.mp4"))
	// Otherwise the URL's last segment (with an extension).
	Expect(deriveFilename("", mustURL("https://cdn.heygen.com/abc/final.mp4?sig=1"), "video/mp4")).To(Equal("final.mp4"))
	// No usable path segment -> generated name from the MIME type.
	Expect(deriveFilename("", mustURL("https://cdn.heygen.com/download"), "video/mp4")).To(HavePrefix("download."))
}

func TestCleanMime(t *testing.T) {
	RegisterTestingT(t)

	Expect(cleanMime("video/mp4")).To(Equal("video/mp4"))
	Expect(cleanMime("video/mp4; charset=binary")).To(Equal("video/mp4"))
	Expect(cleanMime("")).To(Equal("application/octet-stream"))
}
