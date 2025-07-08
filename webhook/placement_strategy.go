package webhook

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// PlacementRule represents a single placement rule with weight and node selector
type PlacementRule struct {
	Weight       int               `json:"weight"`
	NodeSelector map[string]string `json:"nodeSelector"`
}

// PlacementStrategy represents the complete placement strategy for a workload
type PlacementStrategy struct {
	Base  int             `json:"base"`
	Rules []PlacementRule `json:"rules"`
}

// ParsePlacementStrategy parses the custom scheduling annotation into a structured strategy
// Format: "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot"
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
// Format: "base=1,weight=1,nodeSelector=node-type:ondemand"
func parseFirstRule(part string, strategy *PlacementStrategy) error {
	// Split by comma to get individual parameters
	params := strings.Split(part, ",")

	rule := PlacementRule{
		NodeSelector: make(map[string]string),
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
		}
	}

	strategy.Rules = append(strategy.Rules, rule)
	return nil
}

// parseRule parses a subsequent rule
// Format: "weight=2,nodeSelector=node-type:spot"
func parseRule(part string) (*PlacementRule, error) {
	rule := &PlacementRule{
		NodeSelector: make(map[string]string),
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
		}
	}

	return rule, nil
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
	if len(rule.NodeSelector) == 0 {
		return fmt.Errorf("empty nodeSelector in rule")
	}

	// Apply nodeSelector
	if pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = make(map[string]string)
	}

	// Merge nodeSelector
	for key, value := range rule.NodeSelector {
		pod.Spec.NodeSelector[key] = value
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
		ruleKey := nodeSelector2String(rule.NodeSelector)
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
