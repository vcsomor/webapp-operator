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
	webappv1alpha1 "github.com/vcsomor/webapp-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// WebappReconciler reconciles a Webapp object
type WebappReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const (
	certManagerClusterIssuerAnnotation = "cert-manager.io/cluster-issuer"
)

//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.viq.io,resources=webapps/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
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
	logger := log.FromContext(ctx)

	webapp := &webappv1alpha1.Webapp{}
	err := r.Get(ctx, req.NamespacedName, webapp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("webapp resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get webapp")
		return ctrl.Result{}, err
	}

	// if webapp marked to be deleted
	if webapp.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, err
	}

	namespacedName := types.NamespacedName{Name: webapp.Name, Namespace: webapp.Namespace}
	// Check if the deployment already exists, if not create a new one
	dplyFound := &appsv1.Deployment{}
	err = r.Get(ctx, namespacedName, dplyFound)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		deployment, err := r.deploymentForWebapp(webapp)
		if err != nil {
			logger.Error(err, "Failed to define new Deployment resource for Webapp")
			return ctrl.Result{}, err
		}

		logger.Info("Creating a new Deployment",
			"Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			logger.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}

		logger.Info("Deployment created",
			"Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check deployment differences
	dplyCheck := checkDeploymentDiff(&webapp.Spec, &dplyFound.Spec)
	if dplyCheck.hasDiff() {
		logger.Info("Deployment reconciliation required",
			"Deployment.Namespace", dplyFound.Namespace, "Deployment.Name", dplyFound.Name)

		// if replicas are different
		if dplyCheck.replicasDiff {
			logger.Info("Changing replica count",
				"Deployment.Namespace", dplyFound.Namespace, "Deployment.Name", dplyFound.Name)
			replicas := webapp.Spec.Replicas
			dplyFound.Spec.Replicas = &replicas
			if err = r.Update(ctx, dplyFound); err != nil {
				logger.Error(err, "Failed to update Deployment (replicas)",
					"Deployment.Namespace", dplyFound.Namespace, "Deployment.Name", dplyFound.Name)
				return ctrl.Result{}, err
			}

			// After the update requeue to check other differences
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// if container port is different
		if dplyCheck.portDiff {
			logger.Info("Changing container port",
				"Deployment.Namespace", dplyFound.Namespace, "Deployment.Name", dplyFound.Name)
			cPort := webapp.Spec.ContainerPort
			dplyFound.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = cPort
			if err = r.Update(ctx, dplyFound); err != nil {
				logger.Error(err, "Failed to update Deployment (port)",
					"Deployment.Namespace", dplyFound.Namespace, "Deployment.Name", dplyFound.Name)
				return ctrl.Result{}, err
			}
			// After the update requeue to check other differences
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// Check if the service already exists, if not create a new one
	srvFound := &corev1.Service{}
	err = r.Get(ctx, namespacedName, srvFound)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new service
		srv, err := r.serviceForWebapp(webapp)
		if err != nil {
			logger.Error(err, "Failed to define new Service resource for Webapp")
			return ctrl.Result{}, err
		}

		logger.Info("Creating a new Service",
			"Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
		if err = r.Create(ctx, srv); err != nil {
			logger.Error(err, "Failed to create new Service",
				"Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
			return ctrl.Result{}, err
		}

		logger.Info("Service created",
			"Service.Namespace", srv.Namespace, "Service.Name", srv.Name)
		// Service created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check if the service already exists, if not create a new one
	ingFound := &networkingv1.Ingress{}
	err = r.Get(ctx, namespacedName, ingFound)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new Ingress
		ingr, err := r.ingressForWebapp(webapp, srvFound.Name)
		if err != nil {
			logger.Error(err, "Failed to define new Ingress resource for Webapp")
			return ctrl.Result{}, err
		}

		logger.Info("Creating a new Ingress",
			"Ingress.Namespace", ingr.Namespace, "Ingress.Name", ingr.Name)
		if err = r.Create(ctx, ingr); err != nil {
			logger.Error(err, "Failed to create new Ingress",
				"Ingress.Namespace", ingr.Namespace, "Ingress.Name", ingr.Name)
			return ctrl.Result{}, err
		}

		logger.Info("Ingress created",
			"Ingress.Namespace", ingr.Namespace, "Ingress.Name", ingr.Name)
		// Ingress created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Ingress")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Check deployment differences
	ingrCheck := checkIngressDiff(&webapp.Spec, ingFound)
	if ingrCheck.hasDiff() {
		logger.Info("Ingress reconciliation required",
			"Ingress.Namespace", ingFound.Namespace, "Ingress.Name", ingFound.Name)

		// if host is different
		if ingrCheck.hostDiff {
			logger.Info("Changing Host for ingress",
				"Ingress.Namespace", ingFound.Namespace, "Ingress.Name", ingFound.Name)
			host := webapp.Spec.Host
			spec := &ingFound.Spec
			spec.TLS = tlsForIngress(ingFound.Name, host)
			spec.Rules = rulesForIngress(srvFound.Name, host)
			if err = r.Update(ctx, ingFound); err != nil {
				logger.Error(err, "Failed to update Ingress (host)",
					"Ingress.Namespace", ingFound.Namespace, "Ingress.Name", ingFound.Name)
				return ctrl.Result{}, err
			}

			// After the update requeue to check other differences
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		// if cert-manager is different
		if ingrCheck.certManagerDiff {
			logger.Info("Changing Cert-manager for ingress",
				"Ingress.Namespace", ingFound.Namespace, "Ingress.Name", ingFound.Name)
			certManager := webapp.Spec.CertManager
			meta := &ingFound.ObjectMeta
			meta.Annotations[certManagerClusterIssuerAnnotation] = certManager

			if err = r.Update(ctx, ingFound); err != nil {
				logger.Error(err, "Failed to update Ingress (cert-manager)",
					"Ingress.Namespace", ingFound.Namespace, "Ingress.Name", ingFound.Name)
				return ctrl.Result{}, err
			}

			// After the update requeue to check other differences
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *WebappReconciler) deploymentForWebapp(webapp *webappv1alpha1.Webapp) (*appsv1.Deployment, error) {
	name := webapp.Name
	image := webapp.Spec.Image
	replicas := webapp.Spec.Replicas
	port := webapp.Spec.ContainerPort

	labels := labelsForWebapp(name)

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
	if err := ctrl.SetControllerReference(webapp, dep, r.Scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

func (r *WebappReconciler) serviceForWebapp(webapp *webappv1alpha1.Webapp) (*corev1.Service, error) {
	name := webapp.Name

	labels := labelsForWebapp(name)

	srv := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webapp.Name,
			Namespace: webapp.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
						StrVal: "80",
					},
				},
			},
		},
	}

	// Set the ownerRef for the Service
	if err := ctrl.SetControllerReference(webapp, srv, r.Scheme); err != nil {
		return nil, err
	}
	return srv, nil
}

func (r *WebappReconciler) ingressForWebapp(webapp *webappv1alpha1.Webapp, serviceName string) (*networkingv1.Ingress, error) {
	name := webapp.Name

	labels := labelsForWebapp(name)
	host := webapp.Spec.Host

	ingr := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webapp.Name,
			Namespace: webapp.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
				certManagerClusterIssuerAnnotation:           webapp.Spec.CertManager,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &[]string{"nginx"}[0],
			TLS:              tlsForIngress(name, host),
			Rules:            rulesForIngress(serviceName, host),
		},
	}

	// Set the ownerRef for the Ingress
	if err := ctrl.SetControllerReference(webapp, ingr, r.Scheme); err != nil {
		return nil, err
	}
	return ingr, nil
}

func tlsForIngress(name, host string) []networkingv1.IngressTLS {
	return []networkingv1.IngressTLS{
		{
			Hosts: []string{
				host,
			},
			SecretName: name + "-tls-secret",
		},
	}
}

func rulesForIngress(serviceName, host string) []networkingv1.IngressRule {
	return []networkingv1.IngressRule{
		{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &[]networkingv1.PathType{networkingv1.PathTypePrefix}[0],
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{
										Number: 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func labelsForWebapp(name string) map[string]string {
	return map[string]string{
		"app":                          name,
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/name":       "Webapp",
		"app.kubernetes.io/part-of":    "webapp-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebappReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1alpha1.Webapp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

type dplyCheckResult struct {
	replicasDiff bool
	portDiff     bool
}

func (r *dplyCheckResult) hasDiff() bool {
	return r.portDiff || r.replicasDiff
}

func checkDeploymentDiff(spec *webappv1alpha1.WebappSpec, found *appsv1.DeploymentSpec) *dplyCheckResult {
	replicasDiff := spec.Replicas != *found.Replicas
	portDiff := false

	containers := found.Template.Spec.Containers
	if len(containers) > 0 {
		ports := containers[0].Ports
		if len(ports) > 0 {
			if ports[0].ContainerPort != spec.ContainerPort {
				portDiff = true
			}
		}
	}

	return &dplyCheckResult{
		replicasDiff: replicasDiff,
		portDiff:     portDiff,
	}
}

type ingressCheckResult struct {
	hostDiff        bool
	certManagerDiff bool
}

func (r *ingressCheckResult) hasDiff() bool {
	return r.hostDiff || r.certManagerDiff
}

func checkIngressDiff(spec *webappv1alpha1.WebappSpec, found *networkingv1.Ingress) *ingressCheckResult {
	hostDiff := false
	certManagerDiff := false

	// desired hostname
	foundSpec := found.Spec
	host := spec.Host

	// check host in TLS
	if len(foundSpec.TLS) > 0 {
		h := foundSpec.TLS[0].Hosts
		if len(h) > 0 {
			if h[0] != host {
				hostDiff = true
			}
		}
	}

	// check host in rules
	if len(foundSpec.Rules) > 0 {
		if foundSpec.Rules[0].Host != host {
			hostDiff = true
		}
	}

	// desired cert-manager
	foundAnnotations := found.ObjectMeta.Annotations
	cm, f := foundAnnotations[certManagerClusterIssuerAnnotation]
	if !f || (cm != spec.CertManager) {
		certManagerDiff = true
	}

	return &ingressCheckResult{
		hostDiff:        hostDiff,
		certManagerDiff: certManagerDiff,
	}
}
