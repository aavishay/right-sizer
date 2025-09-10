// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
package controllers

import (
	"context"
	"encoding/json"
	"right-sizer/config"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrlFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResizePolicyWithFeatureFlag(t *testing.T) {
	tests := []struct {
		name               string
		enableFeatureFlag  bool
		expectResizePolicy bool
	}{
		{
			name:               "Feature flag enabled - should apply resize policy",
			enableFeatureFlag:  true,
			expectResizePolicy: true,
		},
		{
			name:               "Feature flag disabled - should not apply resize policy",
			enableFeatureFlag:  false,
			expectResizePolicy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test pod
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			}

			// Create fake client
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			ctrlClient := ctrlFake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pod).
				Build()
			fakeClient := fake.NewSimpleClientset(pod)

			// Create config with feature flag setting
			cfg := config.GetDefaults()
			cfg.UpdateResizePolicy = tt.enableFeatureFlag

			// Test InPlaceRightSizer
			t.Run("InPlaceRightSizer", func(t *testing.T) {
				// Track if patch was called
				patchCalled := false
				var patchedResizePolicy []corev1.ContainerResizePolicy

				// Add reactor to track patch calls
				fakeClient.PrependReactor("patch", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					patchAction := action.(clienttesting.PatchAction)
					if patchAction.GetPatchType() == types.StrategicMergePatchType {
						patchCalled = true

						// Parse the patch to extract resize policy
						var patchData map[string]interface{}
						if err := json.Unmarshal(patchAction.GetPatch(), &patchData); err == nil {
							if spec, ok := patchData["spec"].(map[string]interface{}); ok {
								if containers, ok := spec["containers"].([]interface{}); ok && len(containers) > 0 {
									if container, ok := containers[0].(map[string]interface{}); ok {
										if resizePolicy, ok := container["resizePolicy"].([]interface{}); ok {
											// Found resize policy in patch
											for _, policy := range resizePolicy {
												if policyMap, ok := policy.(map[string]interface{}); ok {
													patchedResizePolicy = append(patchedResizePolicy, corev1.ContainerResizePolicy{
														ResourceName:  corev1.ResourceName(policyMap["resourceName"].(string)),
														RestartPolicy: corev1.ResourceResizeRestartPolicy(policyMap["restartPolicy"].(string)),
													})
												}
											}
										}
									}
								}
							}
						}
					}
					return false, nil, nil
				})

				r := &InPlaceRightSizer{
					Client:    ctrlClient,
					ClientSet: fakeClient,
					Config:    cfg,
				}

				// Apply resize policy
				ctx := context.Background()
				err := r.applyResizePolicy(ctx, pod)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Check if resize policy was applied based on feature flag
				if tt.expectResizePolicy {
					if !patchCalled {
						t.Error("Expected patch to be called but it wasn't")
					}
					if len(patchedResizePolicy) != 2 {
						t.Errorf("Expected 2 resize policies, got %d", len(patchedResizePolicy))
					}
				} else {
					if patchCalled {
						t.Error("Expected patch NOT to be called but it was")
					}
				}
			})

			// Test AdaptiveRightSizer
			t.Run("AdaptiveRightSizer", func(t *testing.T) {
				// Track if patch was called
				patchCalled := false
				var patchedResizePolicy []corev1.ContainerResizePolicy

				// Add reactor to track patch calls
				fakeClient.PrependReactor("patch", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					patchAction := action.(clienttesting.PatchAction)
					if patchAction.GetPatchType() == types.JSONPatchType && patchAction.GetSubresource() == "" {
						patchCalled = true

						// Parse the patch to extract resize policy
						var patchOps []map[string]interface{}
						if err := json.Unmarshal(patchAction.GetPatch(), &patchOps); err == nil {
							for _, op := range patchOps {
								if op["op"] == "add" || op["op"] == "replace" {
									if path, ok := op["path"].(string); ok {
										if path == "/spec/containers/0/resizePolicy" {
											if value, ok := op["value"].([]interface{}); ok {
												for _, policy := range value {
													if policyMap, ok := policy.(map[string]interface{}); ok {
														patchedResizePolicy = append(patchedResizePolicy, corev1.ContainerResizePolicy{
															ResourceName:  corev1.ResourceName(policyMap["resourceName"].(string)),
															RestartPolicy: corev1.ResourceResizeRestartPolicy(policyMap["restartPolicy"].(string)),
														})
													}
												}
											}
										}
									}
								}
							}
						}
					}
					return false, nil, nil
				})

				r := &AdaptiveRightSizer{
					Client:    ctrlClient,
					ClientSet: fakeClient,
					Config:    cfg,
				}

				// Apply resize policy for container
				ctx := context.Background()
				err := r.applyResizePolicyForContainer(ctx, pod, 0)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Check if resize policy was applied based on feature flag
				if tt.expectResizePolicy {
					if !patchCalled {
						t.Error("Expected patch to be called but it wasn't")
					}
					if len(patchedResizePolicy) != 2 {
						t.Errorf("Expected 2 resize policies, got %d", len(patchedResizePolicy))
					}
				} else {
					if patchCalled {
						t.Error("Expected patch NOT to be called but it was")
					}
				}
			})
		})
	}
}

