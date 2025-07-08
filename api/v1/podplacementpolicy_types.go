package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PodPlacementPolicySpec defines the desired state of PodPlacementPolicy
type PodPlacementPolicySpec struct {
	// Selector defines which deployments this policy applies to
	Selector *metav1.LabelSelector `json:"selector"`

	// Strategy defines the placement strategy
	Strategy PlacementStrategySpec `json:"strategy"`

	// Enabled controls whether this policy is active
	Enabled bool `json:"enabled,omitempty"`

	// Priority defines precedence when multiple policies match (higher = more priority)
	Priority int32 `json:"priority,omitempty"`
}

// PlacementStrategySpec defines the placement strategy
type PlacementStrategySpec struct {
	// Base defines minimum pods that should be placed on the first rule
	Base int `json:"base"`

	// Rules defines the placement rules with weights and constraints
	Rules []PlacementRuleSpec `json:"rules"`

	// RebalancePolicy controls how and when rebalancing occurs
	RebalancePolicy *RebalancePolicySpec `json:"rebalancePolicy,omitempty"`
}

// PlacementRuleSpec defines a single placement rule
type PlacementRuleSpec struct {
	// Weight for weighted distribution beyond base count
	Weight int `json:"weight"`

	// NodeSelector constraints for pod placement
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity rules for pod placement
	Affinity []AffinityRuleSpec `json:"affinity,omitempty"`

	// Name provides a human-readable identifier for this rule
	Name string `json:"name,omitempty"`

	// Description explains the purpose of this rule
	Description string `json:"description,omitempty"`
}

// AffinityRuleSpec defines pod affinity or anti-affinity constraints
type AffinityRuleSpec struct {
	// Type specifies "affinity" or "anti-affinity"
	Type string `json:"type"`

	// LabelSelector for pod selection
	LabelSelector map[string]string `json:"labelSelector"`

	// TopologyKey for the affinity constraint
	TopologyKey string `json:"topologyKey"`

	// RequiredDuringScheduling makes this constraint hard vs soft
	RequiredDuringScheduling bool `json:"requiredDuringScheduling,omitempty"`

	// Weight for preferred constraints (1-100)
	Weight int32 `json:"weight,omitempty"`
}

// RebalancePolicySpec controls rebalancing behavior
type RebalancePolicySpec struct {
	// Enabled controls whether automatic rebalancing is active
	Enabled bool `json:"enabled,omitempty"`

	// DriftThreshold is the percentage drift that triggers rebalancing (default: 20%)
	DriftThreshold float64 `json:"driftThreshold,omitempty"`

	// CheckInterval defines how often to check for drift (default: 10m)
	CheckInterval metav1.Duration `json:"checkInterval,omitempty"`

	// MaxPodsPerRebalance limits disruption (default: 1)
	MaxPodsPerRebalance int32 `json:"maxPodsPerRebalance,omitempty"`

	// RebalanceWindow defines when rebalancing is allowed
	RebalanceWindow *TimeWindowSpec `json:"rebalanceWindow,omitempty"`
}

// TimeWindowSpec defines a time window for operations
type TimeWindowSpec struct {
	// StartTime in format "15:04" (24h format)
	StartTime string `json:"startTime"`

	// EndTime in format "15:04" (24h format)
	EndTime string `json:"endTime"`

	// Days of week when window is active (Mon, Tue, Wed, Thu, Fri, Sat, Sun)
	Days []string `json:"days,omitempty"`

	// Timezone for the time window (default: UTC)
	Timezone string `json:"timezone,omitempty"`
}

