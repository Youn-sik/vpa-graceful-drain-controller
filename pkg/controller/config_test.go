package controller

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Describe("NewDefaultConfig", func() {
		It("should create config with default values", func() {
			config := NewDefaultConfig()
			
			Expect(config.GetGracePeriod()).To(Equal(30 * time.Second))
			Expect(config.GetDrainTimeout()).To(Equal(300 * time.Second))
			Expect(config.NamespaceSelector).To(BeNil())
		})
	})

	Describe("ParseConfig", func() {
		Context("when ConfigMap is empty", func() {
			It("should return default config", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.GetGracePeriod()).To(Equal(30 * time.Second))
				Expect(config.GetDrainTimeout()).To(Equal(300 * time.Second))
				Expect(config.NamespaceSelector).To(BeNil())
			})
		})

		Context("when ConfigMap has valid values", func() {
			It("should parse gracePeriodSeconds correctly", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds": "60",
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.GetGracePeriod()).To(Equal(60 * time.Second))
				Expect(config.GetDrainTimeout()).To(Equal(300 * time.Second))
			})

			It("should parse drainTimeoutSeconds correctly", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"drainTimeoutSeconds": "600",
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.GetGracePeriod()).To(Equal(30 * time.Second))
				Expect(config.GetDrainTimeout()).To(Equal(600 * time.Second))
			})

			It("should parse both values correctly", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds":  "45",
						"drainTimeoutSeconds": "900",
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.GetGracePeriod()).To(Equal(45 * time.Second))
				Expect(config.GetDrainTimeout()).To(Equal(900 * time.Second))
			})

			It("should parse namespaceSelector correctly", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"include": ["default", "production"],
							"exclude": ["kube-system", "kube-public"]
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector).ToNot(BeNil())
				Expect(config.NamespaceSelector.Matches("default")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("production")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("kube-system")).To(BeFalse())
				Expect(config.NamespaceSelector.Matches("kube-public")).To(BeFalse())
				Expect(config.NamespaceSelector.Matches("other")).To(BeFalse())
			})
		})

		Context("when ConfigMap has invalid values", func() {
			It("should return error for invalid gracePeriodSeconds", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds": "invalid",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("gracePeriodSeconds"))
			})

			It("should return error for invalid drainTimeoutSeconds", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"drainTimeoutSeconds": "invalid",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("drainTimeoutSeconds"))
			})

			It("should return error for invalid namespaceSelector JSON", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": "invalid json",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("namespaceSelector"))
			})

			It("should return error for negative gracePeriodSeconds", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds": "-10",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("gracePeriodSeconds must be non-negative"))
			})

			It("should return error for negative drainTimeoutSeconds", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"drainTimeoutSeconds": "-10",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("drainTimeoutSeconds must be positive"))
			})

			It("should return error for zero drainTimeoutSeconds", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"drainTimeoutSeconds": "0",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("drainTimeoutSeconds must be positive"))
			})

			It("should return error for gracePeriodSeconds exceeding maximum", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds": "3601",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("gracePeriodSeconds must be less than 3600"))
			})

			It("should return error for drainTimeoutSeconds exceeding maximum", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"drainTimeoutSeconds": "7201",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("drainTimeoutSeconds must be less than 7200"))
			})

			It("should return error when drainTimeout is less than gracePeriod", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"gracePeriodSeconds":  "60",
						"drainTimeoutSeconds": "30",
					},
				}

				_, err := ParseConfig(configMap)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("drainTimeoutSeconds (30) must be greater than gracePeriodSeconds (60)"))
			})
		})

		Context("when ConfigMap is nil", func() {
			It("should return error", func() {
				_, err := ParseConfig(nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("configMap cannot be nil"))
			})
		})
	})

	Describe("NamespaceSelector", func() {
		Context("when only include is specified", func() {
			It("should match only included namespaces", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"include": ["default", "production"]
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector.Matches("default")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("production")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("staging")).To(BeFalse())
			})
		})

		Context("when only exclude is specified", func() {
			It("should match all except excluded namespaces", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"exclude": ["kube-system", "kube-public"]
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector.Matches("default")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("production")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("kube-system")).To(BeFalse())
				Expect(config.NamespaceSelector.Matches("kube-public")).To(BeFalse())
			})
		})

		Context("when both include and exclude are specified", func() {
			It("should prioritize include over exclude", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"include": ["default", "production", "kube-system"],
							"exclude": ["kube-system", "kube-public"]
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector.Matches("default")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("production")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("kube-system")).To(BeTrue()) // included overrides excluded
				Expect(config.NamespaceSelector.Matches("kube-public")).To(BeFalse())
				Expect(config.NamespaceSelector.Matches("staging")).To(BeFalse())
			})
		})

		Context("when empty arrays are specified", func() {
			It("should handle empty include array", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"include": []
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector.Matches("default")).To(BeFalse())
				Expect(config.NamespaceSelector.Matches("production")).To(BeFalse())
			})

			It("should handle empty exclude array", func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-config",
						Namespace: "test-namespace",
					},
					Data: map[string]string{
						"namespaceSelector": `{
							"exclude": []
						}`,
					},
				}

				config, err := ParseConfig(configMap)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.NamespaceSelector.Matches("default")).To(BeTrue())
				Expect(config.NamespaceSelector.Matches("kube-system")).To(BeTrue())
			})
		})
	})

	Describe("Config struct methods", func() {
		It("should implement Config interface correctly", func() {
			config := &Config{
				GracePeriodSeconds:  45,
				DrainTimeoutSeconds: 900,
			}

			Expect(config.GetGracePeriod()).To(Equal(45 * time.Second))
			Expect(config.GetDrainTimeout()).To(Equal(900 * time.Second))
		})
	})
})