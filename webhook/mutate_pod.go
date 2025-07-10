package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
	Client       client.Client
	Log          logr.Logger
	decoder      *admission.Decoder
	StateManager *StateManager
}

//+kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.smart-scheduler.io,admissionReviewVersions=v1

// Handle processes pod admission requests and applies smart scheduling logic
func (pm *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	startTime := time.Now()
	log := pm.Log.WithValues("pod", req.Name, "namespace", req.Namespace, "uid", req.UID, "operation", req.Operation)

	// Add detailed request logging for debugging
	log.Info("=== WEBHOOK REQUEST START ===",
		"requestUID", req.UID,
		"kind", req.Kind.Kind,
		"operation", req.Operation,
		"userInfo", req.UserInfo,
		"dryRun", req.DryRun,
		"oldObject", len(req.OldObject.Raw) > 0,
		"subResource", req.SubResource)

	defer func() {
		duration := time.Since(startTime)
		log.Info("=== WEBHOOK REQUEST END ===",
			"duration", duration.String(),
			"durationMs", duration.Milliseconds())
	}()

	pod := &corev1.Pod{}
	err := pm.decoder.Decode(req, pod)
	if err != nil {
		log.Error(err, "Failed to decode pod")
		return admission.Errored(http.StatusBadRequest, err)
	}

	log.Info("Decoded pod details",
		"podName", pod.Name,
		"generateName", pod.GenerateName,
		"ownerReferences", len(pod.OwnerReferences),
		"existingAnnotations", len(pod.Annotations),
		"existingLabels", len(pod.Labels),
		"hasNodeSelector", len(pod.Spec.NodeSelector) > 0,
		"hasAffinity", pod.Spec.Affinity != nil)

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

	// Log owner reference details
	for i, ownerRef := range pod.OwnerReferences {
		log.Info("Owner reference found",
			"index", i,
			"name", ownerRef.Name,
			"kind", ownerRef.Kind,
			"apiVersion", ownerRef.APIVersion,
			"controller", ownerRef.Controller != nil && *ownerRef.Controller,
			"blockOwnerDeletion", ownerRef.BlockOwnerDeletion != nil && *ownerRef.BlockOwnerDeletion)
	}

	// Find the parent Deployment by traversing owner references
	deployment, err := pm.findParentDeployment(ctx, pod)
	if err != nil {
		log.Error(err, "Failed to find parent deployment")
		// Don't fail the request, allow default scheduling
		return pm.allowWithFallback(log, "failed to find parent deployment")
	}

	if deployment == nil {
		log.Info("No parent deployment found, skipping smart scheduling")
		return admission.Allowed("")
	}

	log.Info("Found parent deployment",
		"deploymentName", deployment.Name,
		"deploymentNamespace", deployment.Namespace,
		"deploymentUID", deployment.UID,
		"generation", deployment.Generation,
		"replicas", deployment.Spec.Replicas)

	// Check for smart scheduling annotations on the deployment
	annotations := deployment.Annotations
	if annotations == nil {
		log.Info("Deployment has no annotations, allowing default scheduling")
		return admission.Allowed("")
	}

	log.Info("Deployment annotations found",
		"annotationCount", len(annotations),
		"hasScheduleStrategy", annotations["smart-scheduler.io/schedule-strategy"] != "")

	scheduleStrategy, exists := annotations["smart-scheduler.io/schedule-strategy"]
	if !exists {
		log.Info("No schedule strategy annotation found, allowing default scheduling")
		return admission.Allowed("")
	}

	log.Info("Found scheduling strategy", "strategy", scheduleStrategy, "deployment", deployment.Name)

	// Parse the placement strategy
	strategy, err := ParsePlacementStrategy(scheduleStrategy)
	if err != nil {
		log.Error(err, "Failed to parse placement strategy", "strategy", scheduleStrategy)
		// Don't fail the request, allow default scheduling
		return pm.allowWithFallback(log, fmt.Sprintf("invalid placement strategy: %v", err))
	}

	log.Info("Parsed placement strategy", "base", strategy.Base, "rules", len(strategy.Rules))

	// Get current placement state using StateManager
	placementState, err := pm.StateManager.GetPlacementState(ctx, deployment, strategy)
	if err != nil {
		log.Error(err, "Failed to get placement state")
		// Don't fail the request, try to continue with basic logic
		return pm.applyStrategyWithFallback(ctx, req, pod, deployment, strategy, log)
	}

	log.Info("Current placement state", "totalPods", placementState.TotalPods, "counts", placementState.PodCounts)

	// Apply the placement strategy to the pod
	originalPod := pod.DeepCopy()
	err = ApplyPlacementStrategy(pod, strategy, placementState.PodCounts)
	if err != nil {
		log.Error(err, "Failed to apply placement strategy")
		// Don't fail the request, allow default scheduling
		return pm.allowWithFallback(log, fmt.Sprintf("failed to apply placement strategy: %v", err))
	}

	// Mark pod as processed
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["smart-scheduler.io/processed"] = "true"
	pod.Annotations["smart-scheduler.io/strategy-applied"] = scheduleStrategy
	pod.Annotations["smart-scheduler.io/placement-rule"] = pm.getAppliedRuleKey(originalPod, pod, strategy)

	// Update placement state
	appliedRuleKey := pm.getAppliedRuleKey(originalPod, pod, strategy)
	if appliedRuleKey != "" {
		log.Info("Updating placement state", "appliedRuleKey", appliedRuleKey)
		err = pm.StateManager.IncrementPodCount(ctx, deployment, appliedRuleKey)
		if err != nil {
			log.Error(err, "Failed to update placement state, continuing without state update")
			// Don't fail the request, just log the error
		}
	}

	// Create the patch
	patch, err := createPatch(originalPod, pod)
	if err != nil {
		log.Error(err, "Failed to create patch")
		return pm.allowWithFallback(log, fmt.Sprintf("failed to create patch: %v", err))
	}

	log.Info("Successfully applied smart scheduling",
		"nodeSelector", pod.Spec.NodeSelector,
		"hasAffinity", pod.Spec.Affinity != nil,
		"appliedRule", appliedRuleKey,
		"patchSize", len(patch))

	return admission.PatchResponseFromRaw(req.Object.Raw, patch)
}

