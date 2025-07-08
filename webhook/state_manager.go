package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PlacementState represents the current state of a deployment's pod placement
type PlacementState struct {
	DeploymentName      string             `json:"deploymentName"`
	DeploymentNamespace string             `json:"deploymentNamespace"`
	Strategy            *PlacementStrategy `json:"strategy"`
	PodCounts           map[string]int     `json:"podCounts"`
	LastUpdated         time.Time          `json:"lastUpdated"`
	TotalPods           int                `json:"totalPods"`
}

// StateManager manages placement state using ConfigMaps for atomic updates
type StateManager struct {
	Client client.Client
	Log    logr.Logger
}

// NewStateManager creates a new state manager
func NewStateManager(client client.Client, log logr.Logger) *StateManager {
	return &StateManager{
		Client: client,
		Log:    log,
	}
}

// GetPlacementState retrieves the current placement state for a deployment
func (sm *StateManager) GetPlacementState(ctx context.Context, deployment *appsv1.Deployment, strategy *PlacementStrategy) (*PlacementState, error) {
	configMapName := sm.getConfigMapName(deployment)

	// Try to get existing ConfigMap
	configMap := &corev1.ConfigMap{}
	err := sm.Client.Get(ctx, client.ObjectKey{
		Namespace: deployment.Namespace,
		Name:      configMapName,
	}, configMap)

	if apierrors.IsNotFound(err) {
		// ConfigMap doesn't exist, create initial state
		return sm.createInitialState(ctx, deployment, strategy)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get placement state ConfigMap: %w", err)
	}

	// Parse existing state
	stateData, exists := configMap.Data["placement-state"]
	if !exists {
		sm.Log.Info("ConfigMap exists but no placement-state data, recreating",
			"configMap", configMapName)
		return sm.createInitialState(ctx, deployment, strategy)
	}

	var state PlacementState
	err = json.Unmarshal([]byte(stateData), &state)
	if err != nil {
		sm.Log.Error(err, "Failed to unmarshal placement state, recreating",
			"configMap", configMapName)
		return sm.createInitialState(ctx, deployment, strategy)
	}

	// Update strategy if it has changed
	state.Strategy = strategy

	// Refresh pod counts from actual pods
	actualCounts, err := sm.getCurrentPodCounts(ctx, deployment, strategy)
	if err != nil {
		sm.Log.Error(err, "Failed to get actual pod counts, using cached counts")
	} else {
		state.PodCounts = actualCounts
		state.TotalPods = 0
		for _, count := range actualCounts {
			state.TotalPods += count
		}
		state.LastUpdated = time.Now()
	}

	return &state, nil
}

// UpdatePlacementState atomically updates the placement state
func (sm *StateManager) UpdatePlacementState(ctx context.Context, state *PlacementState) error {
	configMapName := sm.getConfigMapName(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      state.DeploymentName,
			Namespace: state.DeploymentNamespace,
		},
	})

	// Update timestamp
	state.LastUpdated = time.Now()

	// Marshal state to JSON
	stateData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal placement state: %w", err)
	}

	// Create or update ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: state.DeploymentNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":        "smart-scheduler",
				"app.kubernetes.io/component":   "placement-state",
				"smart-scheduler.io/deployment": state.DeploymentName,
			},
		},
		Data: map[string]string{
			"placement-state": string(stateData),
			"last-updated":    state.LastUpdated.Format(time.RFC3339),
		},
	}

	// Try to get existing ConfigMap for optimistic locking
	existing := &corev1.ConfigMap{}
	err = sm.Client.Get(ctx, client.ObjectKey{
		Namespace: state.DeploymentNamespace,
		Name:      configMapName,
	}, existing)

	if apierrors.IsNotFound(err) {
		// Create new ConfigMap
		err = sm.Client.Create(ctx, configMap)
		if err != nil {
			return fmt.Errorf("failed to create placement state ConfigMap: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to get existing placement state ConfigMap: %w", err)
	} else {
		// Update existing ConfigMap
		configMap.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
		err = sm.Client.Update(ctx, configMap)
		if err != nil {
			return fmt.Errorf("failed to update placement state ConfigMap: %w", err)
		}
	}

	return nil
}

