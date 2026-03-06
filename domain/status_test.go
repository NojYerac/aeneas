package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nojyerac/aeneas/domain"
)

var _ = Describe("Status State Machine", func() {

	Describe("Workflow Status Transitions", func() {
		Context("Valid transitions", func() {
			It("allows draft -> active", func() {
				err := domain.TransitionWorkflow(domain.WorkflowDraft, domain.WorkflowActive)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows active -> archived", func() {
				err := domain.TransitionWorkflow(domain.WorkflowActive, domain.WorkflowArchived)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Invalid transitions", func() {
			It("rejects draft -> archived", func() {
				err := domain.TransitionWorkflow(domain.WorkflowDraft, domain.WorkflowArchived)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid workflow transition"))
			})

			It("rejects archived -> active", func() {
				err := domain.TransitionWorkflow(domain.WorkflowArchived, domain.WorkflowActive)
				Expect(err).To(HaveOccurred())
			})

			It("rejects archived -> draft", func() {
				err := domain.TransitionWorkflow(domain.WorkflowArchived, domain.WorkflowDraft)
				Expect(err).To(HaveOccurred())
			})

			It("rejects active -> draft", func() {
				err := domain.TransitionWorkflow(domain.WorkflowActive, domain.WorkflowDraft)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Execution Status Transitions", func() {
		Context("Valid transitions", func() {
			It("allows pending -> running", func() {
				err := domain.TransitionExecution(domain.ExecutionPending, domain.ExecutionRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows pending -> canceled", func() {
				err := domain.TransitionExecution(domain.ExecutionPending, domain.ExecutionCancelled)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows running -> succeeded", func() {
				err := domain.TransitionExecution(domain.ExecutionRunning, domain.ExecutionSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows running -> failed", func() {
				err := domain.TransitionExecution(domain.ExecutionRunning, domain.ExecutionFailed)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows running -> canceled", func() {
				err := domain.TransitionExecution(domain.ExecutionRunning, domain.ExecutionCancelled)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Invalid transitions", func() {
			It("rejects pending -> succeeded", func() {
				err := domain.TransitionExecution(domain.ExecutionPending, domain.ExecutionSucceeded)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid execution transition"))
			})

			It("rejects pending -> failed", func() {
				err := domain.TransitionExecution(domain.ExecutionPending, domain.ExecutionFailed)
				Expect(err).To(HaveOccurred())
			})

			It("rejects succeeded -> running", func() {
				err := domain.TransitionExecution(domain.ExecutionSucceeded, domain.ExecutionRunning)
				Expect(err).To(HaveOccurred())
			})

			It("rejects failed -> running", func() {
				err := domain.TransitionExecution(domain.ExecutionFailed, domain.ExecutionRunning)
				Expect(err).To(HaveOccurred())
			})

			It("rejects canceled -> running", func() {
				err := domain.TransitionExecution(domain.ExecutionCancelled, domain.ExecutionRunning)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Step Execution Status Transitions", func() {
		Context("Valid transitions", func() {
			It("allows pending -> running", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionPending, domain.StepExecutionRunning)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows pending -> skipped", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionPending, domain.StepExecutionSkipped)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows running -> succeeded", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionRunning, domain.StepExecutionSucceeded)
				Expect(err).ToNot(HaveOccurred())
			})

			It("allows running -> failed", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionRunning, domain.StepExecutionFailed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Invalid transitions", func() {
			It("rejects pending -> succeeded", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionPending, domain.StepExecutionSucceeded)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid step execution transition"))
			})

			It("rejects pending -> failed", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionPending, domain.StepExecutionFailed)
				Expect(err).To(HaveOccurred())
			})

			It("rejects succeeded -> running", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionSucceeded, domain.StepExecutionRunning)
				Expect(err).To(HaveOccurred())
			})

			It("rejects failed -> running", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionFailed, domain.StepExecutionRunning)
				Expect(err).To(HaveOccurred())
			})

			It("rejects skipped -> running", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionSkipped, domain.StepExecutionRunning)
				Expect(err).To(HaveOccurred())
			})

			It("rejects running -> skipped", func() {
				err := domain.TransitionStepExecution(domain.StepExecutionRunning, domain.StepExecutionSkipped)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
