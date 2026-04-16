// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewInformerCache(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
		{Kind: "Revision", LabelPrefix: "k8s.knative.revision"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)
	require.NotNil(t, cache)

	assert.Len(t, cache.supportedCRD, 2)
	assert.Equal(t, "k8s.rollout", cache.supportedCRD["Rollout"])
	assert.Equal(t, "k8s.knative.revision", cache.supportedCRD["Revision"])
	assert.Equal(t, 60*time.Second, cache.resyncPeriod)
	assert.NotNil(t, cache.ownerMap)
	assert.NotNil(t, cache.stopCh)
}

func TestInformerCache_Get(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Test get when empty
	owner, found := cache.Get("default", "my-rs")
	assert.False(t, found)
	assert.Nil(t, owner)

	// Add entry directly
	cache.mu.Lock()
	cache.ownerMap["default/my-rs"] = &OwnerInfo{
		Kind:      "Rollout",
		Name:      "my-rollout",
		UID:       "uid-12345",
		Namespace: "default",
	}
	cache.mu.Unlock()

	// Test get after adding
	owner, found = cache.Get("default", "my-rs")
	assert.True(t, found)
	require.NotNil(t, owner)
	assert.Equal(t, "Rollout", owner.Kind)
	assert.Equal(t, "my-rollout", owner.Name)
	assert.Equal(t, "uid-12345", owner.UID)
	assert.Equal(t, "default", owner.Namespace)
}

func TestInformerCache_GetConcurrent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Add test data
	cache.mu.Lock()
	cache.ownerMap["default/test-rs"] = &OwnerInfo{
		Kind: "Rollout",
		Name: "test-rollout",
		UID:  "uid-test",
	}
	cache.mu.Unlock()

	// Concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			owner, found := cache.Get("default", "test-rs")
			assert.True(t, found)
			assert.Equal(t, "test-rollout", owner.Name)
		}()
	}
	wg.Wait()
}

func TestInformerCache_OnAdd(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create a ReplicaSet with Rollout owner
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc123",
			Namespace: "production",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "Rollout",
					Name:       "my-app",
					UID:        types.UID("rollout-uid-12345"),
				},
			},
		},
	}

	// Call onAdd
	cache.onAdd(rs)

	// Verify cache entry
	owner, found := cache.Get("production", "my-app-abc123")
	require.True(t, found)
	assert.Equal(t, "Rollout", owner.Kind)
	assert.Equal(t, "my-app", owner.Name)
	assert.Equal(t, "rollout-uid-12345", owner.UID)
	assert.Equal(t, "production", owner.Namespace)
}

func TestInformerCache_OnAdd_UnsupportedKind(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create a ReplicaSet with Deployment owner (not in supported CRDs)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy-abc123",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment", // Not supported
					Name:       "my-deploy",
					UID:        types.UID("deploy-uid-12345"),
				},
			},
		},
	}

	// Call onAdd
	cache.onAdd(rs)

	// Verify cache entry was NOT added
	_, found := cache.Get("default", "my-deploy-abc123")
	assert.False(t, found, "Deployment-owned ReplicaSet should not be cached")
}

func TestInformerCache_OnAdd_NoOwner(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create a ReplicaSet without OwnerReferences
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "standalone-rs",
			Namespace:       "default",
			OwnerReferences: []metav1.OwnerReference{}, // empty
		},
	}

	// Call onAdd
	cache.onAdd(rs)

	// Verify cache entry was NOT added
	_, found := cache.Get("default", "standalone-rs")
	assert.False(t, found, "ReplicaSet without owner should not be cached")
}

func TestInformerCache_OnAdd_InvalidObject(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Call onAdd with invalid object (should not panic)
	cache.onAdd("not a replicaset")
	cache.onAdd(nil)
	cache.onAdd(123)

	// Verify cache is still empty
	assert.Len(t, cache.ownerMap, 0)
}

func TestInformerCache_OnUpdate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create old and new ReplicaSets
	oldRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc123",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Rollout",
					Name: "my-app-old",
					UID:  types.UID("old-uid"),
				},
			},
		},
	}

	newRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-abc123",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Rollout",
					Name: "my-app-new",
					UID:  types.UID("new-uid"),
				},
			},
		},
	}

	// Add initial entry
	cache.onAdd(oldRS)

	owner, found := cache.Get("default", "my-app-abc123")
	require.True(t, found)
	assert.Equal(t, "my-app-old", owner.Name)

	// Call onUpdate
	cache.onUpdate(oldRS, newRS)

	// Verify cache was updated
	owner, found = cache.Get("default", "my-app-abc123")
	require.True(t, found)
	assert.Equal(t, "my-app-new", owner.Name)
	assert.Equal(t, "new-uid", owner.UID)
}

