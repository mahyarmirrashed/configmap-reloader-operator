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

package controller

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigMapReloaderReconciler reconciles a ConfigMapReloader object
type ConfigMapReloaderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigMapReloaderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var configMap corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &configMap); err != nil {
		if errors.IsNotFound(err) {
			// NOTE: configmap deleted, nothing to do
			return ctrl.Result{}, nil
		}

		logger.Error(err, "failed to get configmap")
		return ctrl.Result{}, err
	}

	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(req.Namespace)); err != nil {
		logger.Error(err, "failed to list deployments")
		return ctrl.Result{}, err
	}

	for _, deployment := range deployments.Items {
		// check if reloader is enabled
		enabled, ok := deployment.Annotations["reloader.experiments.k8s.mahyarmirrashed.com/enabled"]
		if !ok || enabled != "true" {
			continue
		}
		// ensure that deployment has container referencing configmap
		if !deploymentReferencesConfigMap(deployment, configMap.Name) {
			continue
		}

		// trigger rolling restart
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}

		deployment.Spec.Template.Annotations["reloader.experiments.k8s.mahyarmirrashed.com/restarted-at"] = time.Now().Format(time.RFC3339)

		if err := r.Update(ctx, &deployment); err != nil {
			logger.Error(err, "failed to update deployment", "name", deployment.Name)
			return ctrl.Result{}, err
		}

		logger.Info("triggered restart for deployment", "name", deployment.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReloaderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Named("configmapreloader").
		Complete(r)
}

func deploymentReferencesConfigMap(deployment appsv1.Deployment, configMapName string) bool {
	// check containers
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// check `envFrom`
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
				return true
			}
		}
		// check `env`
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == configMapName {
				return true
			}
		}
	}

	// check volumes
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name == configMapName {
			return true
		}
	}

	return false
}
