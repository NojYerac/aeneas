package http_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHTTP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Suite")
}

var _ = Describe("HTTP", func() {
	It("should be testable", func() {
		Expect(true).To(BeTrue())
	})
})
