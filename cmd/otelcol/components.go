// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"

	// Extensions
	"go.opentelemetry.io/collector/extension/zpagesextension"

	// Extensions - Contrib
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"

	// Receivers - Core
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	// Receivers - Contrib
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

	// Processors - Core
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"

	// Processors - Contrib
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

	// Processors - Custom
	"github.com/Wondermove-Inc/k-o11y-otel-collector/processor/crdprocessor"

	// Exporters - Core
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	// Exporters - Contrib
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
)

func components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	// Extensions
	factories.Extensions, err = extension.MakeFactoryMap(
		zpagesextension.NewFactory(),
		healthcheckextension.NewFactory(),
		pprofextension.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Receivers
	factories.Receivers, err = receiver.MakeFactoryMap(
		// Core
		otlpreceiver.NewFactory(),
		// Contrib
		filelogreceiver.NewFactory(),
		hostmetricsreceiver.NewFactory(),
		k8sclusterreceiver.NewFactory(),
		k8seventsreceiver.NewFactory(),
		kubeletstatsreceiver.NewFactory(),
		prometheusreceiver.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Processors
	factories.Processors, err = processor.MakeFactoryMap(
		// Core
		batchprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
		// Contrib
		attributesprocessor.NewFactory(),
		filterprocessor.NewFactory(),
		k8sattributesprocessor.NewFactory(),
		metricstransformprocessor.NewFactory(),
		resourcedetectionprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
		transformprocessor.NewFactory(),
		// Custom - CRD Processor
		crdprocessor.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Exporters
	factories.Exporters, err = exporter.MakeFactoryMap(
		// Core
		debugexporter.NewFactory(),
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
		// Contrib
		clickhouseexporter.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	// Connectors (empty for now)
	factories.Connectors, err = connector.MakeFactoryMap()
	if err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