// PodPlacementPolicyStatus defines the observed state of PodPlacementPolicy
type PodPlacementPolicyStatus struct {
	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// MatchedDeployments lists deployments currently using this policy
	MatchedDeployments []DeploymentReference `json:"matchedDeployments,omitempty"`

	// Statistics about policy usage
	Statistics *PolicyStatistics `json:"statistics,omitempty"`

	// LastRebalance tracks the most recent rebalancing action
	LastRebalance *metav1.Time `json:"lastRebalance,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// DeploymentReference identifies a deployment using this policy
type DeploymentReference struct {
	// Name of the deployment
	Name string `json:"name"`

	// Namespace of the deployment
	Namespace string `json:"namespace"`

	// CurrentDrift percentage of the deployment's actual vs expected placement
	CurrentDrift float64 `json:"currentDrift,omitempty"`

	// LastApplied when the policy was last applied to this deployment
	LastApplied *metav1.Time `json:"lastApplied,omitempty"`
}

// PolicyStatistics provides metrics about policy effectiveness
type PolicyStatistics struct {
	// TotalPodsManaged across all deployments using this policy
	TotalPodsManaged int32 `json:"totalPodsManaged,omitempty"`

	// AverageDrift across all managed deployments
	AverageDrift float64 `json:"averageDrift,omitempty"`

	// RebalanceCount total number of rebalancing actions performed
	RebalanceCount int32 `json:"rebalanceCount,omitempty"`

	// LastUpdated when these statistics were calculated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Enabled",type="boolean",JSONPath=".spec.enabled"
//+kubebuilder:printcolumn:name="Priority",type="integer",JSONPath=".spec.priority"
//+kubebuilder:printcolumn:name="Matched Deployments",type="integer",JSONPath=".status.statistics.totalPodsManaged"
//+kubebuilder:printcolumn:name="Avg Drift",type="string",JSONPath=".status.statistics.averageDrift"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PodPlacementPolicy is the Schema for the podplacementpolicies API
type PodPlacementPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodPlacementPolicySpec   `json:"spec,omitempty"`
	Status PodPlacementPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PodPlacementPolicyList contains a list of PodPlacementPolicy
type PodPlacementPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodPlacementPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodPlacementPolicy{}, &PodPlacementPolicyList{})
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodPlacementPolicy) DeepCopyInto(out *PodPlacementPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodPlacementPolicy.
func (in *PodPlacementPolicy) DeepCopy() *PodPlacementPolicy {
	if in == nil {
		return nil
	}
	out := new(PodPlacementPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodPlacementPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodPlacementPolicyList) DeepCopyInto(out *PodPlacementPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PodPlacementPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodPlacementPolicyList.
func (in *PodPlacementPolicyList) DeepCopy() *PodPlacementPolicyList {
	if in == nil {
		return nil
	}
	out := new(PodPlacementPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodPlacementPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodPlacementPolicySpec) DeepCopyInto(out *PodPlacementPolicySpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	in.Strategy.DeepCopyInto(&out.Strategy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodPlacementPolicySpec.
func (in *PodPlacementPolicySpec) DeepCopy() *PodPlacementPolicySpec {
	if in == nil {
		return nil
	}
	out := new(PodPlacementPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodPlacementPolicyStatus) DeepCopyInto(out *PodPlacementPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.MatchedDeployments != nil {
		in, out := &in.MatchedDeployments, &out.MatchedDeployments
		*out = make([]DeploymentReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Statistics != nil {
		in, out := &in.Statistics, &out.Statistics
		*out = new(PolicyStatistics)
		(*in).DeepCopyInto(*out)
	}
	if in.LastRebalance != nil {
		in, out := &in.LastRebalance, &out.LastRebalance
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodPlacementPolicyStatus.
func (in *PodPlacementPolicyStatus) DeepCopy() *PodPlacementPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(PodPlacementPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PlacementStrategySpec) DeepCopyInto(out *PlacementStrategySpec) {
	*out = *in
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]PlacementRuleSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.RebalancePolicy != nil {
		in, out := &in.RebalancePolicy, &out.RebalancePolicy
		*out = new(RebalancePolicySpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PlacementStrategySpec.
func (in *PlacementStrategySpec) DeepCopy() *PlacementStrategySpec {
	if in == nil {
		return nil
	}
	out := new(PlacementStrategySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PlacementRuleSpec) DeepCopyInto(out *PlacementRuleSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = make([]AffinityRuleSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PlacementRuleSpec.
func (in *PlacementRuleSpec) DeepCopy() *PlacementRuleSpec {
	if in == nil {
		return nil
	}
	out := new(PlacementRuleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AffinityRuleSpec) DeepCopyInto(out *AffinityRuleSpec) {
	*out = *in
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AffinityRuleSpec.
func (in *AffinityRuleSpec) DeepCopy() *AffinityRuleSpec {
	if in == nil {
		return nil
	}
	out := new(AffinityRuleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RebalancePolicySpec) DeepCopyInto(out *RebalancePolicySpec) {
	*out = *in
	out.CheckInterval = in.CheckInterval
	if in.RebalanceWindow != nil {
		in, out := &in.RebalanceWindow, &out.RebalanceWindow
		*out = new(TimeWindowSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RebalancePolicySpec.
func (in *RebalancePolicySpec) DeepCopy() *RebalancePolicySpec {
	if in == nil {
		return nil
	}
	out := new(RebalancePolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeWindowSpec) DeepCopyInto(out *TimeWindowSpec) {
	*out = *in
	if in.Days != nil {
		in, out := &in.Days, &out.Days
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeWindowSpec.
func (in *TimeWindowSpec) DeepCopy() *TimeWindowSpec {
	if in == nil {
		return nil
	}
	out := new(TimeWindowSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentReference) DeepCopyInto(out *DeploymentReference) {
	*out = *in
	if in.LastApplied != nil {
		in, out := &in.LastApplied, &out.LastApplied
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentReference.
func (in *DeploymentReference) DeepCopy() *DeploymentReference {
	if in == nil {
		return nil
	}
	out := new(DeploymentReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyStatistics) DeepCopyInto(out *PolicyStatistics) {
	*out = *in
	if in.LastUpdated != nil {
		in, out := &in.LastUpdated, &out.LastUpdated
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyStatistics.
func (in *PolicyStatistics) DeepCopy() *PolicyStatistics {
	if in == nil {
		return nil
	}
	out := new(PolicyStatistics)
	in.DeepCopyInto(out)
	return out
}
