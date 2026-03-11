// Package integration contains end-to-end integration tests for the Aeneas workflow engine.
// These tests require Docker and are tagged with `//go:build integration`.
//
// To run integration tests:
//
//	go test -tags integration ./integration/...
//
// or with Ginkgo:
//
//	ginkgo -r -tags integration ./integration/
package integration_test
