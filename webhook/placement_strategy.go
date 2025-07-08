package webhook

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AffinityRule represents pod affinity or anti-affinity configuration
type AffinityRule struct {
	Type                     string            `json:"type"` // "affinity" or "anti-affinity"
	LabelSelector            map[string]string `json:"labelSelector"`
	TopologyKey              string            `json:"topologyKey"`
	RequiredDuringScheduling bool              `json:"requiredDuringScheduling"`
}

// PlacementRule represents a single placement rule with weight, node selector, and affinity
type PlacementRule struct {
	Weight       int               `json:"weight"`
	NodeSelector map[string]string `json:"nodeSelector"`
	Affinity     []AffinityRule    `json:"affinity,omitempty"`
}

// PlacementStrategy represents the complete placement strategy for a workload
type PlacementStrategy struct {
	Base  int             `json:"base"`
	Rules []PlacementRule `json:"rules"`
}

// ParsePlacementStrategy parses the custom scheduling annotation into a structured strategy
// Enhanced format: "base=1,weight=1,nodeSelector=node-type:ondemand,affinity=app:web-app:zone:preferred;weight=2,nodeSelector=node-type:spot,anti-affinity=app:web-app:zone:required"
func ParsePlacementStrategy(annotation string) (*PlacementStrategy, error) {
	if annotation == "" {
		return nil, fmt.Errorf("empty annotation")
	}

	strategy := &PlacementStrategy{
		Rules: make([]PlacementRule, 0),
	}

	// Split by semicolon to get individual rules
	parts := strings.Split(annotation, ";")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if i == 0 {
			// First part should contain base count and first rule
			if err := parseFirstRule(part, strategy); err != nil {
				return nil, fmt.Errorf("failed to parse first rule: %w", err)
			}
		} else {
			// Subsequent parts are additional rules
			rule, err := parseRule(part)
			if err != nil {
				return nil, fmt.Errorf("failed to parse rule %d: %w", i, err)
			}
			strategy.Rules = append(strategy.Rules, *rule)
		}
	}

	if len(strategy.Rules) == 0 {
		return nil, fmt.Errorf("no placement rules found")
	}

	return strategy, nil
}

// parseFirstRule parses the first rule which includes the base count
// Format: "base=1,weight=1,nodeSelector=node-type:ondemand,affinity=app:web-app:zone:preferred"
func parseFirstRule(part string, strategy *PlacementStrategy) error {
	// Split by comma to get individual parameters
	params := strings.Split(part, ",")

	rule := PlacementRule{
		NodeSelector: make(map[string]string),
		Affinity:     make([]AffinityRule, 0),
	}

	for _, param := range params {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}

		if strings.HasPrefix(param, "base=") {
			baseStr := strings.TrimPrefix(param, "base=")
			base, err := strconv.Atoi(baseStr)
			if err != nil {
				return fmt.Errorf("invalid base count: %s", baseStr)
			}
			strategy.Base = base
		} else if strings.HasPrefix(param, "weight=") {
			weightStr := strings.TrimPrefix(param, "weight=")
			weight, err := strconv.Atoi(weightStr)
			if err != nil {
				return fmt.Errorf("invalid weight: %s", weightStr)
			}
			rule.Weight = weight
		} else if strings.HasPrefix(param, "nodeSelector=") {
			nodeSelectorStr := strings.TrimPrefix(param, "nodeSelector=")
			if err := parseNodeSelector(nodeSelectorStr, rule.NodeSelector); err != nil {
				return fmt.Errorf("invalid nodeSelector: %w", err)
			}
		} else if strings.HasPrefix(param, "affinity=") || strings.HasPrefix(param, "anti-affinity=") {
			affinityRule, err := parseAffinityRule(param)
			if err != nil {
				return fmt.Errorf("invalid affinity rule: %w", err)
			}
			rule.Affinity = append(rule.Affinity, *affinityRule)
		}
	}

	strategy.Rules = append(strategy.Rules, rule)
	return nil
}

