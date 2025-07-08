package webhook

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	log.Info("Found scheduling strategy", "strategy", scheduleStrategy)

	// TODO: Parse the scheduling strategy and apply mutations
	// For now, just return allowed

	return admission.Allowed("")
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
		WithDefaulter(pm).
		Complete()
}

// Default implements the defaulting webhook interface (required by controller-runtime)
func (pm *PodMutator) Default(ctx context.Context, obj runtime.Object) error {
	// This method is required by the defaulting webhook interface
	// The actual logic is in the Handle method
	return nil
}
