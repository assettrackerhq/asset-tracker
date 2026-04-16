---
title: Helm Installation
visible_when:
  entitlements:
    - isHelmInstallEnabled
---

# Helm Installation

Install your application on a Kubernetes cluster using Helm charts. Read the docs or select your deployment preferences.

## Requirements

Review the following prerequisites before installing.

- Kubernetes cluster v1.26 or later
- Helm 3.x installed on your workstation
- kubectl configured with cluster access
- StorageClass available for persistent volumes

<Tip title="Before You Begin">
Run `kubectl get sc` to confirm a default StorageClass is available. If no default is set, the installation will fail when creating persistent volume claims.
</Tip>

## Configuration

Customize the options below. The install commands will update automatically based on your selections.

<KubernetesDistribution />
<NetworkAvailability installType="helm" />
<RegistryAccess />
<VersionSelector installType="helm" />

## Install

<HelmInstallAssets />

<InstanceName />

## Post-Install

<Note>
After installation, verify that all pods are running with `kubectl get pods -n <namespace>` before proceeding to post-installation configuration.
</Note>

See the post-installation documentation for next steps including configuring TLS, setting up backups, and connecting to your identity provider.
