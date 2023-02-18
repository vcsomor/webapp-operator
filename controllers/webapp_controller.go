/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"time"

	webappv1alpha1 "github.com/vcsomor/webapp-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// WebappReconciler reconciles a Webapp object
type WebappReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Webapp object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *WebappReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	webapp := &webappv1alpha1.Webapp{}
	err := r.Get(ctx, req.NamespacedName, webapp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("webbapp resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get memcached")
		return ctrl.Result{}, err
	}

	// if webapp marked to be deleted
	if webapp.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: webapp.Name, Namespace: webapp.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		deployment, err := r.deploymentForWebapp(webapp)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for Webapp")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check deployment replicas
	dplyCheck := checkDeploymentUpdate(&webapp.Spec, &found.Spec)
	if dplyCheck.isUpdateRequired() {
		// if replicas are different
		if dplyCheck.replicasDifferent {
			replicas := webapp.Spec.Replicas
			found.Spec.Replicas = &replicas
			if err = r.Update(ctx, found); err != nil {
				log.Error(err, "Failed to update Deployment",
					"Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
				return ctrl.Result{}, err
			}
			// After the update requeue to check other differences
			return ctrl.Result{Requeue: true}, nil
		}

		// if container port is different
		if dplyCheck.portDifferent {
			cPort := webapp.Spec.ContainerPort
			found.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = cPort
			if err = r.Update(ctx, found); err != nil {
				log.Error(err, "Failed to update Deployment",
					"Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
				return ctrl.Result{}, err
			}
			// After the update requeue to check other differences
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *WebappReconciler) deploymentForWebapp(webapp *webappv1alpha1.Webapp) (*appsv1.Deployment, error) {
	name := webapp.Name

	image := webapp.Spec.Image
	replicas := webapp.Spec.Replicas
	port := webapp.Spec.ContainerPort

	labels := map[string]string{
		"app":                          name,
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/name":       "Webapp",
		"app.kubernetes.io/part-of":    "webapp-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webapp.Name,
			Namespace: webapp.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "webapp",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: port,
						}},
					}},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(webapp, dep, r.Scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebappReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1alpha1.Webapp{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

type deploymentCheckResult struct {
	replicasDifferent bool
	portDifferent     bool
}

func (r *deploymentCheckResult) isUpdateRequired() bool {
	return r.portDifferent || r.replicasDifferent
}

func checkDeploymentUpdate(spec *webappv1alpha1.WebappSpec, found *appsv1.DeploymentSpec) *deploymentCheckResult {
	replicasDifferent := spec.Replicas != *found.Replicas
	portDifferent := true

	containers := found.Template.Spec.Containers
	if len(containers) != 1 {
		ports := containers[0].Ports
		if len(ports) != 1 {
			if ports[0].ContainerPort == spec.ContainerPort {
				portDifferent = false
			}
		}
	}

	return &deploymentCheckResult{
		replicasDifferent: replicasDifferent,
		portDifferent:     portDifferent,
	}
}
