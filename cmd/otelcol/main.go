// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "k-o11y-otelcol",
		Description: "K-O11y OpenTelemetry Collector with CRD Processor",
		Version:     "0.109.0.1",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					fileprovider.NewFactory(),
					yamlprovider.NewFactory(),
				},
			},
		},
	}

	cmd := otelcol.NewCommand(set)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("collector server run finished with error: %v", err)
	}
}
