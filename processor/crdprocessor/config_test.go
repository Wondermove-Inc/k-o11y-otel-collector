// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with defaults",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
				CustomResources: []CRDConfig{
					{
						Group:       "argoproj.io",
						Version:     "v1alpha1",
						Kind:        "Rollout",
						LabelPrefix: "k8s.rollout",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple CRDs",
			config: &Config{
				CacheTTL:           30 * time.Second,
				CacheMaxSize:       5000,
				APITimeout:         5 * time.Second,
				PassthroughOnError: false,
				CustomResources: []CRDConfig{
					{
						Group:       "argoproj.io",
						Version:     "v1alpha1",
						Kind:        "Rollout",
						LabelPrefix: "k8s.rollout",
					},
					{
						Group:       "serving.knative.dev",
						Version:     "v1",
						Kind:        "Revision",
						LabelPrefix: "k8s.knative.revision",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with empty CustomResources",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
				CustomResources:    []CRDConfig{},
			},
			wantErr: false,
		},
		{
			name: "invalid negative cache_ttl",
			config: &Config{
				CacheTTL:           -1 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
			},
			wantErr: true,
			errMsg:  "cache_ttl must be non-negative",
		},
		{
			name: "invalid negative cache_max_size",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       -1,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
			},
			wantErr: true,
			errMsg:  "cache_max_size must be non-negative",
		},
		{
			name: "invalid negative api_timeout",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         -1 * time.Second,
				PassthroughOnError: true,
			},
			wantErr: true,
			errMsg:  "api_timeout must be non-negative",
		},
		{
			name: "invalid CRD missing kind",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
				CustomResources: []CRDConfig{
					{
						Group:       "argoproj.io",
						Version:     "v1alpha1",
						Kind:        "", // missing
						LabelPrefix: "k8s.rollout",
					},
				},
			},
			wantErr: true,
			errMsg:  "kind is required",
		},
		{
			name: "invalid CRD missing label_prefix",
			config: &Config{
				CacheTTL:           60 * time.Second,
				CacheMaxSize:       10000,
				APITimeout:         10 * time.Second,
				PassthroughOnError: true,
				CustomResources: []CRDConfig{
					{
						Group:       "argoproj.io",
						Version:     "v1alpha1",
						Kind:        "Rollout",
						LabelPrefix: "", // missing
					},
				},
			},
			wantErr: true,
			errMsg:  "label_prefix is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCRDConfig(t *testing.T) {
	crd := CRDConfig{
		Group:       "argoproj.io",
		Version:     "v1alpha1",
		Kind:        "Rollout",
		LabelPrefix: "k8s.rollout",
	}

	assert.Equal(t, "argoproj.io", crd.Group)
	assert.Equal(t, "v1alpha1", crd.Version)
	assert.Equal(t, "Rollout", crd.Kind)
	assert.Equal(t, "k8s.rollout", crd.LabelPrefix)
}