// allowWithFallback allows the request with a warning annotation
func (pm *PodMutator) allowWithFallback(log logr.Logger, reason string) admission.Response {
	log.Info("Allowing pod with fallback to default scheduling", "reason", reason)
	return admission.Allowed(fmt.Sprintf("SmartScheduler fallback: %s", reason))
}

// applyStrategyWithFallback applies strategy with basic logic when StateManager fails
func (pm *PodMutator) applyStrategyWithFallback(ctx context.Context, req admission.Request, pod *corev1.Pod, deployment *appsv1.Deployment, strategy *PlacementStrategy, log logr.Logger) admission.Response {
	log.Info("Applying strategy with fallback logic")

	// Try to get basic pod counts without StateManager
	currentCounts, err := pm.getBasicPodCounts(ctx, deployment, strategy)
	if err != nil {
		log.Error(err, "Failed to get basic pod counts")
		return pm.allowWithFallback(log, "failed to get pod counts")
	}

	originalPod := pod.DeepCopy()
	err = ApplyPlacementStrategy(pod, strategy, currentCounts)
	if err != nil {
		log.Error(err, "Failed to apply placement strategy in fallback mode")
		return pm.allowWithFallback(log, "failed to apply strategy in fallback")
	}

	// Mark pod as processed
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["smart-scheduler.io/processed"] = "true"
	pod.Annotations["smart-scheduler.io/fallback-mode"] = "true"

	// Create the patch
	patch, err := createPatch(originalPod, pod)
	if err != nil {
		log.Error(err, "Failed to create patch in fallback mode")
		return pm.allowWithFallback(log, "failed to create patch")
	}

	log.Info("Successfully applied smart scheduling in fallback mode", "nodeSelector", pod.Spec.NodeSelector)
	return admission.PatchResponseFromRaw(req.Object.Raw, patch)
}

// getAppliedRuleKey determines which rule was applied to the pod
func (pm *PodMutator) getAppliedRuleKey(originalPod, modifiedPod *corev1.Pod, strategy *PlacementStrategy) string {
	// Compare nodeSelectors to determine which rule was applied
	appliedNodeSelector := make(map[string]string)

	// Find newly added nodeSelector entries
	if modifiedPod.Spec.NodeSelector != nil {
		for key, value := range modifiedPod.Spec.NodeSelector {
			if originalPod.Spec.NodeSelector == nil || originalPod.Spec.NodeSelector[key] != value {
				appliedNodeSelector[key] = value
			}
		}
	}

	// Match against strategy rules
	for _, rule := range strategy.Rules {
		if isNodeSelectorSubset(rule.NodeSelector, appliedNodeSelector) {
			return ruleToString(rule)
		}
	}

	// Fallback to full nodeSelector
	return nodeSelector2String(appliedNodeSelector)
}

// getBasicPodCounts gets pod counts without using StateManager
func (pm *PodMutator) getBasicPodCounts(ctx context.Context, deployment *appsv1.Deployment, strategy *PlacementStrategy) (map[string]int, error) {
	counts := make(map[string]int)

	// Initialize counts for all rules
	for _, rule := range strategy.Rules {
		ruleKey := ruleToString(rule)
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
			ruleKey := ruleToString(rule)
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
		if modified.Spec.NodeSelector == nil {
			patch = append(patch, map[string]interface{}{
				"op":   "remove",
				"path": "/spec/nodeSelector",
			})
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  "/spec/nodeSelector",
				"value": modified.Spec.NodeSelector,
			})
		}
	}

	// Add affinity patch if it was modified
	if !affinityEqual(original.Spec.Affinity, modified.Spec.Affinity) {
		if modified.Spec.Affinity == nil {
			patch = append(patch, map[string]interface{}{
				"op":   "remove",
				"path": "/spec/affinity",
			})
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  "/spec/affinity",
				"value": modified.Spec.Affinity,
			})
		}
	}

	// Add annotations patch
	if !annotationsEqual(original.Annotations, modified.Annotations) {
		if modified.Annotations == nil {
			patch = append(patch, map[string]interface{}{
				"op":   "remove",
				"path": "/metadata/annotations",
			})
		} else {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  "/metadata/annotations",
				"value": modified.Annotations,
			})
		}
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

// affinityEqual compares two affinity objects
func affinityEqual(a, b *corev1.Affinity) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// For simplicity, we'll do a deep comparison via JSON marshaling
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
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

	// Initialize StateManager
	if pm.StateManager == nil {
		pm.StateManager = NewStateManager(mgr.GetClient(), pm.Log.WithName("StateManager"))
	}

	// Register the mutating admission webhook
	mgr.GetWebhookServer().Register("/mutate-v1-pod", &admission.Webhook{
		Handler: pm,
	})

	pm.Log.Info("Webhook registered successfully", "path", "/mutate-v1-pod")
	return nil
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
