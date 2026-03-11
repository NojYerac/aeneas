package local_test

import (
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
	RunSpecs(t, "LocalRunner Suite")
}

var _ = Describe("LocalRunner", Label("integration"), func() {
	var (
		runner *local.LocalRunner
		logger *logrus.Logger
	)

	BeforeEach(func() {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)

		var err error
		runner, err = local.NewLocalRunner(logger)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Execute", func() {
		Context("with a simple successful command", func() {
			It("should execute and return exit code 0", func(ctx SpecContext) {
				step := &domain.StepDefinition{
					Name:           "test-echo",
					Image:          "alpine:latest",
					Command:        []string{"echo"},
					Args:           []string{"hello world"},
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("hello world"))
			}, SpecTimeout(60*time.Second))
		})

		Context("with a failing command", func() {
			It("should return non-zero exit code", func(ctx SpecContext) {
				step := &domain.StepDefinition{
					Name:           "test-fail",
					Image:          "alpine:latest",
					Command:        []string{"sh"},
					Args:           []string{"-c", "exit 42"},
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(42))
			}, SpecTimeout(60*time.Second))
		})

		Context("with environment variables", func() {
			It("should pass environment variables to the container", func(ctx SpecContext) {
				step := &domain.StepDefinition{
					Name:    "test-env",
					Image:   "alpine:latest",
					Command: []string{"sh"},
					Args:    []string{"-c", "echo $TEST_VAR"},
					Env: map[string]string{
						"TEST_VAR": "test_value",
					},
					TimeoutSeconds: 30,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("test_value"))
			}, SpecTimeout(60*time.Second))
		})

		Context("with a timeout", func() {
			It("should timeout long-running commands", func(ctx SpecContext) {
				step := &domain.StepDefinition{
					Name:           "test-timeout",
					Image:          "alpine:latest",
					Command:        []string{"sleep"},
					Args:           []string{"60"},
					TimeoutSeconds: 1,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("timeout"))
				Expect(result).To(BeNil())
			}, SpecTimeout(10*time.Second))
		})

		Context("with image pull", func() {
			It("should pull image if not present", func(ctx SpecContext) {
				// Using a specific version that's unlikely to be cached
				step := &domain.StepDefinition{
					Name:           "test-pull",
					Image:          "alpine:3.19",
					Command:        []string{"echo"},
					Args:           []string{"pulled"},
					TimeoutSeconds: 60,
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
			}, SpecTimeout(120*time.Second))
		})
	})
})
