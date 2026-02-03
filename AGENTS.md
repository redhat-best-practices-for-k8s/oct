# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

OCT (Offline Catalog Tool) is a containerized Go application that retrieves certified artifacts (operators, containers, and helm charts) from Red Hat's online catalog (Pyxis) for use in disconnected/air-gapped environments.

### Purpose

The primary use case is supporting Red Hat's certification tools like the certsuite (TNF) when running in environments without internet access. The certsuite's `affiliated-certification` test suite verifies certification status of CNF components. When the online Red Hat catalog is unreachable, it falls back to OCT's embedded offline database.

### Key Features

- Downloads and caches certified container images catalog from Red Hat
- Downloads and caches certified operators catalog (certified-operators and redhat-operators organizations)
- Downloads and caches certified Helm charts from OpenShift Charts repository
- Container image is rebuilt automatically every 6 hours via GitHub Actions
- Supports multi-architecture builds (amd64, arm64, ppc64le, s390x)

### Container Image

Available at: `quay.io/redhat-best-practices-for-k8s/oct:latest`

## Build Commands

### Build the OCT Binary

```bash
make build-oct           # Build the oct binary
make build-oct-debug     # Build with debug symbols
make build               # Run lint, test, and build
```

### Run Tests

```bash
make test                # Run unit tests with coverage
make coverage-html       # Generate HTML coverage report
```

### Linting

```bash
make lint                # Run golangci-lint (5m timeout)
make vet                 # Run go vet on all packages
```

### Dependency Management

```bash
make update-deps         # Run go mod tidy and vendor
```

### Update Certified Catalog

```bash
make update-certified-catalog  # Fetch latest catalog data (operators, containers, helm)
```

### Clean

```bash
make clean               # Remove built binary and coverage files
```

## Container Usage

### Get Embedded Database (Dump Only Mode)

Extract the pre-built offline database from the container:

```bash
mkdir db
docker run -v $(pwd)/db:/tmp/dump:Z --user $(id -u):$(id -g) \
    --env OCT_DUMP_ONLY=true \
    quay.io/redhat-best-practices-for-k8s/oct:latest
```

### Fetch Latest Online Catalog

Download fresh catalog data from Red Hat's online services:

```bash
mkdir db
docker run -v $(pwd)/db:/tmp/dump:Z --user $(id -u):$(id -g) \
    quay.io/redhat-best-practices-for-k8s/oct:latest
```

### Force Database Regeneration

```bash
docker run -v $(pwd)/db:/tmp/dump:Z --user $(id -u):$(id -g) \
    --env OCT_FORCE_REGENERATE_DB=true \
    quay.io/redhat-best-practices-for-k8s/oct:latest
```

### Build Local Container Image

```bash
docker build -t oct:local --build-arg OCT_LOCAL_FOLDER=. \
    --no-cache -f Dockerfile.local .
```

## Code Organization

```
oct/
├── cmd/
│   └── tnf/
│       ├── main.go           # CLI entry point (Cobra)
│       └── fetch/
│           └── fetch.go      # Fetch command implementation
├── pkg/
│   └── certdb/
│       ├── certdb.go         # CertificationStatusValidator interface
│       ├── config/
│       │   └── config.go     # Registry mappings and configuration
│       ├── offlinecheck/
│       │   ├── container.go  # Offline container certification check
│       │   ├── operator.go   # Offline operator certification check
│       │   ├── helm.go       # Offline helm chart certification check
│       │   └── registry.go   # Catalog loading orchestration
│       └── onlinecheck/
│           └── onlinecheck.go # Online certification check via Pyxis API
├── scripts/
│   ├── run.sh                # Container entrypoint script
│   └── curl-endpoints.sh     # API endpoint verification
├── Dockerfile                # Production multi-stage build
├── Dockerfile.local          # Local development build
└── .golangci.yml             # Linter configuration
```

### Key Packages

**cmd/tnf/fetch**: The `fetch` subcommand that downloads catalog data from Red Hat's APIs:
- Containers: `https://catalog.redhat.com/api/containers/v1/images`
- Operators: `https://catalog.redhat.com/api/containers/v1/operators/bundles`
- Helm Charts: `https://charts.openshift.io/index.yaml`

**pkg/certdb**: Core certification validation logic with two implementations:
- `OnlineValidator`: Queries Red Hat's Pyxis API in real-time
- `OfflineValidator`: Uses locally cached database files

**pkg/certdb/config**: Registry mapping configuration (e.g., `registry.redhat.io` to `registry.access.redhat.com`)

### Database Format

The offline database is stored in JSON format:

```
data/
├── archive.json              # Catalog metadata (counts)
├── containers/
│   └── containers.db         # Certified containers (JSON)
├── operators/
│   ├── operator_catalog_page_0_500.db
│   ├── operator_catalog_page_1_500.db
│   └── ...                   # Paginated operator catalog (JSON)
└── helm/
    └── helm.db               # Helm chart index (YAML)
```

## Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/sirupsen/logrus` - Logging
- `helm.sh/helm/v3` - Helm chart handling
- `github.com/Masterminds/semver/v3` - Semantic version parsing
- `gopkg.in/yaml.v3` - YAML parsing

## Development Guidelines

### Go Version

This repository uses Go 1.25.6.

### Testing Framework

Uses standard Go testing with testify assertions. Run tests with:

```bash
UNIT_TEST="true" go test -coverprofile=cover.out ./...
```

### Linting

All code must pass golangci-lint with the configuration in `.golangci.yml`. Key enabled linters include:
- errcheck, govet, staticcheck
- gosec (security)
- funlen (max 60 lines, 45 statements)
- gocyclo (max complexity 20)
- lll (max line length 250)

### API Endpoints

The tool interacts with these Red Hat APIs:
- **Pyxis Container API**: `https://catalog.redhat.com/api/containers/v1/`
- **OpenShift Charts**: `https://charts.openshift.io/index.yaml`
- **Health Check**: `https://catalog.redhat.com/api/containers/v1/ping`

### Important Notes

1. **Only partner certified images** are stored in the offline database. Red Hat images checked against the offline database will appear as not certified (the online database includes both).

2. The container image is automatically rebuilt every 6 hours via the `recreate-image.yml` GitHub Actions workflow.

3. The fetch tool downloads catalog pages concurrently using goroutines for improved performance.

4. The database format is coupled with what certsuite (TNF) expects. Changes to the format require coordination with the certsuite repository.

## Common Workflows

### Running OCT Locally

```bash
# Build the binary
make build-oct

# Fetch all catalogs
./oct fetch --operator --container --helm
```

### Verifying Certification Status Programmatically

```go
import "github.com/redhat-best-practices-for-k8s/oct/pkg/certdb"

validator, err := certdb.GetValidator("/path/to/offline/db")
if err != nil {
    // Handle error - falls back to offline if online unreachable
}

// Check container certification
isCertified := validator.IsContainerCertified(registry, repository, tag, digest)

// Check operator certification
isCertified := validator.IsOperatorCertified(csvName, ocpVersion)

// Check helm chart certification
isCertified := validator.IsHelmChartCertified(helmRelease, kubeVersion)
```

## License

This project is licensed under the GNU General Public License v2.0.
