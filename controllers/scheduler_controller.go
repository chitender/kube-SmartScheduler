package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	startTime := time.Now()
	log := log.FromContext(ctx).WithValues("reconcileID", generateReconcileID())

	// Add detailed reconciliation logging
	log.Info("=== SCHEDULER RECONCILE START ===",
		"requestedName", req.Name,
		"requestedNamespace", req.Namespace,
		"timestamp", startTime.Format(time.RFC3339))

	defer func() {
		duration := time.Since(startTime)
		log.Info("=== SCHEDULER RECONCILE END ===",
			"duration", duration.String(),
			"durationMs", duration.Milliseconds())
	}()

	// Fetch the Deployment instance
	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		log.Info("Deployment not found or error fetching", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling deployment",
		"deployment", deployment.Name,
		"namespace", deployment.Namespace,
		"uid", deployment.UID,
		"generation", deployment.Generation,
		"observedGeneration", deployment.Status.ObservedGeneration,
		"replicas", deployment.Spec.Replicas,
		"availableReplicas", deployment.Status.AvailableReplicas,
		"readyReplicas", deployment.Status.ReadyReplicas,
		"resourceVersion", deployment.ResourceVersion)

	// Check if deployment has custom scheduling annotations
	annotations := deployment.Annotations
	if annotations == nil {
		log.Info("No annotations found on deployment, skipping")
		// No custom scheduling, nothing to do
		return ctrl.Result{}, nil
	}

	log.Info("Deployment annotations",
		"annotationCount", len(annotations),
		"allAnnotations", annotations)

	scheduleStrategy, exists := annotations["smart-scheduler.io/schedule-strategy"]
	if !exists {
		log.Info("No smart-scheduler.io/schedule-strategy annotation found, skipping")
		// No custom scheduling strategy, nothing to do
		return ctrl.Result{}, nil
	}

	log.Info("Found scheduling strategy", "strategy", scheduleStrategy, "deployment", deployment.Name)

	// TODO: Parse and validate the scheduling strategy
	// TODO: Update deployment status or create configuration for webhook
	// For now, just log that we found a strategy

	log.Info("Strategy processing complete, no further action needed")
	return ctrl.Result{}, nil
}

// generateReconcileID creates a unique ID for each reconciliation
func generateReconcileID() string {
	return time.Now().Format("20060102150405.000000")
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchedulerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		WithOptions(controller.Options{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				log := r.Log.WithValues("eventType", "CREATE", "deploymentName", e.Object.GetName())
				log.Info("Deployment CREATE event", "namespace", e.Object.GetNamespace())
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldDep := e.ObjectOld.(*appsv1.Deployment)
				newDep := e.ObjectNew.(*appsv1.Deployment)

				log := r.Log.WithValues("eventType", "UPDATE", "deploymentName", newDep.Name)

				// Check if scheduling-related annotations changed
				oldAnnotations := oldDep.Annotations
				newAnnotations := newDep.Annotations

				oldStrategy := ""
				newStrategy := ""
				if oldAnnotations != nil {
					oldStrategy = oldAnnotations["smart-scheduler.io/schedule-strategy"]
				}
				if newAnnotations != nil {
					newStrategy = newAnnotations["smart-scheduler.io/schedule-strategy"]
				}

				strategyChanged := oldStrategy != newStrategy
				generationChanged := oldDep.Generation != newDep.Generation
				replicasChanged := oldDep.Spec.Replicas != newDep.Spec.Replicas

				log.Info("Deployment UPDATE event evaluation",
					"namespace", newDep.Namespace,
					"strategyChanged", strategyChanged,
					"generationChanged", generationChanged,
					"replicasChanged", replicasChanged,
					"oldStrategy", oldStrategy,
					"newStrategy", newStrategy,
					"oldGeneration", oldDep.Generation,
					"newGeneration", newDep.Generation,
					"oldReplicas", oldDep.Spec.Replicas,
					"newReplicas", newDep.Spec.Replicas,
					"shouldReconcile", strategyChanged || generationChanged || replicasChanged)

				return strategyChanged || generationChanged || replicasChanged
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				log := r.Log.WithValues("eventType", "DELETE", "deploymentName", e.Object.GetName())
				log.Info("Deployment DELETE event", "namespace", e.Object.GetNamespace())
				return true
			},
		}).
		// Remove the Owns() directive that was causing unnecessary reconciliations
		Complete(r)
}
