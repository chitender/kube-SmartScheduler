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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kube-smartscheduler/smart-scheduler/webhook"
)

// RebalanceController monitors pods and deployments for placement drift and rebalancing needs
type RebalanceController struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	StateManager *webhook.StateManager
}

// DriftReport represents placement drift for a deployment
type DriftReport struct {
	DeploymentName      string         `json:"deploymentName"`
	DeploymentNamespace string         `json:"deploymentNamespace"`
	ExpectedCounts      map[string]int `json:"expectedCounts"`
	ActualCounts        map[string]int `json:"actualCounts"`
	DriftPercentage     float64        `json:"driftPercentage"`
	RequiresRebalance   bool           `json:"requiresRebalance"`
	Timestamp           time.Time      `json:"timestamp"`
}

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// ruleToString converts a placement rule to a string key for tracking
func ruleToString(rule webhook.PlacementRule) string {
	return nodeSelector2String(rule.NodeSelector)
}

// nodeSelector2String converts a nodeSelector map to a string key for tracking
func nodeSelector2String(nodeSelector map[string]string) string {
	if len(nodeSelector) == 0 {
		return ""
	}

	var parts []string
	for key, value := range nodeSelector {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}

	return fmt.Sprintf("%v", parts) // Simple string representation
}

// Reconcile handles rebalancing requests and placement drift detection
func (r *RebalanceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	startTime := time.Now()
	log := r.Log.WithValues("rebalance", req.NamespacedName, "reconcileID", generateRebalanceReconcileID())

	// Add comprehensive reconciliation logging
	log.Info("=== REBALANCE RECONCILE START ===",
		"requestedName", req.Name,
		"requestedNamespace", req.Namespace,
		"timestamp", startTime.Format(time.RFC3339))

	defer func() {
		duration := time.Since(startTime)
		log.Info("=== REBALANCE RECONCILE END ===",
			"duration", duration.String(),
			"durationMs", duration.Milliseconds())
	}()

	// Check if this is a Deployment or Pod event
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, req.NamespacedName, deployment)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Deployment not found, likely deleted")
			// Object deleted, clean up state
			return r.handleDeploymentDeletion(ctx, req.NamespacedName, log)
		}
		log.Error(err, "Failed to get deployment")
		return ctrl.Result{}, err
	}

	log.Info("Found deployment for rebalance check",
		"deploymentName", deployment.Name,
		"deploymentNamespace", deployment.Namespace,
		"uid", deployment.UID,
		"generation", deployment.Generation,
		"observedGeneration", deployment.Status.ObservedGeneration,
		"replicas", deployment.Spec.Replicas,
		"availableReplicas", deployment.Status.AvailableReplicas,
		"readyReplicas", deployment.Status.ReadyReplicas,
		"resourceVersion", deployment.ResourceVersion)

	// Check if deployment has smart scheduler annotations
	if deployment.Annotations == nil {
		log.Info("Deployment has no annotations, skipping rebalance check")
		return ctrl.Result{}, nil
	}

	log.Info("Deployment annotations for rebalance",
		"annotationCount", len(deployment.Annotations),
		"hasScheduleStrategy", deployment.Annotations["smart-scheduler.io/schedule-strategy"] != "",
		"hasPolicyName", deployment.Annotations["smart-scheduler.io/policy-name"] != "",
		"hasProcessed", deployment.Annotations["smart-scheduler.io/processed"] != "")

	scheduleStrategy, exists := deployment.Annotations["smart-scheduler.io/schedule-strategy"]
	if !exists {
		log.Info("No schedule strategy annotation found, skipping rebalance")
		return ctrl.Result{}, nil
	}

	log.Info("Processing rebalance check for deployment", "strategy", scheduleStrategy)

	// Parse the strategy
	strategy, err := webhook.ParsePlacementStrategy(scheduleStrategy)
	if err != nil {
		log.Error(err, "Failed to parse placement strategy")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	log.Info("Parsed strategy for rebalance",
		"base", strategy.Base,
		"rulesCount", len(strategy.Rules))

	// Get current placement state
	placementState, err := r.StateManager.GetPlacementState(ctx, deployment, strategy)
	if err != nil {
		log.Error(err, "Failed to get placement state")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	log.Info("Current placement state for rebalance",
		"totalPods", placementState.TotalPods,
		"podCounts", placementState.PodCounts,
		"lastUpdated", placementState.LastUpdated)

	// Calculate drift
	driftReport, err := r.calculateDrift(ctx, deployment, strategy, placementState)
	if err != nil {
		log.Error(err, "Failed to calculate drift")
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	log.Info("Drift analysis complete",
		"driftPercentage", driftReport.DriftPercentage,
		"requiresRebalance", driftReport.RequiresRebalance,
		"expectedCounts", driftReport.ExpectedCounts,
		"actualCounts", driftReport.ActualCounts)

	// Handle rebalancing if needed
	if driftReport.RequiresRebalance {
		log.Info("Rebalancing required, proceeding with rebalance operation")
		return r.performRebalancing(ctx, deployment, strategy, driftReport, log)
	}

	log.Info("No rebalancing required, scheduling next check")
	// Schedule next check
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

// generateRebalanceReconcileID creates a unique ID for each rebalance reconciliation
func generateRebalanceReconcileID() string {
	return "rebalance-" + time.Now().Format("20060102150405.000000")
}

// calculateDrift analyzes the current placement vs expected placement
func (r *RebalanceController) calculateDrift(ctx context.Context, deployment *appsv1.Deployment, strategy *webhook.PlacementStrategy, state *webhook.PlacementState) (*DriftReport, error) {
	// Get actual pod counts by querying current pods
	actualCounts, err := r.getActualPodCounts(ctx, deployment, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get actual pod counts: %w", err)
	}

	// Calculate expected distribution
	expectedCounts := r.calculateExpectedDistribution(strategy, state.TotalPods)

	// Calculate drift percentage
	totalDrift := 0
	totalExpected := 0
	for ruleKey, expected := range expectedCounts {
		actual := actualCounts[ruleKey]
		drift := abs(expected - actual)
		totalDrift += drift
		totalExpected += expected
	}

	driftPercentage := 0.0
	if totalExpected > 0 {
		driftPercentage = float64(totalDrift) / float64(totalExpected) * 100
	}

	// Determine if rebalancing is required (>20% drift)
	requiresRebalance := driftPercentage > 20.0

	return &DriftReport{
		DeploymentName:      deployment.Name,
		DeploymentNamespace: deployment.Namespace,
		ExpectedCounts:      expectedCounts,
		ActualCounts:        actualCounts,
		DriftPercentage:     driftPercentage,
		RequiresRebalance:   requiresRebalance,
		Timestamp:           time.Now(),
	}, nil
}

// calculateExpectedDistribution calculates expected pod distribution based on strategy
func (r *RebalanceController) calculateExpectedDistribution(strategy *webhook.PlacementStrategy, totalPods int) map[string]int {
	expected := make(map[string]int)

	// Initialize all rule counts
	for _, rule := range strategy.Rules {
		ruleKey := ruleToString(rule)
		expected[ruleKey] = 0
	}

	if totalPods <= strategy.Base {
		// All pods should be on first rule
		if len(strategy.Rules) > 0 {
			firstRuleKey := ruleToString(strategy.Rules[0])
			expected[firstRuleKey] = totalPods
		}
		return expected
	}

	// Base pods on first rule
	if len(strategy.Rules) > 0 {
		firstRuleKey := ruleToString(strategy.Rules[0])
		expected[firstRuleKey] = strategy.Base
	}

	// Distribute remaining pods by weight
	remainingPods := totalPods - strategy.Base
	totalWeight := 0
	for _, rule := range strategy.Rules {
		totalWeight += rule.Weight
	}

	if totalWeight > 0 {
		for _, rule := range strategy.Rules {
			ruleKey := ruleToString(rule)
			weightedCount := int(float64(remainingPods) * float64(rule.Weight) / float64(totalWeight))
			expected[ruleKey] += weightedCount
		}
	}

	return expected
}

// performRebalancing performs the actual rebalancing by selectively deleting pods
func (r *RebalanceController) performRebalancing(ctx context.Context, deployment *appsv1.Deployment, strategy *webhook.PlacementStrategy, drift *DriftReport, log logr.Logger) (ctrl.Result, error) {
	log.Info("Starting rebalancing process", "driftPercentage", drift.DriftPercentage)

	// Get all pods for this deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

	err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     deployment.Namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list pods: %w", err)
	}

	// Identify pods to delete for rebalancing
	podsToDelete := r.selectPodsForRebalancing(podList.Items, drift)

	// Delete pods gradually (max 1 at a time to avoid disruption)
	deletedCount := 0
	maxDeletions := 1

	for _, pod := range podsToDelete {
		if deletedCount >= maxDeletions {
			break
		}

		// Skip pods already being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		log.Info("Deleting pod for rebalancing", "pod", pod.Name, "nodeSelector", pod.Spec.NodeSelector)

		err = r.Delete(ctx, &pod)
		if err != nil {
			log.Error(err, "Failed to delete pod", "pod", pod.Name)
			continue
		}

		deletedCount++

		// Create event for visibility
		r.createRebalanceEvent(ctx, deployment, pod.Name, "PodDeleted",
			fmt.Sprintf("Pod deleted for placement rebalancing, drift: %.1f%%", drift.DriftPercentage))
	}

	if deletedCount > 0 {
		log.Info("Rebalancing in progress", "deletedPods", deletedCount)
		// Requeue sooner to monitor rebalancing progress
		return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
	}

	log.Info("No pods deleted, rebalancing may be complete")
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// selectPodsForRebalancing identifies which pods should be deleted for rebalancing
func (r *RebalanceController) selectPodsForRebalancing(pods []corev1.Pod, drift *DriftReport) []corev1.Pod {
	var podsToDelete []corev1.Pod

	// Group pods by rule key
	podsByRule := make(map[string][]corev1.Pod)
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		ruleKey := nodeSelector2String(pod.Spec.NodeSelector)
		podsByRule[ruleKey] = append(podsByRule[ruleKey], pod)
	}

	// Delete pods from over-allocated rules
	for ruleKey, actual := range drift.ActualCounts {
		expected := drift.ExpectedCounts[ruleKey]
		if actual > expected {
			// This rule has too many pods
			excess := actual - expected
			rulePods := podsByRule[ruleKey]

			// Sort pods by creation time (delete newest first to preserve disruption)
			if len(rulePods) > 0 {
				for i := 0; i < excess && i < len(rulePods); i++ {
					podsToDelete = append(podsToDelete, rulePods[i])
				}
			}
		}
	}

	return podsToDelete
}

// getActualPodCounts gets current pod counts from the cluster
func (r *RebalanceController) getActualPodCounts(ctx context.Context, deployment *appsv1.Deployment, strategy *webhook.PlacementStrategy) (map[string]int, error) {
	counts := make(map[string]int)

	// Initialize counts for all rules
	for _, rule := range strategy.Rules {
		ruleKey := ruleToString(rule)
		counts[ruleKey] = 0
	}

	// Get all pods for this deployment
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)

	err := r.List(ctx, podList, &client.ListOptions{
		Namespace:     deployment.Namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Count pods by their nodeSelector
	for _, pod := range podList.Items {
		// Skip pods that are being deleted or failed
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		// Convert pod's nodeSelector to string key
		podKey := nodeSelector2String(pod.Spec.NodeSelector)

		// Find matching rule
		for _, rule := range strategy.Rules {
			ruleKey := ruleToString(rule)
			if podKey == ruleKey {
				counts[ruleKey]++
				break
			}
		}
	}

	return counts, nil
}

// handleDeploymentDeletion cleans up state when deployment is deleted
func (r *RebalanceController) handleDeploymentDeletion(ctx context.Context, deploymentKey types.NamespacedName, log logr.Logger) (ctrl.Result, error) {
	log.Info("Deployment deleted, cleaning up placement state")

	err := r.StateManager.CleanupStaleStates(ctx, deploymentKey.Namespace)
	if err != nil {
		log.Error(err, "Failed to cleanup stale states")
	}

	return ctrl.Result{}, nil
}

// createRebalanceEvent creates a Kubernetes event for rebalancing actions
func (r *RebalanceController) createRebalanceEvent(ctx context.Context, deployment *appsv1.Deployment, podName, reason, message string) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("smart-scheduler-%d", time.Now().Unix()),
			Namespace: deployment.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       "Deployment",
			Name:       deployment.Name,
			Namespace:  deployment.Namespace,
			UID:        deployment.UID,
			APIVersion: "apps/v1",
		},
		Reason:  reason,
		Message: message,
		Type:    "Normal",
		Source: corev1.EventSource{
			Component: "smart-scheduler-rebalancer",
		},
		FirstTimestamp: metav1.NewTime(time.Now()),
		LastTimestamp:  metav1.NewTime(time.Now()),
	}

	err := r.Create(ctx, event)
	if err != nil {
		r.Log.Error(err, "Failed to create rebalance event")
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *RebalanceController) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize StateManager if not provided
	if r.StateManager == nil {
		r.StateManager = webhook.NewStateManager(mgr.GetClient(), r.Log.WithName("StateManager"))
	}

	// Create deployment-specific predicates
	deploymentPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Type guard to ensure we only handle Deployments
			deployment, ok := e.Object.(*appsv1.Deployment)
			if !ok {
				return false
			}
			log := r.Log.WithValues("eventType", "CREATE", "deploymentName", deployment.Name, "namespace", deployment.Namespace)

			hasStrategy := deployment.Annotations != nil && deployment.Annotations["smart-scheduler.io/schedule-strategy"] != ""
			log.Info("Deployment CREATE event for rebalance controller",
				"hasScheduleStrategy", hasStrategy)

			return hasStrategy
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Type guard to ensure we only handle Deployments
			oldDep, oldOk := e.ObjectOld.(*appsv1.Deployment)
			newDep, newOk := e.ObjectNew.(*appsv1.Deployment)
			if !oldOk || !newOk {
				return false
			}

			log := r.Log.WithValues("eventType", "UPDATE", "deploymentName", newDep.Name, "namespace", newDep.Namespace)

			// Check if deployment has scheduling strategy
			oldStrategy := ""
			newStrategy := ""
			if oldDep.Annotations != nil {
				oldStrategy = oldDep.Annotations["smart-scheduler.io/schedule-strategy"]
			}
			if newDep.Annotations != nil {
				newStrategy = newDep.Annotations["smart-scheduler.io/schedule-strategy"]
			}

			hasStrategy := newStrategy != ""
			strategyChanged := oldStrategy != newStrategy
			generationChanged := oldDep.Generation != newDep.Generation
			statusChanged := oldDep.Status.ReadyReplicas != newDep.Status.ReadyReplicas ||
				oldDep.Status.AvailableReplicas != newDep.Status.AvailableReplicas

			shouldReconcile := hasStrategy && (strategyChanged || generationChanged || statusChanged)

			log.Info("Deployment UPDATE event evaluation for rebalance controller",
				"hasStrategy", hasStrategy,
				"strategyChanged", strategyChanged,
				"generationChanged", generationChanged,
				"statusChanged", statusChanged,
				"oldStrategy", oldStrategy,
				"newStrategy", newStrategy,
				"oldGeneration", oldDep.Generation,
				"newGeneration", newDep.Generation,
				"oldReadyReplicas", oldDep.Status.ReadyReplicas,
				"newReadyReplicas", newDep.Status.ReadyReplicas,
				"shouldReconcile", shouldReconcile)

			return shouldReconcile
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Type guard to ensure we only handle Deployments
			deployment, ok := e.Object.(*appsv1.Deployment)
			if !ok {
				return false
			}
			log := r.Log.WithValues("eventType", "DELETE", "deploymentName", deployment.Name, "namespace", deployment.Namespace)

			hasStrategy := deployment.Annotations != nil && deployment.Annotations["smart-scheduler.io/schedule-strategy"] != ""
			log.Info("Deployment DELETE event for rebalance controller",
				"hasScheduleStrategy", hasStrategy)

			return hasStrategy
		},
	}

	// Create pod-specific predicates
	podPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Type guard to ensure we only handle Pods
			pod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return false
			}
			log := r.Log.WithValues("eventType", "POD_CREATE", "podName", pod.Name, "namespace", pod.Namespace)

			hasSmartSchedulerAnnotation := pod.Annotations != nil &&
				(pod.Annotations["smart-scheduler.io/processed"] != "" ||
					pod.Annotations["smart-scheduler.io/strategy-applied"] != "")

			log.Info("Pod CREATE event for rebalance controller",
				"hasSmartSchedulerAnnotation", hasSmartSchedulerAnnotation,
				"hasOwnerRef", len(pod.OwnerReferences) > 0)

			return hasSmartSchedulerAnnotation
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Type guard to ensure we only handle Pods
			oldPod, oldOk := e.ObjectOld.(*corev1.Pod)
			newPod, newOk := e.ObjectNew.(*corev1.Pod)
			if !oldOk || !newOk {
				return false
			}

			log := r.Log.WithValues("eventType", "POD_UPDATE", "podName", newPod.Name, "namespace", newPod.Namespace)

			// Only care about smart scheduler managed pods
			hasSmartSchedulerAnnotation := newPod.Annotations != nil &&
				(newPod.Annotations["smart-scheduler.io/processed"] != "" ||
					newPod.Annotations["smart-scheduler.io/strategy-applied"] != "")

			if !hasSmartSchedulerAnnotation {
				return false
			}

			// Only trigger on significant status changes
			phaseChanged := oldPod.Status.Phase != newPod.Status.Phase
			beingDeleted := newPod.DeletionTimestamp != nil && oldPod.DeletionTimestamp == nil

			shouldReconcile := phaseChanged || beingDeleted

			log.Info("Pod UPDATE event evaluation for rebalance controller",
				"hasSmartSchedulerAnnotation", hasSmartSchedulerAnnotation,
				"phaseChanged", phaseChanged,
				"beingDeleted", beingDeleted,
				"oldPhase", oldPod.Status.Phase,
				"newPhase", newPod.Status.Phase,
				"shouldReconcile", shouldReconcile)

			return shouldReconcile
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Type guard to ensure we only handle Pods
			pod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return false
			}
			log := r.Log.WithValues("eventType", "POD_DELETE", "podName", pod.Name, "namespace", pod.Namespace)

			hasSmartSchedulerAnnotation := pod.Annotations != nil &&
				(pod.Annotations["smart-scheduler.io/processed"] != "" ||
					pod.Annotations["smart-scheduler.io/strategy-applied"] != "")

			log.Info("Pod DELETE event for rebalance controller",
				"hasSmartSchedulerAnnotation", hasSmartSchedulerAnnotation)

			return hasSmartSchedulerAnnotation
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithEventFilter(deploymentPredicates).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToDeployment),
			builder.WithPredicates(podPredicates),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // Reduce concurrency to avoid overlapping reconciliations
		}).
		Complete(r)
}

// mapPodToDeployment maps pod events to deployment reconcile requests
func (r *RebalanceController) mapPodToDeployment(ctx context.Context, obj client.Object) []ctrl.Request {
	pod := obj.(*corev1.Pod)

	// Find parent deployment
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			// Get the ReplicaSet to find parent deployment
			rs := &appsv1.ReplicaSet{}
			err := r.Get(context.Background(), types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      ownerRef.Name,
			}, rs)
			if err != nil {
				continue
			}

			// Check if ReplicaSet has a Deployment owner
			for _, rsOwnerRef := range rs.OwnerReferences {
				if rsOwnerRef.Kind == "Deployment" {
					return []ctrl.Request{
						{
							NamespacedName: types.NamespacedName{
								Namespace: pod.Namespace,
								Name:      rsOwnerRef.Name,
							},
						},
					}
				}
			}
		}
	}

	return []ctrl.Request{}
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
