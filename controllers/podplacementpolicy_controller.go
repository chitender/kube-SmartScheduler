package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	smartschedulerv1 "github.com/kube-smartscheduler/smart-scheduler/api/v1"
	"github.com/kube-smartscheduler/smart-scheduler/webhook"
)

// PodPlacementPolicyController reconciles a PodPlacementPolicy object
type PodPlacementPolicyController struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	StateManager *webhook.StateManager
}

//+kubebuilder:rbac:groups=smartscheduler.io,resources=podplacementpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=smartscheduler.io,resources=podplacementpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=smartscheduler.io,resources=podplacementpolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile handles PodPlacementPolicy changes and applies them to matching deployments
func (r *PodPlacementPolicyController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("podplacementpolicy", req.NamespacedName)

	// Fetch the PodPlacementPolicy instance
	policy := &smartschedulerv1.PodPlacementPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Policy was deleted, clean up any applied annotations
			return r.handlePolicyDeletion(ctx, req.NamespacedName, log)
		}
		return ctrl.Result{}, err
	}

	log.Info("Processing PodPlacementPolicy", "enabled", policy.Spec.Enabled, "priority", policy.Spec.Priority)

	// Skip disabled policies
	if !policy.Spec.Enabled {
		log.Info("Policy is disabled, skipping")
		return r.updatePolicyStatus(ctx, policy, nil, log)
	}

	// Find matching deployments
	matchedDeployments, err := r.findMatchingDeployments(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to find matching deployments")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, err
	}

	log.Info("Found matching deployments", "count", len(matchedDeployments))

	// Apply policy to each matching deployment
	var deploymentRefs []smartschedulerv1.DeploymentReference
	for _, deployment := range matchedDeployments {
		ref, err := r.applyPolicyToDeployment(ctx, policy, &deployment, log)
		if err != nil {
			log.Error(err, "Failed to apply policy to deployment", "deployment", deployment.Name)
			continue
		}
		if ref != nil {
			deploymentRefs = append(deploymentRefs, *ref)
		}
	}

	// Update policy status
	return r.updatePolicyStatus(ctx, policy, deploymentRefs, log)
}

