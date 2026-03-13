package rpc_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

func TestRPC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPC Suite")
}

var ctxMatcher = mock.MatchedBy(func(arg any) bool {
	_, ok := arg.(context.Context)
	return ok
})
