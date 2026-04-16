// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package crdprocessor

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the crdprocessor
type Config struct {
	// CacheTTL is the time-to-live for cache entries
	// Default: 60s
	CacheTTL time.Duration `mapstructure:"cache_ttl"`

	// CacheMaxSize is the maximum number of entries in the cache
	// Default: 10000
	CacheMaxSize int `mapstructure:"cache_max_size"`

	// APITimeout is the timeout for K8s API calls (used during initial sync)
	// Default: 10s
	APITimeout time.Duration `mapstructure:"api_timeout"`

	// PassthroughOnError allows data to pass through when errors occur
	// If true, telemetry data is passed without CRD labels when lookup fails
	// Default: true
	PassthroughOnError bool `mapstructure:"passthrough_on_error"`

	// CustomResources is the list of CRDs to process
	CustomResources []CRDConfig `mapstructure:"custom_resources"`
}

// CRDConfig defines a single CRD to process
type CRDConfig struct {
	// Group is the API group (e.g., "argoproj.io")
	Group string `mapstructure:"group"`

	// Version is the API version (e.g., "v1alpha1")
	Version string `mapstructure:"version"`

	// Kind is the resource kind (e.g., "Rollout")
	Kind string `mapstructure:"kind"`

	// LabelPrefix is the prefix for generated labels (e.g., "k8s.rollout")
	// The processor will add {LabelPrefix}.name and {LabelPrefix}.uid
	LabelPrefix string `mapstructure:"label_prefix"`
}

var _ component.Config = (*Config)(nil)

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.CacheTTL < 0 {
		return errors.New("cache_ttl must be non-negative")
	}
	if cfg.CacheMaxSize < 0 {
		return errors.New("cache_max_size must be non-negative")
	}
	if cfg.APITimeout < 0 {
		return errors.New("api_timeout must be non-negative")
	}

	for i, crd := range cfg.CustomResources {
		if crd.Kind == "" {
			return errors.New("custom_resources[" + string(rune(i)) + "].kind is required")
		}
		if crd.LabelPrefix == "" {
			return errors.New("custom_resources[" + string(rune(i)) + "].label_prefix is required")
		}
	}

	return nil
}