// findMatchingDeployments finds deployments that match the policy selector
func (r *PodPlacementPolicyController) findMatchingDeployments(ctx context.Context, policy *smartschedulerv1.PodPlacementPolicy) ([]appsv1.Deployment, error) {
	deploymentList := &appsv1.DeploymentList{}

	// If no selector is specified, return empty list
	if policy.Spec.Selector == nil {
		return []appsv1.Deployment{}, nil
	}

	// Convert LabelSelector to labels.Selector
	selector, err := metav1.LabelSelectorAsSelector(policy.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %w", err)
	}

	err = r.List(ctx, deploymentList, &client.ListOptions{
		Namespace:     policy.Namespace,
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	return deploymentList.Items, nil
}

// applyPolicyToDeployment applies the placement policy to a specific deployment
func (r *PodPlacementPolicyController) applyPolicyToDeployment(ctx context.Context, policy *smartschedulerv1.PodPlacementPolicy, deployment *appsv1.Deployment, log logr.Logger) (*smartschedulerv1.DeploymentReference, error) {
	deploymentLog := log.WithValues("deployment", deployment.Name)

	// Check if deployment already has a higher priority policy
	if r.hasHigherPriorityPolicy(deployment, policy) {
		deploymentLog.Info("Deployment already has higher priority policy, skipping")
		return nil, nil
	}

	// Convert CRD strategy to annotation format
	strategyAnnotation, err := r.convertStrategyToAnnotation(policy.Spec.Strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to convert strategy to annotation: %w", err)
	}

	// Update deployment annotations
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	// Apply the strategy annotation
	deployment.Annotations["smart-scheduler.io/schedule-strategy"] = strategyAnnotation
	deployment.Annotations["smart-scheduler.io/policy-name"] = policy.Name
	deployment.Annotations["smart-scheduler.io/policy-priority"] = fmt.Sprintf("%d", policy.Spec.Priority)
	deployment.Annotations["smart-scheduler.io/policy-applied"] = time.Now().Format(time.RFC3339)

	err = r.Update(ctx, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment: %w", err)
	}

	deploymentLog.Info("Applied placement policy to deployment")

	// Calculate current drift
	drift, err := r.calculateDeploymentDrift(ctx, deployment, policy)
	if err != nil {
		deploymentLog.Error(err, "Failed to calculate drift")
		drift = 0 // Use 0 as fallback
	}

	return &smartschedulerv1.DeploymentReference{
		Name:         deployment.Name,
		Namespace:    deployment.Namespace,
		CurrentDrift: drift,
		LastApplied:  &metav1.Time{Time: time.Now()},
	}, nil
}

// hasHigherPriorityPolicy checks if deployment already has a higher priority policy applied
func (r *PodPlacementPolicyController) hasHigherPriorityPolicy(deployment *appsv1.Deployment, policy *smartschedulerv1.PodPlacementPolicy) bool {
	if deployment.Annotations == nil {
		return false
	}

	priorityStr, exists := deployment.Annotations["smart-scheduler.io/policy-priority"]
	if !exists {
		return false
	}

	currentPriority := int32(0)
	fmt.Sscanf(priorityStr, "%d", &currentPriority)

	return currentPriority > policy.Spec.Priority
}

// convertStrategyToAnnotation converts CRD strategy to annotation format
func (r *PodPlacementPolicyController) convertStrategyToAnnotation(strategy smartschedulerv1.PlacementStrategySpec) (string, error) {
	// Convert to the annotation format: "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot"

	if len(strategy.Rules) == 0 {
		return "", fmt.Errorf("strategy must have at least one rule")
	}

	var parts []string

	// First rule includes base
	firstRule := strategy.Rules[0]
	firstPart := fmt.Sprintf("base=%d,weight=%d", strategy.Base, firstRule.Weight)

	if len(firstRule.NodeSelector) > 0 {
		nodeSelectorPart := ""
		for key, value := range firstRule.NodeSelector {
			if nodeSelectorPart != "" {
				nodeSelectorPart += ","
			}
			nodeSelectorPart += fmt.Sprintf("%s:%s", key, value)
		}
		firstPart += fmt.Sprintf(",nodeSelector=%s", nodeSelectorPart)
	}

	// Add affinity rules if present
	for _, affinity := range firstRule.Affinity {
		affinityPart := fmt.Sprintf("%s=", affinity.Type)
		for key, value := range affinity.LabelSelector {
			affinityPart += fmt.Sprintf("%s:%s", key, value)
			break // For simplicity, use first label pair
		}
		affinityPart += fmt.Sprintf(":%s", affinity.TopologyKey)
		if affinity.RequiredDuringScheduling {
			affinityPart += ":required"
		} else {
			affinityPart += ":preferred"
		}
		firstPart += "," + affinityPart
	}

	parts = append(parts, firstPart)

	// Additional rules
	for _, rule := range strategy.Rules[1:] {
		rulePart := fmt.Sprintf("weight=%d", rule.Weight)

		if len(rule.NodeSelector) > 0 {
			nodeSelectorPart := ""
			for key, value := range rule.NodeSelector {
				if nodeSelectorPart != "" {
					nodeSelectorPart += ","
				}
				nodeSelectorPart += fmt.Sprintf("%s:%s", key, value)
			}
			rulePart += fmt.Sprintf(",nodeSelector=%s", nodeSelectorPart)
		}

		// Add affinity rules if present
		for _, affinity := range rule.Affinity {
			affinityPart := fmt.Sprintf("%s=", affinity.Type)
			for key, value := range affinity.LabelSelector {
				affinityPart += fmt.Sprintf("%s:%s", key, value)
				break // For simplicity, use first label pair
			}
			affinityPart += fmt.Sprintf(":%s", affinity.TopologyKey)
			if affinity.RequiredDuringScheduling {
				affinityPart += ":required"
			} else {
				affinityPart += ":preferred"
			}
			rulePart += "," + affinityPart
		}

		parts = append(parts, rulePart)
	}

	return fmt.Sprintf("%s", parts[0]) + ";" + fmt.Sprintf("%s", parts[1:]), nil
}

// calculateDeploymentDrift calculates current placement drift for a deployment
func (r *PodPlacementPolicyController) calculateDeploymentDrift(ctx context.Context, deployment *appsv1.Deployment, policy *smartschedulerv1.PodPlacementPolicy) (float64, error) {
	// This is a simplified drift calculation
	// In a full implementation, this would use the StateManager and RebalanceController logic

	// Get pods for this deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

	err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     deployment.Namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list pods: %w", err)
	}

	totalPods := 0
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil &&
			(pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending) {
			totalPods++
		}
	}

	// For simplicity, return 0 drift for now
	// In a full implementation, this would calculate expected vs actual distribution
	return 0.0, nil
}

