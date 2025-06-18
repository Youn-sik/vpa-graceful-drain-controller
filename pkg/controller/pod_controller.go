package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/cho/vpa-graceful-drain-controller/pkg/finalizer"
)

const (
	VPAGracefulDrainFinalizer = "vpa-graceful-drain.cho.github.io/finalizer"
)

type PodReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ConfigMapName      string
	ConfigMapNamespace string
}

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Pod not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	config, err := r.getConfig(ctx)
	if err != nil {
		logger.Error(err, "Failed to get configuration")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, err
	}

	if !r.shouldManagePod(&pod, config) {
		logger.V(1).Info("Pod is not managed by VPA graceful drain controller")
		return ctrl.Result{}, nil
	}

	if pod.DeletionTimestamp != nil {
		logger.Info("Pod is being deleted, handling graceful drain", "pod", pod.Name, "namespace", pod.Namespace)
		return r.handlePodDeletion(ctx, &pod, config)
	}

	if r.shouldAddFinalizer(&pod) {
		logger.Info("Adding VPA graceful drain finalizer to pod", "pod", pod.Name, "namespace", pod.Namespace)

		// Create a copy to avoid modifying the cache
		podCopy := pod.DeepCopy()
		controllerutil.AddFinalizer(podCopy, VPAGracefulDrainFinalizer)

		if err := r.Update(ctx, podCopy); err != nil {
			if errors.IsConflict(err) {
				// Conflict error means the resource was modified, retry
				logger.V(1).Info("Conflict updating pod, will retry", "pod", pod.Name)
				return ctrl.Result{RequeueAfter: time.Millisecond * 100}, nil
			}
			logger.Error(err, "Failed to add finalizer to pod")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *PodReconciler) handlePodDeletion(ctx context.Context, pod *corev1.Pod, config *Config) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(pod, VPAGracefulDrainFinalizer) {
		logger.V(1).Info("Pod does not have VPA graceful drain finalizer, skipping")
		return ctrl.Result{}, nil
	}

	drainHandler := finalizer.NewDrainHandler(r.Client, config)

	completed, err := drainHandler.HandleGracefulDrain(ctx, pod)
	if err != nil {
		logger.Error(err, "Failed to handle graceful drain")
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}

	if !completed {
		logger.Info("Graceful drain not yet completed, requeuing", "pod", pod.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	logger.Info("Graceful drain completed, removing finalizer", "pod", pod.Name)

	// Create a copy to avoid modifying the cache
	podCopy := pod.DeepCopy()
	controllerutil.RemoveFinalizer(podCopy, VPAGracefulDrainFinalizer)

	if err := r.Update(ctx, podCopy); err != nil {
		if errors.IsConflict(err) {
			// Conflict error means the resource was modified, retry
			logger.V(1).Info("Conflict removing finalizer, will retry", "pod", pod.Name)
			return ctrl.Result{RequeueAfter: time.Millisecond * 100}, nil
		}
		logger.Error(err, "Failed to remove finalizer from pod")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PodReconciler) shouldManagePod(pod *corev1.Pod, config *Config) bool {
	// Check namespace selector first
	if config.NamespaceSelector != nil && !config.NamespaceSelector.Matches(pod.Namespace) {
		return false
	}

	// Primary check: Look for explicit vpa-managed annotation
	if pod.Annotations != nil {
		if vpaManaged, exists := pod.Annotations["vpa-managed"]; exists {
			return vpaManaged == "true"
		}
	}

	// Fallback: Check for standard VPA annotations for backward compatibility
	if pod.Annotations != nil {
		// VPA updater adds this annotation when it creates a new pod
		if _, hasVPAAnnotation := pod.Annotations["vpa-updater.client.k8s.io/last-updated"]; hasVPAAnnotation {
			return true
		}

		// Alternative: check for VPA resource name annotation
		if vpaName, hasVPAResourceAnnotation := pod.Annotations["vpa.k8s.io/resource-name"]; hasVPAResourceAnnotation && vpaName != "" {
			return true
		}
	}

	// Check for VPA-related labels
	if pod.Labels != nil {
		// VPA might add labels to identify managed pods
		if _, hasVPALabel := pod.Labels["vpa.k8s.io/managed"]; hasVPALabel {
			return true
		}
	}

	// Check if pod's owner is a Deployment/ReplicaSet that might be managed by VPA
	// This is a more heuristic approach - look for specific patterns
	if r.isPodFromVPAManagedWorkload(pod) {
		return true
	}

	return false
}

func (r *PodReconciler) isPodFromVPAManagedWorkload(pod *corev1.Pod) bool {
	// Check if pod has owner references
	if len(pod.OwnerReferences) == 0 {
		return false
	}

	// For now, we'll use a simple heuristic: if the pod has resource requests/limits
	// that look like they might have been set by VPA (non-round numbers), consider it managed
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil || container.Resources.Limits != nil {
			// Check for CPU requests that look VPA-generated (non-round numbers)
			if cpu := container.Resources.Requests.Cpu(); cpu != nil {
				// VPA often sets precise values like "25m" or "152m"
				cpuMillis := cpu.MilliValue()
				if cpuMillis > 0 && cpuMillis%100 != 0 && cpuMillis%50 != 0 {
					return true
				}
			}

			// Check for memory requests that look VPA-generated
			if memory := container.Resources.Requests.Memory(); memory != nil {
				// VPA often sets precise values that aren't round numbers
				memoryBytes := memory.Value()
				// Check if it's not a round number in MB
				if memoryBytes > 0 && (memoryBytes%(1024*1024) != 0) {
					return true
				}
			}
		}
	}

	return false
}

func (r *PodReconciler) shouldAddFinalizer(pod *corev1.Pod) bool {
	return !controllerutil.ContainsFinalizer(pod, VPAGracefulDrainFinalizer)
}

func (r *PodReconciler) getConfig(ctx context.Context) (*Config, error) {
	var configMap corev1.ConfigMap
	namespacedName := types.NamespacedName{
		Name:      r.ConfigMapName,
		Namespace: r.ConfigMapNamespace,
	}

	if err := r.Get(ctx, namespacedName, &configMap); err != nil {
		if errors.IsNotFound(err) {
			return NewDefaultConfig(), nil
		}
		return nil, err
	}

	return ParseConfig(&configMap)
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			predicate.AnnotationChangedPredicate{},
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				// Handle Pod creation events for VPA-managed pods
				pod, ok := object.(*corev1.Pod)
				if !ok {
					return false
				}

				// Check if pod has vpa-managed annotation
				if pod.Annotations != nil {
					if vpaManaged, exists := pod.Annotations["vpa-managed"]; exists && vpaManaged == "true" {
						return true
					}

					// Also check for standard VPA annotations
					if _, hasVPA := pod.Annotations["vpa-updater.client.k8s.io/last-updated"]; hasVPA {
						return true
					}
					if vpaName, hasVPAResource := pod.Annotations["vpa.k8s.io/resource-name"]; hasVPAResource && vpaName != "" {
						return true
					}
				}

				return false
			}),
		)).
		Complete(r)
}
