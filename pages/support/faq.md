---
title: Frequently Asked Questions
---

# Frequently Asked Questions

## General

{{#if entitlements.isEmbeddedClusterDownloadEnabled}}
<Accordion title="What are the system requirements?">

See [Requirements](../installation/requirements) for details.

</Accordion>
{{/if}}

<Accordion title="How do I check for updates?">

See [Checking for Updates](../updates/checking).

</Accordion>

{{#if entitlements.isHelmInstallEnabled}}
## Installation

<Accordion title="Which installation method should I use?">

{{#if entitlements.isEmbeddedClusterDownloadEnabled}}
Choose [Embedded Cluster (Linux)](../installation/linux) for installing on a Linux server, or [Helm](../installation/helm) for deploying to an existing Kubernetes cluster.
{{/if}}

</Accordion>
{{/if}}

## Troubleshooting

<Accordion title="How do I collect diagnostic information?">

Generate a support bundle for troubleshooting:

{{#if entitlements.isEmbeddedClusterDownloadEnabled}}
- [Linux installations](../operations/bundles/linux)
{{/if}}
{{#if entitlements.isHelmInstallEnabled}}
- [Helm installations](../operations/bundles/helm)
{{/if}}
- If you already have a support bundle, [upload it here](../operations/bundles/uploaded)

</Accordion>
