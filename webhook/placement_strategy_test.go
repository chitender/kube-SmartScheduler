package webhook

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseePlacementStrategy(t *testing.T) {
	tests := []struct {
		name          string
		annotation    string
		expectError   bool
		expectedBase  int
		expectedRules int
	}{
		{
			name:          "Valid strategy with two rules",
			annotation:    "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot",
			expectError:   false,
			expectedBase:  1,
			expectedRules: 2,
		},
		{
			name:          "Valid strategy with single rule",
			annotation:    "base=2,weight=1,nodeSelector=zone:us-west-1",
			expectError:   false,
			expectedBase:  2,
			expectedRules: 1,
		},
		{
			name:          "Valid strategy with complex nodeSelector",
			annotation:    "base=1,weight=1,nodeSelector=node-type:ondemand,zone:us-west-1;weight=2,nodeSelector=node-type:spot,zone:us-west-1",
			expectError:   false,
			expectedBase:  1,
			expectedRules: 2,
		},
		{
			name:        "Empty annotation",
			annotation:  "",
			expectError: true,
		},
		{
			name:          "Invalid format - missing base",
			annotation:    "weight=1,nodeSelector=node-type:ondemand",
			expectError:   false, // base defaults to 0
			expectedBase:  0,
			expectedRules: 1,
		},
		{
			name:        "Invalid format - invalid base",
			annotation:  "base=abc,weight=1,nodeSelector=node-type:ondemand",
			expectError: true,
		},
		{
			name:        "Invalid format - invalid weight",
			annotation:  "base=1,weight=abc,nodeSelector=node-type:ondemand",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := ParsePlacementStrategy(tt.annotation)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if strategy.Base != tt.expectedBase {
				t.Errorf("Expected base %d, got %d", tt.expectedBase, strategy.Base)
			}

			if len(strategy.Rules) != tt.expectedRules {
				t.Errorf("Expected %d rules, got %d", tt.expectedRules, len(strategy.Rules))
			}
		})
	}
}

func TestApplyPlacementStrategy(t *testing.T) {
	tests := []struct {
		name                 string
		annotation           string
		currentCounts        map[string]int
		expectedNodeSelector map[string]string
	}{
		{
			name:       "First pod should go to ondemand (base)",
			annotation: "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot",
			currentCounts: map[string]int{
				"node-type=ondemand": 0,
				"node-type=spot":     0,
			},
			expectedNodeSelector: map[string]string{
				"node-type": "ondemand",
			},
		},
		{
			name:       "Second pod should go to spot (weighted)",
			annotation: "base=1,weight=1,nodeSelector=node-type:ondemand;weight=2,nodeSelector=node-type:spot",
			currentCounts: map[string]int{
				"node-type=ondemand": 1,
				"node-type=spot":     0,
			},
			expectedNodeSelector: map[string]string{
				"node-type": "spot",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := ParsePlacementStrategy(tt.annotation)
			if err != nil {
				t.Fatalf("Failed to parse strategy: %v", err)
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{},
			}

			err = ApplyPlacementStrategy(pod, strategy, tt.currentCounts)
			if err != nil {
				t.Fatalf("Failed to apply strategy: %v", err)
			}

			if len(pod.Spec.NodeSelector) != len(tt.expectedNodeSelector) {
				t.Errorf("Expected nodeSelector with %d entries, got %d",
					len(tt.expectedNodeSelector), len(pod.Spec.NodeSelector))
			}

			for key, expectedValue := range tt.expectedNodeSelector {
				if actualValue, exists := pod.Spec.NodeSelector[key]; !exists || actualValue != expectedValue {
					t.Errorf("Expected nodeSelector[%s]=%s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestNodeSelector2String(t *testing.T) {
	tests := []struct {
		name     string
		selector map[string]string
		expected string
	}{
		{
			name:     "Single key-value",
			selector: map[string]string{"node-type": "ondemand"},
			expected: "node-type=ondemand",
		},
		{
			name:     "Empty selector",
			selector: map[string]string{},
			expected: "",
		},
		{
			name:     "Multiple key-values",
			selector: map[string]string{"node-type": "spot", "zone": "us-west-1"},
			// Note: order may vary due to map iteration
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nodeSelector2String(tt.selector)

			if tt.name == "Multiple key-values" {
				// For multiple entries, just check that it contains both
				if !(len(result) > 0 &&
					(result == "node-type=spot,zone=us-west-1" || result == "zone=us-west-1,node-type=spot")) {
					t.Errorf("Expected result to contain both key-value pairs, got %s", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}
