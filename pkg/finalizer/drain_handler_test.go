package finalizer

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestDrainHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DrainHandler Suite")
}

type mockConfig struct {
	gracePeriod  time.Duration
	drainTimeout time.Duration
}

func (c *mockConfig) GetGracePeriod() time.Duration {
	return c.gracePeriod
}

func (c *mockConfig) GetDrainTimeout() time.Duration {
	return c.drainTimeout
}

var _ = Describe("DrainHandler", func() {
	var (
		ctx            context.Context
		drainHandler   *DrainHandler
		fakeClient     client.Client
		scheme         *runtime.Scheme
		config         *mockConfig
		now            time.Time
		logger         = zap.New(zap.UseDevMode(true))
	)

	BeforeEach(func() {
		ctx = context.Background()
		// Set up logger in context
		ctx = context.WithValue(ctx, "logger", logger)
		
		scheme = runtime.NewScheme()
		corev1.AddToScheme(scheme)
		
		config = &mockConfig{
			gracePeriod:  30 * time.Second,
			drainTimeout: 300 * time.Second,
		}
		
		now = time.Now()
	})

	Describe("NewDrainHandler", func() {
		It("should create a new DrainHandler instance", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
			
			Expect(drainHandler).ToNot(BeNil())
			Expect(drainHandler.client).To(Equal(fakeClient))
			Expect(drainHandler.config).To(Equal(config))
		})
	})

	Describe("HandleGracefulDrain", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
		})

		Context("when pod has no deletion timestamp", func() {
			It("should return true and skip drain", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(completed).To(BeTrue())
			})
		})

		Context("when pod has deletion timestamp", func() {
			Context("and grace period has not elapsed", func() {
				It("should return false and continue waiting", func() {
					deletionTime := metav1.NewTime(now.Add(-10 * time.Second)) // 10 seconds ago
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeFalse())
				})
			})

			Context("and drain timeout has been exceeded", func() {
				It("should return true and allow deletion", func() {
					deletionTime := metav1.NewTime(now.Add(-400 * time.Second)) // 400 seconds ago (> 300s timeout)
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeTrue())
				})
			})

			Context("and pod has completed successfully", func() {
				It("should return true for Succeeded phase", func() {
					deletionTime := metav1.NewTime(now.Add(-60 * time.Second))
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodSucceeded,
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeTrue())
				})

				It("should return true for Failed phase", func() {
					deletionTime := metav1.NewTime(now.Add(-60 * time.Second))
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodFailed,
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeTrue())
				})
			})

			Context("and pod is not ready", func() {
				It("should return true when pod ready condition is false", func() {
					deletionTime := metav1.NewTime(now.Add(-60 * time.Second))
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type:   corev1.PodReady,
									Status: corev1.ConditionFalse,
								},
							},
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeTrue())
				})

				It("should return true when pod has no ready condition", func() {
					deletionTime := metav1.NewTime(now.Add(-60 * time.Second))
					pod := &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:              "test-pod",
							Namespace:         "default",
							DeletionTimestamp: &deletionTime,
						},
						Status: corev1.PodStatus{
							Phase:      corev1.PodRunning,
							Conditions: []corev1.PodCondition{},
						},
					}

					completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
					Expect(err).ToNot(HaveOccurred())
					Expect(completed).To(BeTrue())
				})
			})
		})
	})

	Describe("isPodReady", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
		})

		It("should return true when pod ready condition is true", func() {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			isReady := drainHandler.isPodReady(pod)
			Expect(isReady).To(BeTrue())
		})

		It("should return false when pod ready condition is false", func() {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			}

			isReady := drainHandler.isPodReady(pod)
			Expect(isReady).To(BeFalse())
		})

		It("should return false when pod has no ready condition", func() {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			}

			isReady := drainHandler.isPodReady(pod)
			Expect(isReady).To(BeFalse())
		})

		It("should return false when pod has other conditions but no ready condition", func() {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			isReady := drainHandler.isPodReady(pod)
			Expect(isReady).To(BeFalse())
		})
	})

	Describe("checkActiveConnections", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
		})

		Context("when pod is not running", func() {
			It("should return false for Pending phase", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})

			It("should return false for Succeeded phase", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})

			It("should return false for Failed phase", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})
		})

		Context("when pod is running", func() {
			It("should return false when pod has no containers", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})

			It("should return false when pod has no exposed ports", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
								Ports: []corev1.ContainerPort{},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})

			It("should return false when pod is not ready despite having exposed ports", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
										Protocol:      corev1.ProtocolTCP,
									},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}

				hasConnections, err := drainHandler.checkActiveConnections(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasConnections).To(BeFalse())
			})
		})
	})

	Describe("checkPodEndpoints", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
		})

		Context("when pod has no IP address", func() {
			It("should return false", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "",
					},
				}

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})
		})

		Context("when pod has IP address", func() {
			It("should return false when no services exist", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})

			It("should return false when service exists but pod IP is not in endpoints", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "test-app",
						},
					},
				}

				endpoints := &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{
									IP: "10.0.0.2", // Different IP
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(service, endpoints).
					Build()
				drainHandler = NewDrainHandler(fakeClient, config)

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})

			It("should return true when service exists and pod IP is in endpoints", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "test-app",
						},
					},
				}

				endpoints := &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{
								{
									IP: "10.0.0.1", // Matching IP
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(service, endpoints).
					Build()
				drainHandler = NewDrainHandler(fakeClient, config)

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeTrue())
			})

			It("should return false when service has no selector", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: nil, // No selector
					},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(service).
					Build()
				drainHandler = NewDrainHandler(fakeClient, config)

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})

			It("should return false when pod labels don't match service selector", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "different-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "test-app",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(service).
					Build()
				drainHandler = NewDrainHandler(fakeClient, config)

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})

			It("should continue checking when endpoints don't exist for a service", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test-app",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.0.0.1",
					},
				}

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-service",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Selector: map[string]string{
							"app": "test-app",
						},
					},
				}

				// No endpoints object created
				fakeClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(service).
					Build()
				drainHandler = NewDrainHandler(fakeClient, config)

				hasEndpoints, err := drainHandler.checkPodEndpoints(ctx, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(hasEndpoints).To(BeFalse())
			})
		})
	})

	Describe("Integration scenarios", func() {
		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			drainHandler = NewDrainHandler(fakeClient, config)
		})

		It("should handle complete graceful drain flow", func() {
			deletionTime := metav1.NewTime(now.Add(-60 * time.Second)) // 60 seconds ago
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "default",
					DeletionTimestamp: &deletionTime,
					Labels: map[string]string{
						"app": "test-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			// No service exists, so no active connections
			completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(completed).To(BeTrue())
		})

		It("should wait when pod has active connections", func() {
			deletionTime := metav1.NewTime(now.Add(-60 * time.Second)) // 60 seconds ago
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "default",
					DeletionTimestamp: &deletionTime,
					Labels: map[string]string{
						"app": "test-app",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "test-app",
					},
				},
			}

			endpoints := &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1", // Pod IP is in endpoints
							},
						},
					},
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(service, endpoints).
				Build()
			drainHandler = NewDrainHandler(fakeClient, config)

			// Pod has active connections, should continue waiting
			completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
			Expect(err).ToNot(HaveOccurred())
			Expect(completed).To(BeFalse())
		})
	})
})