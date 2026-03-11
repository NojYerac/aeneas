//go:build integration

package local_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner/local"
)

func TestLocalRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Runner Suite")
}

var _ = Describe("LocalRunner", func() {
	var (
		runner *local.LocalRunner
		logger *logrus.Logger
		ctx    context.Context
	)

	BeforeEach(func() {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests

		var err error
		runner, err = local.NewLocalRunner(logger)
		Expect(err).NotTo(HaveOccurred())
		Expect(runner).NotTo(BeNil())

		ctx = context.Background()
	})

	AfterEach(func() {
		if runner != nil {
			_ = runner.Close()
		}
	})

	Describe("Execute", func() {
		Context("with a simple successful command", func() {
			It("should execute and return exit code 0", func() {
				step := &domain.StepDefinition{
					Name:           "test-echo",
					Image:          "alpine:latest",
					Command:        []string{"echo"},
					Args:           []string{"hello world"},
					Env:            nil,
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("hello world"))
			})
		})

		Context("with a failing command", func() {
			It("should return non-zero exit code", func() {
				step := &domain.StepDefinition{
					Name:           "test-fail",
					Image:          "alpine:latest",
					Command:        []string{"sh"},
					Args:           []string{"-c", "exit 42"},
					Env:            nil,
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(42))
			})
		})

		Context("with environment variables", func() {
			It("should pass environment variables to the container", func() {
				step := &domain.StepDefinition{
					Name:    "test-env",
					Image:   "alpine:latest",
					Command: []string{"sh"},
					Args:    []string{"-c", "echo $TEST_VAR"},
					Env: map[string]string{
						"TEST_VAR": "test-value",
					},
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("test-value"))
			})
		})

		Context("with a timeout", func() {
			It("should cancel execution when timeout is exceeded", func() {
				step := &domain.StepDefinition{
					Name:           "test-timeout",
					Image:          "alpine:latest",
					Command:        []string{"sleep"},
					Args:           []string{"10"},
					Env:            nil,
					TimeoutSeconds: 1, // 1 second timeout
				}

				start := time.Now()
				_, err := runner.Execute(ctx, step)
				duration := time.Since(start)

				// Should error due to timeout
				Expect(err).To(HaveOccurred())
				// Should take roughly the timeout duration (with some margin)
				Expect(duration).To(BeNumerically("~", 1*time.Second, 500*time.Millisecond))
			})
		})

		Context("with an image that needs to be pulled", func() {
			It("should pull the image and execute successfully", func() {
				// Use a small, specific image that's unlikely to be cached
				step := &domain.StepDefinition{
					Name:           "test-pull",
					Image:          "busybox:1.36",
					Command:        []string{"echo"},
					Args:           []string{"pulled and executed"},
					Env:            nil,
					TimeoutSeconds: 60, // Allow time for pull
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("pulled and executed"))
			})
		})
	})

	Describe("NewLocalRunner", func() {
		Context("when Docker is available", func() {
			It("should create a runner successfully", func() {
				r, err := local.NewLocalRunner(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(r).NotTo(BeNil())
				_ = r.Close()
			})
		})
	})
})
