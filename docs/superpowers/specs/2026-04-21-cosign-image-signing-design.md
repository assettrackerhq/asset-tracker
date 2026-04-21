# Cosign keyless image signing and verification

## Goal

Add cryptographic signing and post-push verification to the Docker image
publish workflow so that consumers (and the workflow itself) can prove each
published image was built by this repository's `docker-publish.yml` workflow.

## Scope

Changes are limited to `.github/workflows/docker-publish.yml`. The backend and
frontend images (both matrix entries) are signed and verified on every run
triggered by pushes to `main` or tags matching `v*`.

Out of scope: signing the Helm chart, signing SBOMs, provenance attestations,
policy enforcement at admission time, and any consumer-side verification
tooling or documentation.

## Approach

Use [cosign](https://github.com/sigstore/cosign) in keyless mode. Keyless
signing uses the GitHub Actions OIDC token to obtain an ephemeral signing
certificate from Fulcio; the signature and certificate are recorded in the
Rekor public transparency log. No long-lived signing key or secret is
introduced.

Images are signed by digest, not by tag. The `docker/build-push-action` step
already outputs the pushed digest; signing that digest covers every tag that
resolves to it and prevents signature confusion if a tag is later retagged.

Verification runs in the same job, immediately after signing, and pins the
expected certificate identity (the workflow file) and OIDC issuer (GitHub
Actions). Verification failure fails the job.

## Workflow changes

### Permissions

Add job-level permissions to `build-and-push`:

```yaml
permissions:
  contents: read
  id-token: write
```

`id-token: write` is required for the runner to mint the OIDC token cosign
exchanges with Fulcio.

### Steps (added after the existing `docker/build-push-action` step)

1. **Install cosign** via `sigstore/cosign-installer@v3`.
2. **Sign** the pushed image by digest:

   ```yaml
   - name: Sign image
     env:
       DIGEST: ${{ steps.build.outputs.digest }}
       IMAGE: ${{ matrix.image }}
     run: cosign sign --yes "${IMAGE}@${DIGEST}"
   ```

   The existing `docker/build-push-action` step must be given `id: build` so
   its `digest` output is accessible.

3. **Verify** the signature we just created:

   ```yaml
   - name: Verify signature
     env:
       DIGEST: ${{ steps.build.outputs.digest }}
       IMAGE: ${{ matrix.image }}
     run: |
       cosign verify \
         --certificate-oidc-issuer https://token.actions.githubusercontent.com \
         --certificate-identity-regexp '^https://github\.com/assettrackerhq/asset-tracker/\.github/workflows/docker-publish\.yml@refs/(heads/main|tags/v.*)$' \
         "${IMAGE}@${DIGEST}"
   ```

   The identity regex accepts both triggers this workflow supports: pushes to
   `main` and `v*` tag pushes. Rekor transparency log verification is
   automatic in keyless mode.

## Data flow (per matrix entry)

```
build-push-action → digest output
        │
        ▼
cosign sign <image>@<digest>
    ├── OIDC token (GitHub) → Fulcio → short-lived cert
    └── signature + cert bundle → Rekor transparency log
        │
        ▼
cosign verify <image>@<digest>
    ├── fetch signature bundle from registry
    ├── check cert chain back to Fulcio root
    ├── check cert identity regex and OIDC issuer
    └── check Rekor inclusion proof
```

## Error handling

- **Signing failure** (e.g., OIDC token unavailable, Fulcio unreachable): the
  `cosign sign` step exits non-zero and fails the job. The image is already
  pushed at this point; a failed signing run leaves an unsigned image in the
  registry. Re-running the job re-signs the same digest — cosign is
  idempotent and simply adds another signature attestation to the same
  digest.
- **Verification failure**: the `cosign verify` step exits non-zero and fails
  the job. This indicates the signature we just created doesn't match the
  expected identity — a real bug in the workflow configuration that should
  block the run.

## Testing

Validation happens via the workflow itself. After merging:

1. Push to a throwaway branch with a one-line change to force a workflow
   run (the workflow only triggers on `main` and `v*`, so merge to `main`
   is required).
2. Confirm the `Sign image` and `Verify signature` steps pass for both
   matrix entries.
3. Locally verify the published image:

   ```bash
   cosign verify \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     --certificate-identity-regexp '^https://github\.com/assettrackerhq/asset-tracker/\.github/workflows/docker-publish\.yml@refs/(heads/main|tags/v.*)$' \
     unawake2068/asset-tracker-backend:latest
   ```

## Non-goals / follow-ups

- SBOM generation and attestation (`cosign attest`).
- SLSA provenance attestation.
- Consumer-facing documentation on how to verify pulled images.
- Admission controller / policy engine enforcement in the Helm chart.

These can be added later without changing the signing foundation established
here.
