// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

// Package crdprocessor provides a processor that adds Kubernetes CRD
// (Custom Resource Definition) labels to telemetry data.
//
// The processor looks up OwnerReferences of ReplicaSets to find CRD owners
// like Argo Rollouts, and adds corresponding labels (e.g., k8s.rollout.name)
// to traces, metrics, and logs.
//
// This enables filtering and grouping by CRD workloads in observability tools
// like SigNoz, which is not possible with the standard k8sattributes processor.
package crdprocessor
