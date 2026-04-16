// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// InformerCache manages ReplicaSet -> Owner mapping using K8s Informer
type InformerCache struct {
	logger       *zap.Logger
	clientset    kubernetes.Interface
	factory      informers.SharedInformerFactory
	informer     cache.SharedIndexInformer
	stopCh       chan struct{}
	stopOnce     sync.Once
	ownerMap     map[string]*OwnerInfo // key: namespace/rsName
	mu           sync.RWMutex
	supportedCRD map[string]string // Kind -> LabelPrefix
	resyncPeriod time.Duration
	maxSize      int
}

// NewInformerCache creates a new InformerCache
func NewInformerCache(
	logger *zap.Logger,
	crds []CRDConfig,
	resyncPeriod time.Duration,
	maxSize int,
) (*InformerCache, error) {
	// Build supported CRD map
	supportedCRD := make(map[string]string)
	for _, crd := range crds {
		supportedCRD[crd.Kind] = crd.LabelPrefix
	}

	// Default maxSize if not set
	if maxSize <= 0 {
		maxSize = 10000
	}

	return &InformerCache{
		logger:       logger,
		stopCh:       make(chan struct{}),
		ownerMap:     make(map[string]*OwnerInfo),
		supportedCRD: supportedCRD,
		resyncPeriod: resyncPeriod,
		maxSize:      maxSize,
	}, nil
}

// Start initializes and starts the informer
func (ic *InformerCache) Start(ctx context.Context) error {
	// Get in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		ic.logger.Warn("Failed to get in-cluster config, using empty cache",
			zap.Error(err),
		)
		// Return without error - we'll just have an empty cache
		// This allows local development/testing without K8s
		return nil
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	ic.clientset = clientset

	// Create shared informer factory
	ic.factory = informers.NewSharedInformerFactory(clientset, ic.resyncPeriod)
	ic.informer = ic.factory.Apps().V1().ReplicaSets().Informer()

	// Add event handlers
	ic.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ic.onAdd,
		UpdateFunc: ic.onUpdate,
		DeleteFunc: ic.onDelete,
	})

	// Start informer
	go ic.informer.Run(ic.stopCh)

	// Wait for cache sync
	ic.logger.Info("Waiting for informer cache to sync...")
	if !cache.WaitForCacheSync(ic.stopCh, ic.informer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}

	ic.logger.Info("Informer cache synced",
		zap.Int("cached_replicasets", len(ic.ownerMap)),
	)

	return nil
}

// Stop stops the informer (safe to call multiple times)
func (ic *InformerCache) Stop() {
	ic.stopOnce.Do(func() {
		close(ic.stopCh)
		ic.logger.Info("Informer cache stopped")
	})
}

// makeKey creates a cache key from namespace and name
func makeKey(namespace, name string) string {
	return namespace + "/" + name
}

// Get retrieves owner info for a ReplicaSet
func (ic *InformerCache) Get(namespace, rsName string) (*OwnerInfo, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	key := makeKey(namespace, rsName)
	owner, ok := ic.ownerMap[key]
	return owner, ok
}

func (ic *InformerCache) onAdd(obj interface{}) {
	rs, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		return
	}
	ic.updateOwnerMap(rs)
}

func (ic *InformerCache) onUpdate(oldObj, newObj interface{}) {
	rs, ok := newObj.(*appsv1.ReplicaSet)
	if !ok {
		return
	}
	ic.updateOwnerMap(rs)
}

func (ic *InformerCache) onDelete(obj interface{}) {
	rs, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		rs, ok = tombstone.Obj.(*appsv1.ReplicaSet)
		if !ok {
			return
		}
	}

	ic.mu.Lock()
	defer ic.mu.Unlock()

	key := makeKey(rs.Namespace, rs.Name)
	delete(ic.ownerMap, key)

	ic.logger.Debug("Removed ReplicaSet from cache",
		zap.String("key", key),
	)
}

func (ic *InformerCache) updateOwnerMap(rs *appsv1.ReplicaSet) {
	// Check each OwnerReference
	for _, ownerRef := range rs.OwnerReferences {
		// Check if this is a supported CRD
		if _, supported := ic.supportedCRD[ownerRef.Kind]; supported {
			ic.mu.Lock()

			key := makeKey(rs.Namespace, rs.Name)

			// Check if entry already exists (update case)
			_, exists := ic.ownerMap[key]

			// Enforce maxSize limit (only for new entries)
			if !exists && len(ic.ownerMap) >= ic.maxSize {
				ic.mu.Unlock()
				ic.logger.Warn("Cache size limit reached, skipping new entry",
					zap.Int("current_size", len(ic.ownerMap)),
					zap.Int("max_size", ic.maxSize),
					zap.String("skipped_key", key),
				)
				return
			}

			ic.ownerMap[key] = &OwnerInfo{
				Kind:      ownerRef.Kind,
				Name:      ownerRef.Name,
				UID:       string(ownerRef.UID),
				Namespace: rs.Namespace,
			}
			ic.mu.Unlock()

			ic.logger.Debug("Updated ReplicaSet owner mapping",
				zap.String("replicaset", key),
				zap.String("owner_kind", ownerRef.Kind),
				zap.String("owner_name", ownerRef.Name),
			)
			return
		}
	}
}
