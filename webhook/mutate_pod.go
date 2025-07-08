package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodMutator implements the mutating admission webhook for pods
type PodMutator struct {
	Client  client.Client
	Log     logr.Logger
	decoder *admission.Decoder
}

//+kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.smart-scheduler.io,admissionReviewVersions=v1

// Handle processes pod admission requests and applies smart scheduling logic
func (pm *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := pm.Log.WithValues("pod", req.Name, "namespace", req.Namespace)
	log.Info("Processing pod admission request")

	pod := &corev1.Pod{}
	err := pm.decoder.Decode(req, pod)
	if err != nil {
		log.Error(err, "Failed to decode pod")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Skip if pod already has smart-scheduler annotations (to avoid infinite loops)
	if pod.Annotations != nil {
		if _, exists := pod.Annotations["smart-scheduler.io/processed"]; exists {
			log.Info("Pod already processed by smart scheduler, skipping")
			return admission.Allowed("")
		}
	}

	// Check if pod has an owner (e.g., Deployment, ReplicaSet)
	if len(pod.OwnerReferences) == 0 {
		log.Info("Pod has no owner references, skipping smart scheduling")
		return admission.Allowed("")
	}

	// Find the parent Deployment by traversing owner references
	deployment, err := pm.findParentDeployment(ctx, pod)
	if err != nil {
		log.Error(err, "Failed to find parent deployment")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if deployment == nil {
		log.Info("No parent deployment found, skipping smart scheduling")
		return admission.Allowed("")
	}

	// Check for smart scheduling annotations on the deployment
	annotations := deployment.Annotations
	if annotations == nil {
		return admission.Allowed("")
	}

	scheduleStrategy, exists := annotations["smart-scheduler.io/schedule-strategy"]
	if !exists {
		return admission.Allowed("")
	}

	log.Info("Found scheduling strategy", "strategy", scheduleStrategy, "deployment", deployment.Name)

	// Parse the placement strategy
	strategy, err := ParsePlacementStrategy(scheduleStrategy)
	if err != nil {
		log.Error(err, "Failed to parse placement strategy", "strategy", scheduleStrategy)
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("invalid placement strategy: %w", err))
	}

	log.Info("Parsed placement strategy", "base", strategy.Base, "rules", len(strategy.Rules))

	// Get current pod counts for this deployment
	currentCounts, err := pm.getCurrentPodCounts(ctx, deployment, strategy)
	if err != nil {
		log.Error(err, "Failed to get current pod counts")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Current pod counts", "counts", currentCounts)

	// Apply the placement strategy to the pod
	originalPod := pod.DeepCopy()
	err = ApplyPlacementStrategy(pod, strategy, currentCounts)
	if err != nil {
		log.Error(err, "Failed to apply placement strategy")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Mark pod as processed
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["smart-scheduler.io/processed"] = "true"
	pod.Annotations["smart-scheduler.io/strategy-applied"] = scheduleStrategy

	// Create the patch
	patch, err := createPatch(originalPod, pod)
	if err != nil {
		log.Error(err, "Failed to create patch")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	log.Info("Successfully applied smart scheduling", "nodeSelector", pod.Spec.NodeSelector)

	return admission.PatchResponseFromRaw(req.Object.Raw, patch)
}

// getCurrentPodCounts gets the current pod distribution for a deployment
func (pm *PodMutator) getCurrentPodCounts(ctx context.Context, deployment *appsv1.Deployment, strategy *PlacementStrategy) (map[string]int, error) {
	counts := make(map[string]int)

	// Initialize counts for all rules
	for _, rule := range strategy.Rules {
		ruleKey := nodeSelector2String(rule.NodeSelector)
		counts[ruleKey] = 0
	}

	// Get all pods for this deployment
	podList := &corev1.PodList{}

	// Create label selector from deployment
	labelSelector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

	err := pm.Client.List(ctx, podList, &client.ListOptions{
		Namespace:     deployment.Namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Count pods by their nodeSelector
	for _, pod := range podList.Items {
		// Skip pods that are being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Skip pods that are not running or pending
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		// Convert pod's nodeSelector to string key
		podKey := nodeSelector2String(pod.Spec.NodeSelector)

		// Find matching rule
		for _, rule := range strategy.Rules {
			ruleKey := nodeSelector2String(rule.NodeSelector)
			if podKey == ruleKey || isNodeSelectorSubset(rule.NodeSelector, pod.Spec.NodeSelector) {
				counts[ruleKey]++
				break
			}
		}
	}

	return counts, nil
}

// isNodeSelectorSubset checks if the rule's nodeSelector is a subset of the pod's nodeSelector
func isNodeSelectorSubset(ruleSelector, podSelector map[string]string) bool {
	if len(ruleSelector) == 0 {
		return true
	}

	if len(podSelector) == 0 {
		return false
	}

	for key, value := range ruleSelector {
		if podValue, exists := podSelector[key]; !exists || podValue != value {
			return false
		}
	}

	return true
}

// createPatch creates a JSON patch from original pod to modified pod
func createPatch(original, modified *corev1.Pod) ([]byte, error) {
	// For simplicity, we'll create a manual patch
	// In a production environment, you might want to use a proper JSON patch library
	patch := []map[string]interface{}{}

	// Add nodeSelector patch if it was modified
	if !nodeSelectorsEqual(original.Spec.NodeSelector, modified.Spec.NodeSelector) {
		patch = append(patch, map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/nodeSelector",
			"value": modified.Spec.NodeSelector,
		})
	}

	// Add annotations patch
	if !annotationsEqual(original.Annotations, modified.Annotations) {
		patch = append(patch, map[string]interface{}{
			"op":    "replace",
			"path":  "/metadata/annotations",
			"value": modified.Annotations,
		})
	}

	return json.Marshal(patch)
}

// nodeSelectorsEqual compares two nodeSelector maps
func nodeSelectorsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, value := range a {
		if bValue, exists := b[key]; !exists || bValue != value {
			return false
		}
	}

	return true
}

// annotationsEqual compares two annotation maps
func annotationsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, value := range a {
		if bValue, exists := b[key]; !exists || bValue != value {
			return false
		}
	}

	return true
}

// findParentDeployment finds the parent Deployment of a pod by traversing owner references
func (pm *PodMutator) findParentDeployment(ctx context.Context, pod *corev1.Pod) (*appsv1.Deployment, error) {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			// Get the ReplicaSet
			rs := &appsv1.ReplicaSet{}
			err := pm.Client.Get(ctx, client.ObjectKey{
				Namespace: pod.Namespace,
				Name:      ownerRef.Name,
			}, rs)
			if err != nil {
				continue
			}

			// Check if ReplicaSet has a Deployment owner
			for _, rsOwnerRef := range rs.OwnerReferences {
				if rsOwnerRef.Kind == "Deployment" {
					deployment := &appsv1.Deployment{}
					err := pm.Client.Get(ctx, client.ObjectKey{
						Namespace: pod.Namespace,
						Name:      rsOwnerRef.Name,
					}, deployment)
					if err == nil {
						return deployment, nil
					}
				}
			}
		}
	}
	return nil, nil
}

// SetupWebhookWithManager sets up the webhook with the manager
func (pm *PodMutator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	pm.decoder = admission.NewDecoder(mgr.GetScheme())

	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithValidator(pm).
		WithDefaulter(pm).
		Complete()
}

// InjectDecoder injects the decoder into the webhook
func (pm *PodMutator) InjectDecoder(d *admission.Decoder) error {
	pm.decoder = d
	return nil
}

// Default implements the defaulting webhook interface (required by controller-runtime)
func (pm *PodMutator) Default(ctx context.Context, obj runtime.Object) error {
	// This method is required by the defaulting webhook interface
	// The actual logic is in the Handle method
	return nil
}

// ValidateCreate implements the validating webhook interface
func (pm *PodMutator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements the validating webhook interface
func (pm *PodMutator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements the validating webhook interface
func (pm *PodMutator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
