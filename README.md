# heft

`heft` is a small CLI tool that scans Helm charts and reports the container images they use. It prefers high-confidence information from rendered Kubernetes manifests (via `helm template`), and falls back to static YAML and regex-based detection when necessary.

## Usage

```bash
heft scan <chart-ref> [flags]
```

Where `<chart-ref>` can be:

- A local chart directory
- A local packaged chart: `*.tgz`
- An HTTP(S) URL to a chart archive (`.tgz`)
- An `oci://` reference to an OCI-backed chart

When a remote chart reference (HTTP(S) URL or `oci://` ref) is used, `heft`
will download the chart into a temporary directory, extract it, and run all
its detectors (rendered, static, and regex) against the local copy.

Examples:

```bash
# Scan a local chart directory
heft scan ./charts/my-app

# Scan a local packaged chart
printf "" | helm template test ./charts/my-app  # sanity check
heft scan ./charts/my-app-0.1.0.tgz

# Scan a remote chart URL (full scan after download)
heft scan https://charts.example.com/my-app-0.1.0.tgz

# Scan an OCI chart (Helm must be configured for OCI)
heft scan oci://registry.example.com/my-app:0.1.0
```

### Flags

- `--min-confidence=low|medium|high`
  - Filter results by minimum confidence.
  - `high`: only rendered-manifest images (from `helm template`).
  - `medium`: rendered-manifest + static YAML-based images.
  - `low` (default): include regex-based heuristic matches as well.

- `--no-helm-deps`
  - Disable the automatic `helm dependency build` retry when rendering a local chart directory fails due to missing dependencies.

- `--include-optional-deps`
  - When set, also scan subcharts under `charts/` that may be brought in via optional/conditional dependencies. For remote charts, `heft` runs `helm dependency build` first so OCI/remote deps are available locally.

- `--verbose`, `-v`
  - Enable verbose logging on stderr, including which charts/subcharts are scanned and what `helm template` commands are run.

- `--set=key=val`, `--set-string=key=val`
  - Passed through to `helm template` unchanged.

- `--values=path`, `-f=path`
  - Passed through to `helm template` unchanged.

## Output

`heft` prints a YAML document describing discovered images, for example:

```yaml
images:
  - name: ghcr.io/external-secrets/external-secrets:v1.2.1
    confidence: high
    source: rendered-manifest
  - name: example.com/basic/app:v1
    confidence: medium
    source: static-yaml
    file: internal/scan/testdata/basic-chart/values.yaml
```

- `confidence`: one of `high`, `medium`, `low`.
- `source`:
  - `rendered-manifest` for images found via `helm template`.
  - `static-yaml` for images inferred from values/manifests without rendering.
  - `regex-scan` for heuristic matches in files.

Higher-confidence images are preferred and de-duplicated per repository:

- Rendered images win over static and regex-based ones for the same repo.
- Tagged images win over untagged configs at the same confidence level.

## Requirements

- Go toolchain (to build the binary):

```bash
go build ./cmd/heft
```

- Helm 3 on `PATH` for rendered-manifest detection and end-to-end tests.

## Implementation notes

`heft` downloads remote charts when needed, runs Helm to render manifests, and then combines several detection strategies to find container images. It de-duplicates results and lets you filter them by confidence level using `--min-confidence`.

## Code generation and tooling

This project has been substantially generated and maintained with the help of OpenCode, including refactors, test additions, and documentation updates.