func TestInformerCache_OnDelete(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create and add a ReplicaSet
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "to-be-deleted",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Rollout",
					Name: "my-app",
					UID:  types.UID("uid-12345"),
				},
			},
		},
	}

	cache.onAdd(rs)

	// Verify entry exists
	_, found := cache.Get("default", "to-be-deleted")
	require.True(t, found)

	// Call onDelete
	cache.onDelete(rs)

	// Verify entry was removed
	_, found = cache.Get("default", "to-be-deleted")
	assert.False(t, found, "Entry should be removed after delete")
}

func TestInformerCache_OnDelete_InvalidObject(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Add an entry
	cache.mu.Lock()
	cache.ownerMap["default/my-rs"] = &OwnerInfo{
		Kind: "Rollout",
		Name: "my-app",
	}
	cache.mu.Unlock()

	// Call onDelete with invalid objects (should not panic or remove entry)
	cache.onDelete("not a replicaset")
	cache.onDelete(nil)
	cache.onDelete(123)

	// Verify entry still exists
	_, found := cache.Get("default", "my-rs")
	assert.True(t, found, "Entry should still exist after invalid delete")
}

func TestInformerCache_MultipleOwnerReferences(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Create a ReplicaSet with multiple owner references
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complex-rs",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment", // Not supported - should be skipped
					Name: "deploy-owner",
					UID:  types.UID("deploy-uid"),
				},
				{
					Kind: "Rollout", // Supported - should be used
					Name: "rollout-owner",
					UID:  types.UID("rollout-uid"),
				},
				{
					Kind: "StatefulSet", // Not supported - should be skipped
					Name: "sts-owner",
					UID:  types.UID("sts-uid"),
				},
			},
		},
	}

	cache.onAdd(rs)

	// Verify the Rollout owner was cached
	owner, found := cache.Get("default", "complex-rs")
	require.True(t, found)
	assert.Equal(t, "Rollout", owner.Kind)
	assert.Equal(t, "rollout-owner", owner.Name)
}

func TestInformerCache_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Call Stop (should not panic)
	cache.Stop()

	// Verify stopCh is closed
	select {
	case <-cache.stopCh:
		// Channel is closed, as expected
	default:
		t.Error("stopCh should be closed after Stop()")
	}
}

func TestInformerCache_Stop_DoubleCall(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	// Call Stop twice - should NOT panic due to sync.Once
	assert.NotPanics(t, func() {
		cache.Stop()
		cache.Stop()
	})
}

func TestInformerCache_MaxSize(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	// Create cache with maxSize of 2
	cache, err := NewInformerCache(logger, crds, 60*time.Second, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, cache.maxSize)

	// Add first ReplicaSet
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Rollout", Name: "rollout-1", UID: "uid-1"},
			},
		},
	}
	cache.onAdd(rs1)

	// Add second ReplicaSet
	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-2",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Rollout", Name: "rollout-2", UID: "uid-2"},
			},
		},
	}
	cache.onAdd(rs2)

	// Both should be in cache
	_, found1 := cache.Get("default", "rs-1")
	_, found2 := cache.Get("default", "rs-2")
	assert.True(t, found1)
	assert.True(t, found2)

	// Add third ReplicaSet - should be skipped due to maxSize
	rs3 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs-3",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Rollout", Name: "rollout-3", UID: "uid-3"},
			},
		},
	}
	cache.onAdd(rs3)

	// Third should NOT be in cache
	_, found3 := cache.Get("default", "rs-3")
	assert.False(t, found3, "Third entry should be skipped due to maxSize limit")

	// Cache size should still be 2
	assert.Len(t, cache.ownerMap, 2)
}

func TestUpdateOwnerMap_Concurrent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	crds := []CRDConfig{
		{Kind: "Rollout", LabelPrefix: "k8s.rollout"},
	}

	cache, err := NewInformerCache(logger, crds, 60*time.Second, 10000)
	require.NoError(t, err)

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rs-" + string(rune('A'+idx)),
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Rollout",
							Name: "rollout-" + string(rune('A'+idx)),
							UID:  types.UID("uid-" + string(rune('A'+idx))),
						},
					},
				},
			}
			cache.updateOwnerMap(rs)
		}(i)
	}

	// Concurrent reads while writing
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cache.Get("default", "rs-"+string(rune('A'+idx)))
		}(i)
	}

	wg.Wait()
}

func TestOwnerInfo(t *testing.T) {
	owner := &OwnerInfo{
		Kind:      "Rollout",
		Name:      "my-rollout",
		UID:       "uid-12345",
		Namespace: "production",
	}

	assert.Equal(t, "Rollout", owner.Kind)
	assert.Equal(t, "my-rollout", owner.Name)
	assert.Equal(t, "uid-12345", owner.UID)
	assert.Equal(t, "production", owner.Namespace)
}