// parseRule parses a subsequent rule
// Format: "weight=2,nodeSelector=node-type:spot,anti-affinity=app:web-app:zone:required"
func parseRule(part string) (*PlacementRule, error) {
	rule := &PlacementRule{
		NodeSelector: make(map[string]string),
		Affinity:     make([]AffinityRule, 0),
	}

	// Split by comma to get individual parameters
	params := strings.Split(part, ",")

	for _, param := range params {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}

		if strings.HasPrefix(param, "weight=") {
			weightStr := strings.TrimPrefix(param, "weight=")
			weight, err := strconv.Atoi(weightStr)
			if err != nil {
				return nil, fmt.Errorf("invalid weight: %s", weightStr)
			}
			rule.Weight = weight
		} else if strings.HasPrefix(param, "nodeSelector=") {
			nodeSelectorStr := strings.TrimPrefix(param, "nodeSelector=")
			if err := parseNodeSelector(nodeSelectorStr, rule.NodeSelector); err != nil {
				return nil, fmt.Errorf("invalid nodeSelector: %w", err)
			}
		} else if strings.HasPrefix(param, "affinity=") || strings.HasPrefix(param, "anti-affinity=") {
			affinityRule, err := parseAffinityRule(param)
			if err != nil {
				return nil, fmt.Errorf("invalid affinity rule: %w", err)
			}
			rule.Affinity = append(rule.Affinity, *affinityRule)
		}
	}

	return rule, nil
}

// parseAffinityRule parses affinity or anti-affinity rule
// Format: "affinity=app:web-app:zone:preferred" or "anti-affinity=app:web-app:zone:required"
func parseAffinityRule(param string) (*AffinityRule, error) {
	var affinityType string
	var ruleStr string

	if strings.HasPrefix(param, "affinity=") {
		affinityType = "affinity"
		ruleStr = strings.TrimPrefix(param, "affinity=")
	} else if strings.HasPrefix(param, "anti-affinity=") {
		affinityType = "anti-affinity"
		ruleStr = strings.TrimPrefix(param, "anti-affinity=")
	} else {
		return nil, fmt.Errorf("unknown affinity type")
	}

	// Parse rule: "app:web-app:zone:preferred"
	parts := strings.Split(ruleStr, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid affinity rule format, expected labelKey:labelValue:topologyKey:scheduling, got: %s", ruleStr)
	}

	labelKey := strings.TrimSpace(parts[0])
	labelValue := strings.TrimSpace(parts[1])
	topologyKey := strings.TrimSpace(parts[2])
	scheduling := strings.TrimSpace(parts[3])

	if labelKey == "" || labelValue == "" || topologyKey == "" {
		return nil, fmt.Errorf("empty label key, value, or topology key in affinity rule: %s", ruleStr)
	}

	requiredDuringScheduling := false
	if scheduling == "required" {
		requiredDuringScheduling = true
	} else if scheduling != "preferred" {
		return nil, fmt.Errorf("invalid scheduling preference, must be 'required' or 'preferred': %s", scheduling)
	}

	return &AffinityRule{
		Type: affinityType,
		LabelSelector: map[string]string{
			labelKey: labelValue,
		},
		TopologyKey:              topologyKey,
		RequiredDuringScheduling: requiredDuringScheduling,
	}, nil
}

// parseNodeSelector parses nodeSelector string into map
// Format: "node-type:ondemand" or "node-type:spot,zone:us-west-1"
func parseNodeSelector(nodeSelectorStr string, nodeSelector map[string]string) error {
	// Handle multiple key:value pairs separated by commas within the nodeSelector
	pairs := strings.Split(nodeSelectorStr, ",")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.Split(pair, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid nodeSelector format, expected key:value, got: %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			return fmt.Errorf("empty key or value in nodeSelector: %s", pair)
		}

		nodeSelector[key] = value
	}

	if len(nodeSelector) == 0 {
		return fmt.Errorf("no valid nodeSelector pairs found")
	}

	return nil
}

// ApplyPlacementStrategy applies the placement strategy to a pod based on current pod counts
func ApplyPlacementStrategy(pod *corev1.Pod, strategy *PlacementStrategy, currentCounts map[string]int) error {
	if strategy == nil || len(strategy.Rules) == 0 {
		return fmt.Errorf("invalid placement strategy")
	}

	// Calculate total pods placed so far
	totalPods := 0
	for _, count := range currentCounts {
		totalPods += count
	}

	// Determine which rule to apply
	if totalPods < strategy.Base && len(strategy.Rules) > 0 {
		// Apply the first rule for base pods
		return applyRule(pod, strategy.Rules[0])
	}

	// For pods beyond the base count, use weighted distribution
	return applyWeightedRule(pod, strategy, currentCounts, totalPods)
}

