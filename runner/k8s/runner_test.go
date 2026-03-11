package k8s_test

import (
	"context"
	"time"

	"github.com/nojyerac/aeneas/domain"
	"github.com/nojyerac/aeneas/runner"
	k8srunner "github.com/nojyerac/aeneas/runner/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ = Describe("K8sRunner", func() {
	var (
		logger *logrus.Logger
		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		logger = logrus.New()
		logger.SetLevel(logrus.ErrorLevel)
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Execute", func() {
		Context("with successful job execution", func() {
			It("should create job, wait for completion, and return exit code 0", func() {
				// Create fake clientset
				fakeClient := fake.NewSimpleClientset()

				// Set up watch reactors
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				// Create runner with fake client
				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				// Define step
				step := domain.StepDefinition{
					Name:           "test-step",
					Image:          "alpine:latest",
					Command:        []string{"echo"},
					Args:           []string{"hello"},
					Env:            map[string]string{"TEST_VAR": "test_value"},
					TimeoutSeconds: 60,
				}

				// Execute in goroutine
				type result struct {
					res *runner.Result
					err error
				}
				resultChan := make(chan result)
				go func() {
					res, err := testRunner.Execute(ctx, step)
					resultChan <- result{res: res, err: err}
				}()

				// Wait for job creation
				time.Sleep(100 * time.Millisecond)

				// Get the created job
				jobs, err := fakeClient.BatchV1().Jobs("aeneas").List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(jobs.Items).To(HaveLen(1))

				job := jobs.Items[0]
				Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("alpine:latest"))
				Expect(job.Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"echo"}))
				Expect(job.Spec.Template.Spec.Containers[0].Args).To(Equal([]string{"hello"}))
				Expect(job.Spec.BackoffLimit).NotTo(BeNil())
				Expect(*job.Spec.BackoffLimit).To(Equal(int32(0)))

				// Simulate job completion
				job.Status.Succeeded = 1
				jobWatcher.Modify(&job)

				// Create a completed pod for the job
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "aeneas",
						Labels: map[string]string{
							"job-name": job.Name,
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
						},
					},
				}
				_, err = fakeClient.CoreV1().Pods("aeneas").Create(ctx, pod, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Wait for result
				select {
				case res := <-resultChan:
					Expect(res.err).NotTo(HaveOccurred())
					Expect(res.res).NotTo(BeNil())
					Expect(res.res.ExitCode).To(Equal(0))
				case <-time.After(5 * time.Second):
					Fail("Timeout waiting for execute to complete")
				}
			})
		})

		Context("with failed job execution", func() {
			It("should return non-zero exit code", func() {
				// Create fake clientset
				fakeClient := fake.NewSimpleClientset()

				// Set up watch reactors
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				// Create runner with fake client
				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				// Define step
				step := domain.StepDefinition{
					Name:    "failing-step",
					Image:   "alpine:latest",
					Command: []string{"sh", "-c", "exit 1"},
				}

				// Execute in goroutine
				type result struct {
					res *runner.Result
					err error
				}
				resultChan := make(chan result)
				go func() {
					res, err := testRunner.Execute(ctx, step)
					resultChan <- result{res: res, err: err}
				}()

				// Wait for job creation
				time.Sleep(100 * time.Millisecond)

				// Get the created job
				jobs, err := fakeClient.BatchV1().Jobs("aeneas").List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(jobs.Items).To(HaveLen(1))

				job := jobs.Items[0]

				// Simulate job failure
				job.Status.Failed = 1
				jobWatcher.Modify(&job)

				// Create a failed pod for the job
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-failed",
						Namespace: "aeneas",
						Labels: map[string]string{
							"job-name": job.Name,
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 1,
									},
								},
							},
						},
					},
				}
				_, err = fakeClient.CoreV1().Pods("aeneas").Create(ctx, pod, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Wait for result
				select {
				case res := <-resultChan:
					Expect(res.err).NotTo(HaveOccurred())
					Expect(res.res).NotTo(BeNil())
					Expect(res.res.ExitCode).To(Equal(1))
				case <-time.After(5 * time.Second):
					Fail("Timeout waiting for execute to complete")
				}
			})
		})

		Context("with timeout", func() {
			It("should respect context timeout", func() {
				// Create fake clientset
				fakeClient := fake.NewSimpleClientset()

				// Set up watch reactors
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				// Create runner with fake client
				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				// Define step with short timeout
				step := domain.StepDefinition{
					Name:           "timeout-step",
					Image:          "alpine:latest",
					Command:        []string{"sleep", "1000"},
					TimeoutSeconds: 1,
				}

				// Execute
				result, err := testRunner.Execute(ctx, step)

				// Should timeout
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(context.DeadlineExceeded))
				Expect(result).To(BeNil())
			})
		})

		Context("job name generation", func() {
			It("should generate valid kubernetes job names", func() {
				fakeClient := fake.NewSimpleClientset()
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				// Test with various step names
				testCases := []struct {
					stepName string
				}{
					{"simple-name"},
					{"Name With Spaces"},
					{"name_with_underscores"},
					{"UPPERCASE"},
					{"special!@#$%characters"},
					{"very-long-name-that-exceeds-sixty-three-characters-and-should-be-truncated"},
				}

				for _, tc := range testCases {
					step := domain.StepDefinition{
						Name:  tc.stepName,
						Image: "alpine:latest",
					}

					// Execute in goroutine to avoid blocking
					go func() {
						_, _ = testRunner.Execute(ctx, step)
					}()

					// Wait for job creation
					time.Sleep(100 * time.Millisecond)

					// Check job was created with valid name
					jobs, err := fakeClient.BatchV1().Jobs("aeneas").List(ctx, metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					
					if len(jobs.Items) > 0 {
						lastJob := jobs.Items[len(jobs.Items)-1]
						Expect(len(lastJob.Name)).To(BeNumerically("<=", 63))
						Expect(lastJob.Name).To(MatchRegexp("^[a-z0-9][-a-z0-9]*[a-z0-9]$"))
					}

					// Clean up
					_ = fakeClient.BatchV1().Jobs("aeneas").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})
				}
			})
		})

		Context("job configuration", func() {
			It("should set ActiveDeadlineSeconds when timeout is specified", func() {
				fakeClient := fake.NewSimpleClientset()
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				step := domain.StepDefinition{
					Name:           "timeout-test",
					Image:          "alpine:latest",
					Command:        []string{"echo", "test"},
					TimeoutSeconds: 120,
				}

				go func() {
					_, _ = testRunner.Execute(ctx, step)
				}()

				time.Sleep(100 * time.Millisecond)

				jobs, err := fakeClient.BatchV1().Jobs("aeneas").List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(jobs.Items).To(HaveLen(1))

				job := jobs.Items[0]
				Expect(job.Spec.ActiveDeadlineSeconds).NotTo(BeNil())
				Expect(*job.Spec.ActiveDeadlineSeconds).To(Equal(int64(120)))
			})

			It("should set environment variables", func() {
				fakeClient := fake.NewSimpleClientset()
				jobWatcher := watch.NewFake()
				fakeClient.PrependWatchReactor("jobs", k8stesting.DefaultWatchReactor(jobWatcher, nil))

				testRunner := k8srunner.NewTestableK8sRunner(fakeClient, "aeneas", logger, true)

				step := domain.StepDefinition{
					Name:    "env-test",
					Image:   "alpine:latest",
					Command: []string{"env"},
					Env: map[string]string{
						"VAR1": "value1",
						"VAR2": "value2",
					},
				}

				go func() {
					_, _ = testRunner.Execute(ctx, step)
				}()

				time.Sleep(100 * time.Millisecond)

				jobs, err := fakeClient.BatchV1().Jobs("aeneas").List(ctx, metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(jobs.Items).To(HaveLen(1))

				job := jobs.Items[0]
				envVars := job.Spec.Template.Spec.Containers[0].Env
				
				// Check that env vars are present
				envMap := make(map[string]string)
				for _, ev := range envVars {
					envMap[ev.Name] = ev.Value
				}
				
				Expect(envMap).To(HaveKey("VAR1"))
				Expect(envMap).To(HaveKey("VAR2"))
				Expect(envMap["VAR1"]).To(Equal("value1"))
				Expect(envMap["VAR2"]).To(Equal("value2"))
			})
		})
	})
})
