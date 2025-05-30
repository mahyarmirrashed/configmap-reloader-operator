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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ConfigMapReloader Controller", func() {
	const (
		configMapName      = "test-configmap"
		otherConfigMapName = "other" + configMapName
		deploymentName     = "test-deployment"
		namespaceName      = "default"

		newValue = "new-value"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When a configmap is updated", func() {
		AfterEach(func() {
			ctx := context.Background()

			// Delete ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				},
			}
			_ = k8sClient.Delete(ctx, configMap)

			// Delete other ConfigMap
			otherConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherConfigMapName,
					Namespace: namespaceName,
				},
			}
			_ = k8sClient.Delete(ctx, otherConfigMap)

			// Delete Deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespaceName,
				},
			}
			_ = k8sClient.Delete(ctx, deployment)
		})

		It("should update a deployment with a reloader annotation", func() {
			ctx := context.Background()

			// Create configmap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Create deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespaceName,
					Annotations: map[string]string{
						"reloader.experiments.k8s.mahyarmirrashed.com/enabled": "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": deploymentName},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": deploymentName},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
									EnvFrom: []corev1.EnvFromSource{
										{
											ConfigMapRef: &corev1.ConfigMapEnvSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapName,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			// Simulate configmap update
			configMapKey := types.NamespacedName{Name: configMapName, Namespace: namespaceName}
			updatedConfigMap := &corev1.ConfigMap{}
			// grab current configmap
			Expect(k8sClient.Get(ctx, configMapKey, updatedConfigMap)).To(Succeed())
			// update configmap
			updatedConfigMap.Data["key"] = newValue
			Expect(k8sClient.Update(ctx, updatedConfigMap)).To(Succeed())

			// Reconcile configmap
			reconciler := &ConfigMapReloaderReconciler{Client: k8sClient, Scheme: scheme.Scheme}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: configMapKey})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was updated
			updatedDeployment := &appsv1.Deployment{}
			// blocking check to ensure deployment is updated (with timeout)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespaceName}, updatedDeployment); err != nil {
					return false
				}
				// Get updated deployment annotations
				annotations := updatedDeployment.Spec.Template.Annotations
				// Ensure that deployment has annotation indicating restart
				return annotations != nil && annotations["reloader.experiments.k8s.mahyarmirrashed.com/restarted-at"] != ""
			}, timeout, interval).Should(BeTrue())
		})

		It("should not update a deployment without a reloader annotation", func() {
			ctx := context.Background()

			// Create configmap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Create deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespaceName,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": deploymentName},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": deploymentName},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
									EnvFrom: []corev1.EnvFromSource{
										{
											ConfigMapRef: &corev1.ConfigMapEnvSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapName,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			// Simulate configmap update
			configMapKey := types.NamespacedName{Name: configMapName, Namespace: namespaceName}
			updatedConfigMap := &corev1.ConfigMap{}
			// grab current configmap
			Expect(k8sClient.Get(ctx, configMapKey, updatedConfigMap)).To(Succeed())
			// update configmap
			updatedConfigMap.Data["key"] = newValue
			Expect(k8sClient.Update(ctx, updatedConfigMap)).To(Succeed())

			// Reconcile configmap
			reconciler := &ConfigMapReloaderReconciler{Client: k8sClient, Scheme: scheme.Scheme}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: configMapKey})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was updated
			updatedDeployment := &appsv1.Deployment{}
			// blocking consistent check to ensure deployment is never updated (with timeout)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespaceName}, updatedDeployment); err != nil {
					return false
				}
				// Get updated deployment annotations
				annotations := updatedDeployment.Spec.Template.Annotations
				// Ensure that deployment has annotation indicating restart
				return annotations == nil || annotations["reloader.experiments.k8s.mahyarmirrashed.com/restarted-at"] == ""
			}, timeout, interval).Should(BeTrue())
		})

		It("should not update a deployment with a reloader annotation referencing a different configmap", func() {
			ctx := context.Background()

			// Create configmap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespaceName,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			otherConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      otherConfigMapName,
					Namespace: namespaceName,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, otherConfigMap)).To(Succeed())

			// Create deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespaceName,
					Annotations: map[string]string{
						"reloader.experiments.k8s.mahyarmirrashed.com/enabled": "true",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": deploymentName},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": deploymentName},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "app",
									Image: "nginx",
									EnvFrom: []corev1.EnvFromSource{
										{
											ConfigMapRef: &corev1.ConfigMapEnvSource{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: configMapName,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			// Simulate configmap update
			configMapKey := types.NamespacedName{Name: otherConfigMapName, Namespace: namespaceName}
			updatedConfigMap := &corev1.ConfigMap{}
			// grab current configmap
			Expect(k8sClient.Get(ctx, configMapKey, updatedConfigMap)).To(Succeed())
			// update configmap
			updatedConfigMap.Data["key"] = newValue
			Expect(k8sClient.Update(ctx, updatedConfigMap)).To(Succeed())

			// Reconcile configmap
			reconciler := &ConfigMapReloaderReconciler{Client: k8sClient, Scheme: scheme.Scheme}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: configMapKey})
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment was updated
			updatedDeployment := &appsv1.Deployment{}
			// blocking consistent check to ensure deployment is never updated (with timeout)
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespaceName}, updatedDeployment); err != nil {
					return false
				}
				// Get updated deployment annotations
				annotations := updatedDeployment.Spec.Template.Annotations
				// Ensure that deployment has annotation indicating restart
				return annotations == nil || annotations["reloader.experiments.k8s.mahyarmirrashed.com/restarted-at"] == ""
			}, timeout, interval).Should(BeTrue())
		})
	})
})
