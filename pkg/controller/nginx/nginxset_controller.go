/*
Copyright 2025.

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

package nginx

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	kscontroller "kubesphere.io/kubesphere/pkg/controller"
	"kubesphere.io/kubesphere/pkg/controller/nginx/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NginxSetReconciler reconciles a NginxSet object
type NginxSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.dove521.cn,resources=nginxsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.dove521.cn,resources=nginxsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.dove521.cn,resources=nginxsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NginxSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *NginxSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	nginxSet := &api.NginxSet{}
	err := r.Get(ctx, req.NamespacedName, nginxSet)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if nginxSet.Spec.Replicas != 1 {
		nginxSet.Spec.Replicas = 1
		err = r.Update(ctx, nginxSet)
		if err != nil {
			klog.Error(err, "Failed to update NginxSet replicas", "NginxSet.Namespace", nginxSet.Namespace, "NginxSet.Name", nginxSet.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Namespace: nginxSet.Namespace, Name: nginxSet.Name}, deployment)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		deployment = r.deploymentForNginxSet(nginxSet)
		klog.Info("Creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.Create(ctx, deployment)
		if err != nil {
			klog.Error(err, "Failed to create new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		klog.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := nginxSet.Spec.Replicas
	if *deployment.Spec.Replicas != size {
		klog.Info("Updating Deployment size", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name, "Deployment.Size", size)
		deployment.Spec.Replicas = &size
		err = r.Update(ctx, deployment)
		if err != nil {
			klog.Error(err, "Failed to update Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *NginxSetReconciler) deploymentForNginxSet(m *api.NginxSet) *appsv1.Deployment {
	ls := labelsForNginxSet(m.Name)
	replicas := m.Spec.Replicas

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "nginx:latest",
						Name:  "nginx",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
						}},
					}},
				},
			},
		},
	}
	// Set NginxSet instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForNginxSet returns the labels for selecting the resources
// belonging to the given nginxset CR name.
func labelsForNginxSet(name string) map[string]string {
	return map[string]string{"app": "nginx", "nginxset_cr": name}
}

func (r *NginxSetReconciler) Enabled(clusterRole string) bool {
	return true
}

func (r *NginxSetReconciler) Name() string {
	return "nginx_set"
}

// SetupWithManager sets up the controller with the Manager.
func (r *NginxSetReconciler) SetupWithManager(mgr *kscontroller.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.NginxSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
