package environment

import (
	"fmt"
	. "github.com/onsi/gomega"
	"net/url"
	"testing"
)

func Test_ParsingUrl(t *testing.T) {
	RegisterTestingT(t)

	u, err := url.Parse("https://www.google.com?test=true")
	Expect(err).To(BeNil())
	Expect(u).To(Not(BeNil()))

	q := u.Query()
	q.Set("token", "blah")

	fmt.Printf("URL: %v\nHost: %v\nQuery: %v\n", u.String(), u.Host, q.Encode())
}
