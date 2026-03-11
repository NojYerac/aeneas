package local_test

import (
	"context"
	"time"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner/local"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("LocalRunner", func() {
	var (
		runner *local.LocalRunner
		logger *logrus.Logger
		ctx    context.Context
	)

	BeforeEach(func() {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		var err error
		runner, err = local.NewLocalRunner(logger)
		Expect(err).ToNot(HaveOccurred())
		Expect(runner).ToNot(BeNil())

		ctx = context.Background()
	})

	AfterEach(func() {
		if runner != nil {
			_ = runner.Close()
		}
	})

	Describe("Execute", func() {
		Context("when executing a simple command", func() {
			It("should execute successfully and return exit code 0", func() {
				// This test requires Docker to be available
				// Tag as integration test if needed
				Skip("Integration test - requires Docker daemon")

				step := domain.StepDefinition{
					Name:    "test-echo",
					Image:   "alpine:latest",
					Command: []string{"echo"},
					Args:    []string{"Hello, World!"},
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("Hello, World!"))
			})
		})

		Context("when command fails", func() {
			It("should return non-zero exit code", func() {
				Skip("Integration test - requires Docker daemon")

				step := domain.StepDefinition{
					Name:    "test-fail",
					Image:   "alpine:latest",
					Command: []string{"sh"},
					Args:    []string{"-c", "exit 42"},
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.ExitCode).To(Equal(42))
			})
		})

		Context("when timeout is specified", func() {
			It("should timeout long-running commands", func() {
				Skip("Integration test - requires Docker daemon")

				step := domain.StepDefinition{
					Name:           "test-timeout",
					Image:          "alpine:latest",
					Command:        []string{"sleep"},
					Args:           []string{"30"},
					TimeoutSeconds: 1,
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				_, err := runner.Execute(ctx, step)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when environment variables are provided", func() {
			It("should pass them to the container", func() {
				Skip("Integration test - requires Docker daemon")

				step := domain.StepDefinition{
					Name:    "test-env",
					Image:   "alpine:latest",
					Command: []string{"sh"},
					Args:    []string{"-c", "echo $TEST_VAR"},
					Env: map[string]string{
						"TEST_VAR": "test_value",
					},
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Logs).To(ContainSubstring("test_value"))
			})
		})
	})

	Describe("NewLocalRunner", func() {
		Context("when Docker is available", func() {
			It("should create a new runner successfully", func() {
				Skip("Integration test - requires Docker daemon")

				r, err := local.NewLocalRunner(logger)
				Expect(err).ToNot(HaveOccurred())
				Expect(r).ToNot(BeNil())

				_ = r.Close()
			})
		})
	})
})
