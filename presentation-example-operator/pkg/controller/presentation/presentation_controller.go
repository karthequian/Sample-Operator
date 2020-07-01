package presentation

import (
	"context"
	"fmt"

	presentationv1alpha1 "github.com/NautiluX/presentation-example-operator/pkg/apis/presentation/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_presentation")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Presentation Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePresentation{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("presentation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Presentation
	err = c.Watch(&source.Kind{Type: &presentationv1alpha1.Presentation{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Presentation
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &presentationv1alpha1.Presentation{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePresentation implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePresentation{}

// ReconcilePresentation reconciles a Presentation object
type ReconcilePresentation struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Presentation object and makes changes based on the state read
// and what is in the Presentation.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePresentation) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Presentation")

	// Fetch the Presentation instance
	instance := &presentationv1alpha1.Presentation{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	configMapChanged, err := r.ensureLatestConfigMap(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = r.ensureLatestObject(instance, configMapChanged)
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *presentationv1alpha1.Presentation) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.ResourceName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// newDeploymentForCR returns a busybox pod with the same name/namespace as the cr
func newDeploymentForCR(cr *presentationv1alpha1.Presentation) *appsv1.Deployment {
	var replicas int32 = 1

	labels := map[string]string{
		"app": cr.Name,
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Spec.ResourceName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

func newConfigMap(cr *presentationv1alpha1.Presentation) *corev1.ConfigMap {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-config",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"slides.md": cr.Spec.ResourceName,
		},
	}
}

func (r *ReconcilePresentation) ensureLatestConfigMap(instance *presentationv1alpha1.Presentation) (bool, error) {
	configMap := newConfigMap(instance)

	// Set Presentation instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, configMap, r.scheme); err != nil {
		return false, err
	}

	// Check if this ConfigMap already exists
	foundMap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundMap)
	if err != nil && errors.IsNotFound(err) {
		err = r.client.Create(context.TODO(), configMap)
		if err != nil {
			return false, err
		}
	} else if err != nil {
		return false, err
	}

	if foundMap.Data["slides.md"] != configMap.Data["slides.md"] {
		err = r.client.Update(context.TODO(), configMap)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *ReconcilePresentation) ensureLatestObject(instance *presentationv1alpha1.Presentation, configMapChanged bool) error {

	var err error
	if instance.Spec.ResourceType == "pod" {
		// Define a new Pod object
		pod := newPodForCR(instance)
		// Set Presentation instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
			return err
		}
		// Check if this Pod already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)

		if err == nil {
			log.Info("Pod was found:")
			log.Info(fmt.Sprintf("Pod Info: Name: %v, in Namespace: %v, with status: %v ", found.Name, found.Namespace, found.Status))
		}

	} else if instance.Spec.ResourceType == "deployment" {
		// Define a new Pod object
		deployment := newDeploymentForCR(instance)
		// Set Presentation instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
			return err
		}
		// Check if this Pod already exists
		found := &appsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)

		if err == nil {
			log.Info("Deployment was found:")
			log.Info(fmt.Sprintf("Deployment Info: Name: %v, in Namespace: %v, with replicas: %v, with status: %v ", found.Name, found.Namespace, found.Spec.Replicas, found.Status))
		}

	}
	/*
		if err != nil && errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				return err
			}
			// Pod created successfully - don't requeue
			return nil
		} else if err != nil {
			return err
		}

		if configMapChanged {
			err = r.client.Delete(context.TODO(), found)
			if err != nil {
				return err
			}
		}
	*/
	return nil
}
