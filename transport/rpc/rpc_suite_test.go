package rpc_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRPC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPC Suite")
}

var _ = Describe("RPC", func() {
	It("should be testable", func() {
		Expect(true).To(BeTrue())
	})
})
