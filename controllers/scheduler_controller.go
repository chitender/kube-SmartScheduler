package controllers

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SchedulerController reconciles Deployment objects with custom scheduling annotations
type SchedulerController struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile handles Deployment changes and updates pod placement strategies
func (r *SchedulerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Deployment instance
	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling deployment", "deployment", deployment.Name, "namespace", deployment.Namespace)

	// Check if deployment has custom scheduling annotations
	annotations := deployment.Annotations
	if annotations == nil {
		// No custom scheduling, nothing to do
		return ctrl.Result{}, nil
	}

	scheduleStrategy, exists := annotations["smart-scheduler.io/schedule-strategy"]
	if !exists {
		// No custom scheduling strategy, nothing to do
		return ctrl.Result{}, nil
	}

	log.Info("Found scheduling strategy", "strategy", scheduleStrategy, "deployment", deployment.Name)

	// TODO: Parse and validate the scheduling strategy
	// TODO: Update deployment status or create configuration for webhook
	// For now, just log that we found a strategy

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