// IncrementPodCount atomically increments the count for a specific rule
func (sm *StateManager) IncrementPodCount(ctx context.Context, deployment *appsv1.Deployment, ruleKey string) error {
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		// Get current state
		strategy, err := ParsePlacementStrategy(deployment.Annotations["smart-scheduler.io/schedule-strategy"])
		if err != nil {
			return fmt.Errorf("failed to parse strategy: %w", err)
		}

		state, err := sm.GetPlacementState(ctx, deployment, strategy)
		if err != nil {
			return fmt.Errorf("failed to get placement state: %w", err)
		}

		// Increment count
		if state.PodCounts == nil {
			state.PodCounts = make(map[string]int)
		}
		state.PodCounts[ruleKey]++
		state.TotalPods++

		// Try to update
		err = sm.UpdatePlacementState(ctx, state)
		if err == nil {
			sm.Log.Info("Successfully incremented pod count",
				"deployment", deployment.Name,
				"ruleKey", ruleKey,
				"newCount", state.PodCounts[ruleKey])
			return nil
		}

		// If update failed due to conflict, retry
		if apierrors.IsConflict(err) {
			sm.Log.Info("Conflict updating placement state, retrying",
				"attempt", i+1, "error", err)
			time.Sleep(time.Millisecond * 100 * time.Duration(i+1)) // exponential backoff
			continue
		}

		return err
	}

	return fmt.Errorf("failed to increment pod count after %d retries", maxRetries)
}

// createInitialState creates initial placement state by counting existing pods
func (sm *StateManager) createInitialState(ctx context.Context, deployment *appsv1.Deployment, strategy *PlacementStrategy) (*PlacementState, error) {
	counts, err := sm.getCurrentPodCounts(ctx, deployment, strategy)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial pod counts: %w", err)
	}

	totalPods := 0
	for _, count := range counts {
		totalPods += count
	}

	state := &PlacementState{
		DeploymentName:      deployment.Name,
		DeploymentNamespace: deployment.Namespace,
		Strategy:            strategy,
		PodCounts:           counts,
		LastUpdated:         time.Now(),
		TotalPods:           totalPods,
	}

	// Save initial state
	err = sm.UpdatePlacementState(ctx, state)
	if err != nil {
		sm.Log.Error(err, "Failed to save initial placement state, continuing with in-memory state")
		// Don't fail the request, just use in-memory state
	}

	return state, nil
}

// getCurrentPodCounts gets the current pod distribution for a deployment
func (sm *StateManager) getCurrentPodCounts(ctx context.Context, deployment *appsv1.Deployment, strategy *PlacementStrategy) (map[string]int, error) {
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

	err := sm.Client.List(ctx, podList, &client.ListOptions{
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

// getConfigMapName generates a consistent ConfigMap name for a deployment
func (sm *StateManager) getConfigMapName(deployment *appsv1.Deployment) string {
	return fmt.Sprintf("smart-scheduler-%s", deployment.Name)
}

// CleanupStaleStates removes ConfigMaps for deleted deployments
func (sm *StateManager) CleanupStaleStates(ctx context.Context, namespace string) error {
	// List all smart-scheduler ConfigMaps in the namespace
	configMapList := &corev1.ConfigMapList{}
	err := sm.Client.List(ctx, configMapList, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/name":      "smart-scheduler",
			"app.kubernetes.io/component": "placement-state",
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to list placement state ConfigMaps: %w", err)
	}

	for _, configMap := range configMapList.Items {
		deploymentName, exists := configMap.Labels["smart-scheduler.io/deployment"]
		if !exists {
			continue
		}

		// Check if deployment still exists
		deployment := &appsv1.Deployment{}
		err := sm.Client.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      deploymentName,
		}, deployment)

		if apierrors.IsNotFound(err) {
			// Deployment no longer exists, delete the ConfigMap
			sm.Log.Info("Cleaning up stale placement state ConfigMap",
				"configMap", configMap.Name, "deployment", deploymentName)

			err = sm.Client.Delete(ctx, &configMap)
			if err != nil {
				sm.Log.Error(err, "Failed to delete stale placement state ConfigMap",
					"configMap", configMap.Name)
			}
		}
	}

	return nil
}