// applyRule applies a specific placement rule to the pod
func applyRule(pod *corev1.Pod, rule PlacementRule) error {
	// Apply nodeSelector
	if len(rule.NodeSelector) > 0 {
		if pod.Spec.NodeSelector == nil {
			pod.Spec.NodeSelector = make(map[string]string)
		}
		// Merge nodeSelector
		for key, value := range rule.NodeSelector {
			pod.Spec.NodeSelector[key] = value
		}
	}

	// Apply affinity rules
	if len(rule.Affinity) > 0 {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &corev1.Affinity{}
		}

		for _, affinityRule := range rule.Affinity {
			if err := applyAffinityRule(pod, affinityRule); err != nil {
				return fmt.Errorf("failed to apply affinity rule: %w", err)
			}
		}
	}

	return nil
}

// applyAffinityRule applies a single affinity rule to the pod
func applyAffinityRule(pod *corev1.Pod, rule AffinityRule) error {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: rule.LabelSelector,
	}

	if rule.Type == "affinity" {
		// Pod affinity
		if pod.Spec.Affinity.PodAffinity == nil {
			pod.Spec.Affinity.PodAffinity = &corev1.PodAffinity{}
		}

		affinityTerm := corev1.PodAffinityTerm{
			LabelSelector: labelSelector,
			TopologyKey:   rule.TopologyKey,
		}

		if rule.RequiredDuringScheduling {
			pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
				pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
				affinityTerm)
		} else {
			weightedTerm := corev1.WeightedPodAffinityTerm{
				Weight:          100, // Default weight
				PodAffinityTerm: affinityTerm,
			}
			pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				weightedTerm)
		}
	} else if rule.Type == "anti-affinity" {
		// Pod anti-affinity
		if pod.Spec.Affinity.PodAntiAffinity == nil {
			pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
		}

		affinityTerm := corev1.PodAffinityTerm{
			LabelSelector: labelSelector,
			TopologyKey:   rule.TopologyKey,
		}

		if rule.RequiredDuringScheduling {
			pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(
				pod.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
				affinityTerm)
		} else {
			weightedTerm := corev1.WeightedPodAffinityTerm{
				Weight:          100, // Default weight
				PodAffinityTerm: affinityTerm,
			}
			pod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
				pod.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
				weightedTerm)
		}
	}

	return nil
}

// applyWeightedRule applies weighted distribution logic beyond base count
func applyWeightedRule(pod *corev1.Pod, strategy *PlacementStrategy, currentCounts map[string]int, totalPods int) error {
	if len(strategy.Rules) == 0 {
		return fmt.Errorf("no rules available for weighted distribution")
	}

	// Calculate total weight
	totalWeight := 0
	for _, rule := range strategy.Rules {
		totalWeight += rule.Weight
	}

	if totalWeight == 0 {
		return fmt.Errorf("total weight is zero")
	}

	// Find the rule that should get the next pod based on current distribution
	podsBeyondBase := totalPods - strategy.Base
	if podsBeyondBase < 0 {
		podsBeyondBase = 0
	}

	// Calculate expected distribution for each rule
	bestRule := strategy.Rules[0]
	bestDeficit := -1.0

	for _, rule := range strategy.Rules {
		ruleKey := ruleToString(rule)
		currentCount := currentCounts[ruleKey]

		// Calculate expected count for this rule
		expectedRatio := float64(rule.Weight) / float64(totalWeight)
		expectedCount := int(expectedRatio * float64(podsBeyondBase))

		// Calculate deficit (how many pods this rule is behind)
		deficit := float64(expectedCount - currentCount)

		if deficit > bestDeficit {
			bestDeficit = deficit
			bestRule = rule
		}
	}

	return applyRule(pod, bestRule)
}

// ruleToString converts a placement rule to a string key for tracking
func ruleToString(rule PlacementRule) string {
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

	// Sort for consistent string representation
	return strings.Join(parts, ",")
}