func TestEnsureParentHasResizePolicyWithFeatureFlag(t *testing.T) {
	tests := []struct {
		name              string
		enableFeatureFlag bool
		expectUpdate      bool
	}{
		{
			name:              "Feature flag enabled - should update parent",
			enableFeatureFlag: true,
			expectUpdate:      true,
		},
		{
			name:              "Feature flag disabled - should not update parent",
			enableFeatureFlag: false,
			expectUpdate:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a deployment
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("128Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("200m"),
											corev1.ResourceMemory: resource.MustParse("256Mi"),
										},
									},
								},
							},
						},
					},
				},
			}

			// Create a replicaset
			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replicaset",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       deployment.Name,
							UID:        deployment.UID,
						},
					},
				},
			}

			// Create a pod with owner reference to the replicaset
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "ReplicaSet",
							Name:       replicaSet.Name,
							UID:        replicaSet.UID,
						},
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}

			// Create fake client
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)
			ctrlClient := ctrlFake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(deployment, replicaSet, pod).
				Build()
			fakeClient := fake.NewSimpleClientset(deployment, replicaSet, pod)

			// Create config with feature flag setting
			cfg := config.GetDefaults()
			cfg.UpdateResizePolicy = tt.enableFeatureFlag

			// Create AdaptiveRightSizer
			r := &AdaptiveRightSizer{
				Client:    ctrlClient,
				ClientSet: fakeClient,
				Config:    cfg,
			}

			// Apply parent resize policy
			ctx := context.Background()
			err := r.ensureParentHasResizePolicy(ctx, pod)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Get the updated deployment from controller-runtime client
			var updatedDeployment appsv1.Deployment
			err = ctrlClient.Get(ctx, types.NamespacedName{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			}, &updatedDeployment)
			if err != nil {
				t.Errorf("failed to get deployment: %v", err)
			}

			// Check if resize policy was applied to deployment based on feature flag
			hasResizePolicy := len(updatedDeployment.Spec.Template.Spec.Containers) > 0 &&
				len(updatedDeployment.Spec.Template.Spec.Containers[0].ResizePolicy) > 0

			if tt.expectUpdate {
				if !hasResizePolicy {
					t.Error("Expected deployment to have resize policy but it doesn't")
				}
				if len(updatedDeployment.Spec.Template.Spec.Containers[0].ResizePolicy) != 2 {
					t.Errorf("Expected 2 resize policies in deployment, got %d",
						len(updatedDeployment.Spec.Template.Spec.Containers[0].ResizePolicy))
				}
			} else {
				if hasResizePolicy {
					t.Error("Expected deployment NOT to have resize policy but it does")
				}
			}
		})
	}
}
