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

package predictive

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"right-sizer/events"
	"right-sizer/memstore"
)

func TestNewScaler(t *testing.T) {
	client := fake.NewSimpleClientset()
	store := memstore.NewMemoryStore(7, 1440)
	eventBus := events.NewEventBus(100)
	config := DefaultScalerConfig()

	scaler := NewScaler(client, store, eventBus, logr.Discard(), config)

	if scaler == nil {
		t.Fatal("Scaler should not be nil")
	}
	if scaler.k8sClient != client {
		t.Error("k8sClient should be set")
	}
	if scaler.store != store {
		t.Error("store should be set")
	}
}

func TestDefaultScalerConfig(t *testing.T) {
	config := DefaultScalerConfig()

	if config.CheckInterval != 5*time.Minute {
		t.Errorf("Expected check interval 5m, got %v", config.CheckInterval)
	}
	if config.MinConfidence != 0.75 {
		t.Errorf("Expected min confidence 0.75, got %f", config.MinConfidence)
	}
	if config.AutoApply {
		t.Error("AutoApply should be false by default")
	}
}

func TestShouldProcessPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	store := memstore.NewMemoryStore(7, 1440)
	eventBus := events.NewEventBus(100)
	config := DefaultScalerConfig()

	scaler := NewScaler(client, store, eventBus, logr.Discard(), config)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "Running pod old enough",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expected: true,
		},
		{
			name: "Running pod too new",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expected: false,
		},
		{
			name: "Pod not running",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expected: false,
		},
		{
			name: "Pod in excluded namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pod",
					Namespace:         "kube-system",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scaler.shouldProcessPod(tt.pod)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	store := memstore.NewMemoryStore(7, 1440)
	eventBus := events.NewEventBus(100)
	config := DefaultScalerConfig()
	config.AutoApply = true // Customize

	scaler := NewScaler(client, store, eventBus, logr.Discard(), config)

	retrievedConfig := scaler.GetConfig()
	if retrievedConfig.AutoApply != true {
		t.Error("Config AutoApply should be true")
	}
}

func TestUpdateConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	store := memstore.NewMemoryStore(7, 1440)
	eventBus := events.NewEventBus(100)
	config := DefaultScalerConfig()

	scaler := NewScaler(client, store, eventBus, logr.Discard(), config)

	newConfig := DefaultScalerConfig()
	newConfig.MinConfidence = 0.9
	newConfig.AutoApply = true

	scaler.UpdateConfig(newConfig)

	updatedConfig := scaler.GetConfig()
	if updatedConfig.MinConfidence != 0.9 {
		t.Errorf("Expected min confidence 0.9, got %f", updatedConfig.MinConfidence)
	}
	if !updatedConfig.AutoApply {
		t.Error("AutoApply should be true after update")
	}
}

func TestScalerStartStop(t *testing.T) {
	client := fake.NewSimpleClientset()
	store := memstore.NewMemoryStore(7, 1440)
	eventBus := events.NewEventBus(100)
	config := DefaultScalerConfig()
	config.CheckInterval = 100 * time.Millisecond

	scaler := NewScaler(client, store, eventBus, logr.Discard(), config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should succeed
	err := scaler.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Starting again should fail
	err = scaler.Start(ctx)
	if err == nil {
		t.Error("Second start should fail")
	}

	// Stop should not panic
	scaler.Stop()

	// Stop again should not panic
	scaler.Stop()
}
