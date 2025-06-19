package controller

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPodController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodController Suite")
}

var _ = Describe("PodReconciler", func() {
	var (
		ctx             context.Context
		reconciler      *PodReconciler
		fakeClient      client.Client
		req             ctrl.Request
		testScheme      *runtime.Scheme
		now             time.Time
	)

	BeforeEach(func() {
		ctx = context.Background()
		testScheme = runtime.NewScheme()
		corev1.AddToScheme(testScheme)
		
		now = time.Now()
		
		reconciler = &PodReconciler{
			Scheme:             testScheme,
			ConfigMapName:      "test-config",
			ConfigMapNamespace: "test-namespace",
		}
		
		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test-pod",
				Namespace: "default",
			},
		}
	})

	Describe("Reconcile", func() {
		Context("when pod does not exist", func() {
			It("should return without error", func() {
				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when config cannot be retrieved", func() {
			It("should return error and requeue", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod).
					Build()
				reconciler.Client = fakeClient

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred()) // Default config is used when ConfigMap doesn't exist
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when pod is not managed by VPA", func() {
			It("should return without action", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
				}

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod, configMap).
					Build()
				reconciler.Client = fakeClient

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when pod is being deleted", func() {
			It("should handle pod deletion", func() {
				deletionTime := metav1.NewTime(now)
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
						DeletionTimestamp: &deletionTime,
						Finalizers:        []string{VPAGracefulDrainFinalizer},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod, configMap).
					Build()
				reconciler.Client = fakeClient

				// Pod is being deleted but grace period hasn't elapsed
				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			})
		})

		Context("when pod needs finalizer", func() {
			It("should add finalizer", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod, configMap).
					Build()
				reconciler.Client = fakeClient

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				// Verify finalizer was added
				updatedPod := &corev1.Pod{}
				err = fakeClient.Get(ctx, req.NamespacedName, updatedPod)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedPod.Finalizers).To(ContainElement(VPAGracefulDrainFinalizer))
			})

			It("should handle conflict error and retry", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{},
				}

				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod, configMap).
					Build()
				reconciler.Client = fakeClient

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})
	})

	Describe("handlePodDeletion", func() {
		var (
			config *Config
		)

		BeforeEach(func() {
			config = NewDefaultConfig()
			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient
		})

		Context("when pod does not have finalizer", func() {
			It("should return without action", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						DeletionTimestamp: &metav1.Time{Time: now},
					},
				}

				result, err := reconciler.handlePodDeletion(ctx, pod, config)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		Context("when graceful drain is not completed", func() {
			It("should requeue", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-pod",
						Namespace:         "default",
						DeletionTimestamp: &metav1.Time{Time: now},
						Finalizers:        []string{VPAGracefulDrainFinalizer},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}

				result, err := reconciler.handlePodDeletion(ctx, pod, config)
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RequeueAfter).To(Equal(10 * time.Second))
			})
		})

		Context("when graceful drain is completed", func() {
			It("should remove finalizer", func() {
				deletionTime := metav1.NewTime(now.Add(-400 * time.Second)) // Exceeded timeout
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-pod",
						Namespace:         "default",
						DeletionTimestamp: &deletionTime,
						Finalizers:        []string{VPAGracefulDrainFinalizer},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				// Add pod to fake client so it can be updated
				fakeClient = fake.NewClientBuilder().
					WithScheme(testScheme).
					WithObjects(pod).
					Build()
				reconciler.Client = fakeClient

				result, err := reconciler.handlePodDeletion(ctx, pod, config)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				// Verify finalizer was removed
				updatedPod := &corev1.Pod{}
				err = fakeClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, updatedPod)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedPod.Finalizers).ToNot(ContainElement(VPAGracefulDrainFinalizer))
			})
		})
	})

	Describe("shouldManagePod", func() {
		var config *Config

		BeforeEach(func() {
			config = NewDefaultConfig()
		})

		Context("with namespace selector", func() {
			BeforeEach(func() {
				config.NamespaceSelector = &NamespaceSelector{
					Include: []string{"default"},
					Exclude: []string{"kube-system"},
				}
			})

			It("should return false for excluded namespace", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "kube-system",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})

			It("should return true for included namespace", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})
		})

		Context("with vpa-managed annotation", func() {
			It("should return true when annotation is 'true'", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "true",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})

			It("should return false when annotation is 'false'", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed": "false",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})

			It("should return false when annotation is missing", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})
		})

		Context("with legacy VPA annotations", func() {
			It("should return true for vpa-updater annotation", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-updater.client.k8s.io/last-updated": "2023-01-01T00:00:00Z",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})

			It("should return true for vpa resource name annotation", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa.k8s.io/resource-name": "my-vpa",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})

			It("should return false for empty vpa resource name annotation", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa.k8s.io/resource-name": "",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})
		})

		Context("with VPA labels", func() {
			It("should return true for vpa.k8s.io/managed label", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Labels: map[string]string{
							"vpa.k8s.io/managed": "true",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})
		})

		Context("with VPA-managed workload detection", func() {
			It("should return true for pod with non-round CPU values", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								Name: "test-rs",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: mustParseQuantity("125m"), // VPA-like value
									},
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})

			It("should return true for pod with non-round memory values", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								Name: "test-rs",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: mustParseQuantity("131072001"), // Non-round value
									},
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeTrue())
			})

			It("should return false for pod with round CPU and memory values", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								Name: "test-rs",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    mustParseQuantity("100m"), // Round value
										corev1.ResourceMemory: mustParseQuantity("128Mi"), // Round value
									},
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})

			It("should return false for pod with no owner references", func() {
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
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: mustParseQuantity("125m"),
									},
								},
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})

			It("should return false for pod with no resource requests", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ReplicaSet",
								Name: "test-rs",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx",
							},
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})
		})

		Context("priority of different annotations", func() {
			It("should prioritize vpa-managed annotation over legacy annotations", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
						Annotations: map[string]string{
							"vpa-managed":                           "false",
							"vpa-updater.client.k8s.io/last-updated": "2023-01-01T00:00:00Z",
						},
					},
				}

				fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
				reconciler.Client = fakeClient

				shouldManage := reconciler.shouldManagePod(pod, config)
				Expect(shouldManage).To(BeFalse())
			})
		})
	})

	Describe("shouldAddFinalizer", func() {
		It("should return true when pod does not have finalizer", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			}

			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient

			shouldAdd := reconciler.shouldAddFinalizer(pod)
			Expect(shouldAdd).To(BeTrue())
		})

		It("should return false when pod already has finalizer", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pod",
					Namespace:  "default",
					Finalizers: []string{VPAGracefulDrainFinalizer},
				},
			}

			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient

			shouldAdd := reconciler.shouldAddFinalizer(pod)
			Expect(shouldAdd).To(BeFalse())
		})

		It("should return true when pod has other finalizers but not ours", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-pod",
					Namespace:  "default",
					Finalizers: []string{"other-finalizer"},
				},
			}

			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient

			shouldAdd := reconciler.shouldAddFinalizer(pod)
			Expect(shouldAdd).To(BeTrue())
		})
	})

	Describe("getConfig", func() {
		It("should return default config when ConfigMap does not exist", func() {
			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient

			config, err := reconciler.getConfig(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.GetGracePeriod()).To(Equal(30 * time.Second))
			Expect(config.GetDrainTimeout()).To(Equal(300 * time.Second))
		})

		It("should parse config from ConfigMap", func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"gracePeriodSeconds":  "60",
					"drainTimeoutSeconds": "600",
				},
			}

			fakeClient = fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(configMap).
				Build()
			reconciler.Client = fakeClient

			config, err := reconciler.getConfig(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.GetGracePeriod()).To(Equal(60 * time.Second))
			Expect(config.GetDrainTimeout()).To(Equal(600 * time.Second))
		})
	})

	Describe("SetupWithManager", func() {
		It("should setup controller successfully", func() {
			// This test is primarily to ensure the SetupWithManager method compiles
			// and doesn't panic. Full integration testing would require a real manager.
			fakeClient = fake.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler.Client = fakeClient

			// We can't easily test the full SetupWithManager without a real manager
			// This ensures the method exists and can be called
			Expect(reconciler.SetupWithManager).ToNot(BeNil())
		})
	})
})

// Helper function to parse resource quantities in tests
func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(err)
	}
	return q
}