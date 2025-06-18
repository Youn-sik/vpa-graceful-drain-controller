package finalizer

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Config interface {
	GetGracePeriod() time.Duration
	GetDrainTimeout() time.Duration
}

type DrainHandler struct {
	client client.Client
	config Config
}

func NewDrainHandler(client client.Client, config Config) *DrainHandler {
	return &DrainHandler{
		client: client,
		config: config,
	}
}

func (d *DrainHandler) HandleGracefulDrain(ctx context.Context, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	if pod.DeletionTimestamp == nil {
		logger.V(1).Info("Pod has no deletion timestamp, skipping drain")
		return true, nil
	}

	gracePeriod := d.config.GetGracePeriod()
	drainTimeout := d.config.GetDrainTimeout()

	timeSinceDeletion := time.Since(pod.DeletionTimestamp.Time)

	if timeSinceDeletion < gracePeriod {
		logger.Info("Graceful drain period not yet elapsed",
			"elapsed", timeSinceDeletion.String(),
			"gracePeriod", gracePeriod.String(),
			"pod", pod.Name)
		return false, nil
	}

	if timeSinceDeletion > drainTimeout {
		logger.Info("Drain timeout exceeded, allowing pod deletion",
			"elapsed", timeSinceDeletion.String(),
			"drainTimeout", drainTimeout.String(),
			"pod", pod.Name)
		return true, nil
	}

	// If pod has completed successfully or failed, drain is complete
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		logger.Info("Pod has completed, graceful drain completed",
			"pod", pod.Name,
			"phase", pod.Status.Phase)
		return true, nil
	}

	isReady := d.isPodReady(pod)
	if !isReady {
		logger.Info("Pod is not ready, graceful drain completed", "pod", pod.Name)
		return true, nil
	}

	hasActiveConnections, err := d.checkActiveConnections(ctx, pod)
	if err != nil {
		logger.Error(err, "Failed to check active connections")
		return false, err
	}

	if !hasActiveConnections {
		logger.Info("No active connections detected, graceful drain completed", "pod", pod.Name)
		return true, nil
	}

	logger.Info("Pod still has active connections, continuing drain", "pod", pod.Name)
	return false, nil
}

func (d *DrainHandler) isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (d *DrainHandler) checkActiveConnections(ctx context.Context, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	// If pod is not running (succeeded, failed, pending), no active connections
	if pod.Status.Phase != corev1.PodRunning {
		logger.V(1).Info("Pod is not running, no active connections", "pod", pod.Name, "phase", pod.Status.Phase)
		return false, nil
	}

	if len(pod.Spec.Containers) == 0 {
		return false, nil
	}

	// Check if pod has any exposed ports that might have active connections
	hasExposedPorts := false
	for _, container := range pod.Spec.Containers {
		if len(container.Ports) > 0 {
			hasExposedPorts = true
			break
		}
	}

	if !hasExposedPorts {
		logger.V(1).Info("Pod has no exposed ports, assuming no active connections", "pod", pod.Name)
		return false, nil
	}

	// Check readiness probe status - if readiness probe is failing,
	// it's likely the pod is not serving traffic
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			logger.V(1).Info("Pod is not ready, assuming no active connections", "pod", pod.Name)
			return false, nil
		}
	}

	// Check if pod has any endpoints in service
	hasActiveEndpoints, err := d.checkPodEndpoints(ctx, pod)
	if err != nil {
		logger.Error(err, "Failed to check pod endpoints")
		// If we can't determine endpoint status, assume there might be connections
		return true, err
	}

	if !hasActiveEndpoints {
		logger.V(1).Info("Pod has no active endpoints, assuming no active connections", "pod", pod.Name)
		return false, nil
	}

	// If pod is ready and has active endpoints, assume it might have active connections
	// In a production environment, you might want to implement more sophisticated
	// connection checking (e.g., via metrics, custom health endpoints, etc.)
	logger.V(1).Info("Pod appears to be actively serving traffic", "pod", pod.Name)
	return true, nil
}

// checkPodEndpoints checks if the pod is part of any service endpoints
func (d *DrainHandler) checkPodEndpoints(ctx context.Context, pod *corev1.Pod) (bool, error) {
	logger := log.FromContext(ctx)

	// List all services in the pod's namespace
	var serviceList corev1.ServiceList
	if err := d.client.List(ctx, &serviceList, client.InNamespace(pod.Namespace)); err != nil {
		return false, err
	}

	podIP := pod.Status.PodIP
	if podIP == "" {
		logger.V(1).Info("Pod has no IP address", "pod", pod.Name)
		return false, nil
	}

	// Check each service to see if this pod is targeted
	for _, service := range serviceList.Items {
		if service.Spec.Selector == nil {
			continue
		}

		// Check if pod matches service selector
		podLabels := labels.Set(pod.Labels)
		serviceSelector := labels.Set(service.Spec.Selector)

		if serviceSelector.AsSelector().Matches(podLabels) {
			// Get endpoints for this service
			var endpoints corev1.Endpoints
			endpointsName := client.ObjectKey{
				Namespace: service.Namespace,
				Name:      service.Name,
			}

			if err := d.client.Get(ctx, endpointsName, &endpoints); err != nil {
				// If endpoints don't exist, service might not be active
				continue
			}

			// Check if this pod's IP is in the endpoints
			for _, subset := range endpoints.Subsets {
				for _, address := range subset.Addresses {
					if address.IP == podIP {
						logger.V(1).Info("Pod found in service endpoints",
							"pod", pod.Name,
							"service", service.Name,
							"podIP", podIP)
						return true, nil
					}
				}
			}
		}
	}

	logger.V(1).Info("Pod not found in any service endpoints", "pod", pod.Name)
	return false, nil
}
