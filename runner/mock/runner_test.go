package mock_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner/mock"
)

func TestMockRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MockRunner Suite")
}

var _ = Describe("MockRunner", func() {
	var (
		runner *mock.MockRunner
		ctx    context.Context
	)

	BeforeEach(func() {
		runner = mock.NewMockRunner()
		ctx = context.Background()
	})

	Describe("Execute", func() {
		Context("with default behavior", func() {
			It("should return success with default logs", func() {
				step := &domain.StepDefinition{
					Name:  "test-step",
					Image: "alpine:latest",
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(ContainSubstring("Mock execution of step: test-step"))
			})

			It("should track executed steps", func() {
				step1 := &domain.StepDefinition{Name: "step-1", Image: "alpine:latest"}
				step2 := &domain.StepDefinition{Name: "step-2", Image: "ubuntu:latest"}

				_, _ = runner.Execute(ctx, step1)
				_, _ = runner.Execute(ctx, step2)

				Expect(runner.GetExecutionCount()).To(Equal(2))
				Expect(runner.ExecutedSteps).To(HaveLen(2))
				Expect(runner.ExecutedSteps[0].Name).To(Equal("step-1"))
				Expect(runner.ExecutedSteps[1].Name).To(Equal("step-2"))
			})
		})

		Context("with configured responses", func() {
			It("should return the configured result for a step", func() {
				runner.WithResponse("custom-step", 42, "custom logs")

				step := &domain.StepDefinition{
					Name:  "custom-step",
					Image: "alpine:latest",
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())
				Expect(result.ExitCode).To(Equal(42))
				Expect(result.Logs).To(Equal("custom logs"))
			})

			It("should support chained configuration", func() {
				runner.
					WithResponse("step-1", 0, "logs-1").
					WithResponse("step-2", 1, "logs-2").
					WithError("step-3", errors.New("mock error"))

				step1 := &domain.StepDefinition{Name: "step-1", Image: "alpine:latest"}
				step2 := &domain.StepDefinition{Name: "step-2", Image: "alpine:latest"}
				step3 := &domain.StepDefinition{Name: "step-3", Image: "alpine:latest"}

				result1, err1 := runner.Execute(ctx, step1)
				Expect(err1).NotTo(HaveOccurred())
				Expect(result1.ExitCode).To(Equal(0))
				Expect(result1.Logs).To(Equal("logs-1"))

				result2, err2 := runner.Execute(ctx, step2)
				Expect(err2).NotTo(HaveOccurred())
				Expect(result2.ExitCode).To(Equal(1))
				Expect(result2.Logs).To(Equal("logs-2"))

				result3, err3 := runner.Execute(ctx, step3)
				Expect(err3).To(MatchError("mock error"))
				Expect(result3).To(BeNil())
			})
		})

		Context("with configured errors", func() {
			It("should return the configured error", func() {
				expectedErr := errors.New("test error")
				runner.WithError("failing-step", expectedErr)

				step := &domain.StepDefinition{
					Name:  "failing-step",
					Image: "alpine:latest",
				}

				result, err := runner.Execute(ctx, step)
				Expect(err).To(Equal(expectedErr))
				Expect(result).To(BeNil())
			})

			It("should prioritize errors over responses", func() {
				runner.
					WithResponse("step-1", 0, "success logs").
					WithError("step-1", errors.New("error takes precedence"))

				step := &domain.StepDefinition{Name: "step-1", Image: "alpine:latest"}

				result, err := runner.Execute(ctx, step)
				Expect(err).To(MatchError("error takes precedence"))
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("GetLastExecutedStep", func() {
		Context("when no steps have been executed", func() {
			It("should return nil", func() {
				Expect(runner.GetLastExecutedStep()).To(BeNil())
			})
		})

		Context("when steps have been executed", func() {
			It("should return the most recent step", func() {
				step1 := &domain.StepDefinition{Name: "step-1", Image: "alpine:latest"}
				step2 := &domain.StepDefinition{Name: "step-2", Image: "ubuntu:latest"}

				_, _ = runner.Execute(ctx, step1)
				_, _ = runner.Execute(ctx, step2)

				lastStep := runner.GetLastExecutedStep()
				Expect(lastStep).NotTo(BeNil())
				Expect(lastStep.Name).To(Equal("step-2"))
			})
		})
	})

	Describe("Reset", func() {
		It("should clear all execution history and configurations", func() {
			runner.
				WithResponse("step-1", 0, "logs").
				WithError("step-2", errors.New("error"))

			step := &domain.StepDefinition{Name: "step-1", Image: "alpine:latest"}
			_, _ = runner.Execute(ctx, step)

			Expect(runner.GetExecutionCount()).To(Equal(1))

			runner.Reset()

			Expect(runner.GetExecutionCount()).To(Equal(0))
			Expect(runner.GetLastExecutedStep()).To(BeNil())

			// Should now use default behavior
			result, err := runner.Execute(ctx, step)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.ExitCode).To(Equal(0))
			Expect(result.Logs).To(ContainSubstring("Mock execution"))
		})
	})
})
