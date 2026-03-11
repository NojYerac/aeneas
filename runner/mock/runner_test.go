package mock_test

import (
	"context"
	"errors"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
		Context("with default configuration", func() {
			It("should return successful result", func() {
				step := domain.StepDefinition{Name: "test-step"}

				result, err := runner.Execute(ctx, step)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.ExitCode).To(Equal(0))
				Expect(result.Logs).To(Equal("mock execution successful"))
			})

			It("should track executed steps", func() {
				step1 := domain.StepDefinition{Name: "step1"}
				step2 := domain.StepDefinition{Name: "step2"}

				_, _ = runner.Execute(ctx, step1)
				_, _ = runner.Execute(ctx, step2)

				Expect(runner.GetExecutedCount()).To(Equal(2))
				Expect(runner.AssertExecuted("step1")).ToNot(HaveOccurred())
				Expect(runner.AssertExecuted("step2")).ToNot(HaveOccurred())
			})
		})

		Context("with configured responses", func() {
			It("should return configured result for specific step", func() {
				runner.WithResponse("custom-step", 42, "custom logs")

				step := domain.StepDefinition{Name: "custom-step"}
				result, err := runner.Execute(ctx, step)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.ExitCode).To(Equal(42))
				Expect(result.Logs).To(Equal("custom logs"))
			})

			It("should return default for unconfigured steps", func() {
				runner.WithResponse("custom-step", 42, "custom logs")

				step := domain.StepDefinition{Name: "other-step"}
				result, err := runner.Execute(ctx, step)

				Expect(err).ToNot(HaveOccurred())
				Expect(result.ExitCode).To(Equal(0))
			})
		})

		Context("with configured errors", func() {
			It("should return configured error for specific step", func() {
				expectedErr := errors.New("custom error")
				runner.WithError("failing-step", expectedErr)

				step := domain.StepDefinition{Name: "failing-step"}
				result, err := runner.Execute(ctx, step)

				Expect(err).To(Equal(expectedErr))
				Expect(result).To(BeNil())
			})

			It("should return default error when set", func() {
				expectedErr := errors.New("default error")
				runner.WithDefaultError(expectedErr)

				step := domain.StepDefinition{Name: "any-step"}
				result, err := runner.Execute(ctx, step)

				Expect(err).To(Equal(expectedErr))
				Expect(result).To(BeNil())
			})

			It("should prioritize step-specific error over default", func() {
				defaultErr := errors.New("default error")
				specificErr := errors.New("specific error")

				runner.WithDefaultError(defaultErr)
				runner.WithError("specific-step", specificErr)

				step1 := domain.StepDefinition{Name: "specific-step"}
				_, err1 := runner.Execute(ctx, step1)
				Expect(err1).To(Equal(specificErr))

				step2 := domain.StepDefinition{Name: "other-step"}
				_, err2 := runner.Execute(ctx, step2)
				Expect(err2).To(Equal(defaultErr))
			})
		})

		Context("fluent interface", func() {
			It("should support method chaining", func() {
				runner.
					WithResponse("step1", 0, "success").
					WithResponse("step2", 1, "failure").
					WithError("step3", errors.New("error"))

				Expect(runner.Responses["step1"].ExitCode).To(Equal(0))
				Expect(runner.Responses["step2"].ExitCode).To(Equal(1))
				Expect(runner.Errors["step3"]).To(HaveOccurred())
			})
		})
	})

	Describe("Reset", func() {
		It("should clear all state", func() {
			runner.WithResponse("step1", 42, "test")
			runner.WithError("step2", errors.New("error"))

			step := domain.StepDefinition{Name: "step1"}
			_, _ = runner.Execute(ctx, step)

			runner.Reset()

			Expect(runner.GetExecutedCount()).To(Equal(0))
			Expect(runner.Responses).To(BeEmpty())
			Expect(runner.Errors).To(BeEmpty())
			Expect(runner.DefaultResult.ExitCode).To(Equal(0))
			Expect(runner.DefaultError).ToNot(HaveOccurred())
		})
	})

	Describe("AssertExecuted", func() {
		It("should return error for unexecuted step", func() {
			err := runner.AssertExecuted("nonexistent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("was not executed"))
		})

		It("should return nil for executed step", func() {
			step := domain.StepDefinition{Name: "executed-step"}
			_, _ = runner.Execute(ctx, step)

			err := runner.AssertExecuted("executed-step")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