// updatePolicyStatus updates the status of the PodPlacementPolicy
func (r *PodPlacementPolicyController) updatePolicyStatus(ctx context.Context, policy *smartschedulerv1.PodPlacementPolicy, deploymentRefs []smartschedulerv1.DeploymentReference, log logr.Logger) (ctrl.Result, error) {
	// Update matched deployments
	policy.Status.MatchedDeployments = deploymentRefs

	// Calculate statistics
	totalPods := int32(0)
	totalDrift := 0.0
	for _, ref := range deploymentRefs {
		// This would need to be calculated from actual pod counts
		totalPods += 1 // Simplified
		totalDrift += ref.CurrentDrift
	}

	avgDrift := 0.0
	if len(deploymentRefs) > 0 {
		avgDrift = totalDrift / float64(len(deploymentRefs))
	}

	now := metav1.NewTime(time.Now())
	policy.Status.Statistics = &smartschedulerv1.PolicyStatistics{
		TotalPodsManaged: totalPods,
		AverageDrift:     avgDrift,
		LastUpdated:      &now,
	}

	// Update conditions
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "PolicyApplied",
		Message:            fmt.Sprintf("Policy applied to %d deployments", len(deploymentRefs)),
		LastTransitionTime: now,
	}

	// Update or add condition
	policy.Status.Conditions = []metav1.Condition{condition}
	policy.Status.ObservedGeneration = policy.Generation

	err := r.Status().Update(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to update policy status")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Requeue periodically to refresh status
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// handlePolicyDeletion cleans up when a policy is deleted
func (r *PodPlacementPolicyController) handlePolicyDeletion(ctx context.Context, policyKey types.NamespacedName, log logr.Logger) (ctrl.Result, error) {
	log.Info("Policy deleted, cleaning up applied annotations")

	// Find deployments with this policy applied
	deploymentList := &appsv1.DeploymentList{}
	err := r.List(ctx, deploymentList, &client.ListOptions{
		Namespace: policyKey.Namespace,
	})
	if err != nil {
		log.Error(err, "Failed to list deployments for cleanup")
		return ctrl.Result{}, err
	}

	for _, deployment := range deploymentList.Items {
		if deployment.Annotations != nil {
			if policyName, exists := deployment.Annotations["smart-scheduler.io/policy-name"]; exists && policyName == policyKey.Name {
				// Remove policy annotations
				delete(deployment.Annotations, "smart-scheduler.io/schedule-strategy")
				delete(deployment.Annotations, "smart-scheduler.io/policy-name")
				delete(deployment.Annotations, "smart-scheduler.io/policy-priority")
				delete(deployment.Annotations, "smart-scheduler.io/policy-applied")

				err = r.Update(ctx, &deployment)
				if err != nil {
					log.Error(err, "Failed to clean up deployment annotations", "deployment", deployment.Name)
				} else {
					log.Info("Cleaned up deployment annotations", "deployment", deployment.Name)
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *PodPlacementPolicyController) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize StateManager if not provided
	if r.StateManager == nil {
		r.StateManager = webhook.NewStateManager(mgr.GetClient(), r.Log.WithName("StateManager"))
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&smartschedulerv1.PodPlacementPolicy{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(r.mapDeploymentToPolicy),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}

// mapDeploymentToPolicy maps deployment events to policy reconcile requests
func (r *PodPlacementPolicyController) mapDeploymentToPolicy(ctx context.Context, obj client.Object) []ctrl.Request {
	deployment := obj.(*appsv1.Deployment)

	// Find policies that might match this deployment
	policyList := &smartschedulerv1.PodPlacementPolicyList{}
	err := r.List(ctx, policyList, &client.ListOptions{
		Namespace: deployment.Namespace,
	})
	if err != nil {
		return []ctrl.Request{}
	}

	var requests []ctrl.Request
	for _, policy := range policyList.Items {
		if policy.Spec.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.Spec.Selector)
			if err != nil {
				continue
			}

			if selector.Matches(labels.Set(deployment.Labels)) {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: policy.Namespace,
						Name:      policy.Name,
					},
				})
			}
		}
	}

	return requests
}
