# Cluster Debug Tools - Architecture

This document explains the architecture, design decisions, and operational model of cluster-debug-tools.

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Component Architecture](#component-architecture)
- [Command Architecture](#command-architecture)
- [Data Flow](#data-flow)
- [GCS Artifact Pipeline](#gcs-artifact-pipeline)
- [Filter System](#filter-system)
- [Certificate Inspection System](#certificate-inspection-system)
- [Web Components](#web-components)
- [Build System](#build-system)
- [Design Decisions](#design-decisions)

## Overview

### What is cluster-debug-tools?

**cluster-debug-tools** is a kubectl plugin (`kubectl-dev_tool`) that bundles debugging and inspection utilities for OpenShift developer teams. It provides post-hoc analysis of CI test runs, cluster state inspection, and certificate debugging capabilities.

**Key Capabilities:**
- **Audit Log Analysis**: Parse and filter API server audit logs from CI runs with rich querying
- **Event Analysis**: Inspect Kubernetes event logs with filtering, sorting, and multiple output formats
- **Certificate Inspection**: Examine TLS certificates, keys, and CA bundles in cluster resources and on-disk locations
- **Pod Security Checking**: Detect pod security admission violations across namespaces via dry-run
- **CI Artifact Download**: Fetch artifacts from Prow CI or GCS with regex filtering and archive extraction
- **E2E Analysis**: Analyze ClusterOperator and Pod artifacts from e2e-aws test runs
- **Revision Status**: Track static pod revision status and installer pod failures

### What cluster-debug-tools is NOT

- **Not a controller or operator**: It is a stateless CLI tool with no reconciliation loops
- **Not a cluster modifier**: All commands are read-only (except `psa-check` which uses dry-run)
- **Not a stable API**: The tool is provided with no guarantees and may change at any time
- **Not an authenticated service**: GCS access is explicitly unauthenticated

## System Architecture

### High-Level Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                        kubectl-dev_tool Binary                         │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │  Root Command: openshift-dev-helpers                             │  │
│  │                                                                    │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌─────────────────┐  │  │
│  │  │   audit   │ │   event   │ │ psa-check │ │ revision-status │  │  │
│  │  └───────────┘ └───────────┘ └───────────┘ └─────────────────┘  │  │
│  │                                                                    │  │
│  │  ┌───────────┐ ┌──────────────┐ ┌──────────┐                    │  │
│  │  │  certs    │ │  analyze-e2e │ │ download │                    │  │
│  │  │ ┌────────┐│ └──────────────┘ └──────────┘                    │  │
│  │  │ │inspect ││                                                    │  │
│  │  │ │-certs  ││                                                    │  │
│  │  │ ├────────┤│                                                    │  │
│  │  │ │locate- ││                                                    │  │
│  │  │ │certs   ││                                                    │  │
│  │  │ └────────┘│                                                    │  │
│  │  └───────────┘                                                    │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────────────┐    │
│  │  pkg/tools/  │  │    pkg/util/     │  │    vendor libs       │    │
│  │  GCS Client  │  │  Filtering       │  │  cobra, cli-runtime  │    │
│  │              │  │  Writer           │  │  client-go, klog     │    │
│  └──────────────┘  └──────────────────┘  └──────────────────────┘    │
└────────────────────────────────────────────────────────────────────────┘
        │                    │                        │
        ▼                    ▼                        ▼
 ┌──────────────┐   ┌──────────────┐         ┌──────────────┐
 │  Google      │   │  Local       │         │  Kubernetes  │
 │  Cloud       │   │  Files       │         │  API Server  │
 │  Storage     │   │  (audit,     │         │  (live       │
 │  (artifacts) │   │   events)    │         │   cluster)   │
 └──────────────┘   └──────────────┘         └──────────────┘
```

### Command Categories

Commands fall into three categories based on their data source:

**File-based (offline analysis):**
- `audit` -- reads audit log files (plain or gzip)
- `event` -- reads event JSON files
- `download` -- fetches files from GCS to local disk

**Cluster-based (live cluster required):**
- `psa-check` -- queries namespaces via API server dry-run
- `revision-status` -- reads ConfigMaps from a live cluster
- `certs locate-incluster-certs` -- gathers certs from cluster resources and node filesystems

**Hybrid (local or cluster):**
- `certs inspect-certs` -- works with `--local -f <file>` or against a live cluster
- `analyze-e2e` -- fetches from GCS, then analyzes locally

## Component Architecture

### Entry Point

**File**: `cmd/kubectl-dev_tool/main.go`

```
main()
  │
  ├── Create pflag.FlagSet
  ├── Create root cobra.Command via NewCmdDevHelpers()
  │     │
  │     ├── AddCommand(psa.NewCmdPSA)
  │     ├── AddCommand(events.NewCmdEvent)
  │     ├── AddCommand(audit.NewCmdAudit)
  │     ├── AddCommand(mustgather.NewCmdRevisionStatus)
  │     ├── AddCommand(certs.NewCmdCerts)
  │     │     ├── AddCommand(certinspection.NewCmdCertInspection)
  │     │     └── AddCommand(locateinclustercerts.NewCmdLocateInClusterCerts)
  │     ├── AddCommand(analyze_e2e.NewCmdAnalyze)
  │     └── AddCommand(download.NewDownloadCommand)
  │
  └── Execute root command
```

The root command (`openshift-dev-helpers`) has no-op Complete/Validate/Run methods -- it exists only to group subcommands.

### Shared Infrastructure

#### GCS Client (`pkg/tools/gcs.go`)

Provides unauthenticated Google Cloud Storage access for downloading CI artifacts:

```
GCSClient
  ├── FetchArtifacts(link, filter)    -- List + download matching objects concurrently
  ├── SaveArtifacts(dirName, data)    -- Write to disk, extract tar/gzip archives
  ├── FetchAndSaveArtifacts(...)      -- Combined fetch + save
  └── Helper functions
       ├── IsGCSLink(link)            -- Validate URL against known CI patterns
       ├── getBucketAndObject(link)   -- Parse bucket and path from URL
       └── stripJobPrefix(path)       -- Remove job-specific path prefix
```

**Supported URL Patterns:**
- `https://prow.ci.openshift.org/view/gs/<bucket>/<path>`
- `https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/<bucket>/<path>`

#### Filtering (`pkg/util/filtering.go`)

Single function `AcceptString(allowedValues, currValue)` implementing set-based string matching:

```
AcceptString evaluation order:
  1. Check exact negation: "-value" in allowed → reject
  2. Check negation wildcard: "-prefix*" matching → reject
  3. If all values are negations → accept (implicit allow-all)
  4. Check exact match: "value" in allowed → accept
  5. Check positive wildcard: "prefix*" matching → accept
  6. Default → reject
```

#### File Writer (`pkg/util/writer.go`)

`MultiSourceFileWriter` abstraction for writing Kubernetes objects or raw text to files:

```
MultiSourceFileWriter
  ├── WriteFromSource(filepath, source)     -- Write from io.ReadCloser stream
  └── WriteFromResource(filepath, object)   -- Serialize Kubernetes object via ResourcePrinter
```

## Command Architecture

### audit Command (`pkg/cmd/audit/`)

The most complex command. Processes API server audit logs with rich filtering and multiple output modes.

**Data Flow:**
```
CLI Flags → Complete() → Build Filter Chain
                              │
  Audit Log Files ────────────┤
  (plain or .gz)              │
                              ▼
                    Read Events (concurrent goroutines)
                              │
                              ▼
                    Apply Filter Chain
                              │
                              ▼
                    Format Output
                    ├── plain/wide (tabwriter)
                    ├── json (full event dump)
                    ├── top-by-* (aggregation + ranking)
                    ├── stats (latency percentiles)
                    └── latency-dist (gnuplot data)
```

**Filter Dimensions:**
- Verb (get, list, watch, create, update, delete, patch)
- Resource / Subresource
- Namespace / Name
- User / Group
- HTTP status codes (--failed-only for 4xx/5xx)
- Stage (RequestReceived, ResponseStarted, ResponseComplete, Panic)
- Time ranges (--after, --before, --around)
- Duration (--min-duration)
- Field manager
- UID
- Pod security violations

**Concurrency Model:**
When processing directories, `io.go` spawns goroutines per file with `sync.WaitGroup` for coordination and `sync.Mutex` for safe event list accumulation.

### event Command (`pkg/cmd/events/`)

Processes Kubernetes event logs from CI artifacts.

**Filter Dimensions:**
- Namespace, Name, Kind, Reason
- Component (source component)
- UID
- Time window (`--around` with configurable minutes)
- Warnings-only mode

**Output Modes:**
- Plain text
- Wide (additional columns)
- JSON (raw events)
- Component list

### certs Commands

**`inspect-certs`** (`pkg/cmd/certinspection/`):
- Uses `openshift/library-go` cert inspection utilities
- Processes Secrets, ConfigMaps, and CSRs
- Extracts cert metadata (issuer, subject, validity, SANs)

**`locate-incluster-certs`** (`pkg/cmd/locateinclustercerts/`):
- Deep certificate discovery from both API resources and on-disk filesystem paths
- Builds a graph of certificate relationships using `gonum/graph`
- Maps known OpenShift cert locations to human-readable logical names
- Output formats: JSON, DOT (Graphviz), documentation (SVG/PNG + markdown)
- Contains a `Dockerfile` for building a container to collect certs from cluster nodes

### psa-check Command (`pkg/cmd/psa/`)

```
psa-check Flow:
  1. Enumerate target namespaces (or --all-namespaces)
  2. For each namespace:
     a. Perform server-side dry-run update with stricter PSA level
     b. Collect API server warnings from response headers
     c. Parse warnings into structured violation data
  3. Output violations (plain, JSON, YAML, quiet mode)
```

This is the only command that makes mutating API calls (dry-run only, no actual changes).

### download Command (`pkg/cmd/download/`)

```
download Flow:
  1. Parse Prow CI or GCS URL
  2. List objects in GCS bucket matching path prefix
  3. Filter objects by user-provided regex
  4. Download matching objects concurrently
  5. Save to local directory (extract tar/gzip if applicable)
```

### analyze-e2e Command (`pkg/cmd/analyze-e2e/`)

**Note**: Unlike the `download` command which uses the GCS SDK, `analyze-e2e` fetches artifacts via **direct HTTP** from `storage.googleapis.com`. It converts `gcsweb-ci.svc.ci.openshift.org/gcs` URLs to `storage.googleapis.com` URLs. It also uses `InsecureSkipVerify: true` for TLS connections.

The `Analyzer` interface allows pluggable analysis of different artifact types:
```go
type Analyzer interface {
    Analyze(content []byte) (string, error)
}
```

Currently registered analyzers:
- `clusteroperators.json` -> `ClusterOperatorsAnalyzer`
- `pods.json` -> `PodsAnalyzer`

```
analyze-e2e Flow:
  1. Take GCS URL as positional argument
  2. Convert URL to storage.googleapis.com format
  3. Fetch clusteroperators.json and pods.json via HTTP
  4. ClusterOperatorsAnalyzer: Parse conditions (Available, Degraded, Progressing, etc.)
  5. PodsAnalyzer: Parse conditions + container statuses (restart counts, exit codes, termination messages)
  6. Sort and output summary
```

### revision-status Command (`pkg/cmd/revision_status.go`)

```
revision-status Flow:
  1. Use ResourceBuilder to visit ConfigMaps in target namespace
  2. Filter for "revision-status-*" prefix
  3. Classify by status: Succeeded, Failed, InProgress, Unknown
  4. Output table via tabwriter
```

## GCS Artifact Pipeline

```
┌───────────────────────────────────────────────────────────────────┐
│                     GCS Artifact Pipeline                          │
│                                                                     │
│  Prow CI URL / GCS URL                                             │
│  ────────────────────                                              │
│  https://prow.ci.openshift.org/view/gs/origin-ci-test/...         │
│  https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/...     │
│         │                                                           │
│         ▼                                                           │
│  ┌─────────────────┐                                               │
│  │  URL Parsing     │  linkURLRegex extracts bucket + path         │
│  │  (IsGCSLink,     │  Default bucket: origin-ci-test              │
│  │   getBucket...)   │                                              │
│  └────────┬────────┘                                               │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────┐                                               │
│  │  GCS List        │  List objects under prefix                   │
│  │  (10s timeout)   │  Filter by user regex                        │
│  └────────┬────────┘                                               │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────┐                                               │
│  │  Concurrent      │  One goroutine per matching object           │
│  │  Download         │  sync.WaitGroup + sync.Mutex coordination   │
│  │  (unauthed)       │                                              │
│  └────────┬────────┘                                               │
│           │                                                         │
│           ▼                                                         │
│  ┌─────────────────┐                                               │
│  │  Save to Disk    │  .tar files: extract gzip + tar entries      │
│  │  (with extract)  │  Other files: write directly                 │
│  └─────────────────┘                                               │
└───────────────────────────────────────────────────────────────────┘
```

## Filter System

### audit Filter Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Audit Filter Pipeline                         │
│                                                                   │
│  Raw Audit Events (from file/gzip)                               │
│         │                                                         │
│         ▼                                                         │
│  ┌─────────────────┐                                             │
│  │ Verb Filter      │  --verb=get,list,watch                     │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Resource Filter  │  --resource=pods --subresource=status      │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Namespace Filter │  --namespace=openshift-* (wildcard)        │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ User Filter      │  --user=system:serviceaccount:*            │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Status Filter    │  --status=404,500 / --failed-only          │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Time Filter      │  --after / --before / --around             │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Stage Filter     │  --stage=ResponseComplete                  │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  ┌─────────────────┐                                             │
│  │ Duration Filter  │  --min-duration=1s                         │
│  └────────┬────────┘                                             │
│           ▼                                                       │
│  Filtered Events → Output Formatter                              │
└─────────────────────────────────────────────────────────────────┘
```

### AcceptString Matching Logic

```
Input: allowedValues = {"pods", "-kube-system", "system:service*"}
       currValue = "system:serviceaccount:default:builder"

Step 1: Check "-system:serviceaccount:default:builder" in set → No
Step 2: Check negation wildcards → No match
Step 3: All values negative? → No (has "pods" and "system:service*")
Step 4: Exact match "system:serviceaccount:default:builder"? → No
Step 5: Wildcard "system:service*" matches prefix → Yes, ACCEPT
```

## Certificate Inspection System

### inspect-certs Flow

```
┌──────────────────────────────────────────────────────────┐
│                  inspect-certs                            │
│                                                            │
│  Input: Kubernetes Secrets, ConfigMaps, CSRs              │
│  Source: --local -f <file> OR live cluster                 │
│         │                                                  │
│         ▼                                                  │
│  ┌─────────────────┐                                      │
│  │ ResourceBuilder  │  kubectl visitor pattern             │
│  │ Visitor Pattern  │                                      │
│  └────────┬────────┘                                      │
│           │                                                │
│           ▼                                                │
│  ┌─────────────────┐                                      │
│  │ library-go       │  openshift/library-go cert           │
│  │ CertInspection   │  inspection utilities                │
│  └────────┬────────┘                                      │
│           │                                                │
│           ▼                                                │
│  Output: Cert metadata (issuer, subject, SANs, validity)  │
└──────────────────────────────────────────────────────────┘
```

### locate-incluster-certs Flow

```
┌──────────────────────────────────────────────────────────────┐
│              locate-incluster-certs                           │
│                                                                │
│  ┌──────────────┐   ┌──────────────────┐                     │
│  │ Kubernetes    │   │ On-Disk          │                     │
│  │ Resources     │   │ Filesystem       │                     │
│  │ (Secrets,     │   │ (known OpenShift │                     │
│  │  ConfigMaps)  │   │  cert locations) │                     │
│  └──────┬───────┘   └────────┬─────────┘                     │
│         │                    │                                 │
│         └────────┬───────────┘                                 │
│                  ▼                                              │
│         ┌────────────────┐                                     │
│         │ Cert Discovery │  Parse PEM, extract metadata        │
│         └────────┬───────┘                                     │
│                  ▼                                              │
│         ┌────────────────┐                                     │
│         │ Graph Building │  gonum/graph for relationships      │
│         │ (cert → CA,    │  Map to logical names               │
│         │  issuer chains)│                                      │
│         └────────┬───────┘                                     │
│                  ▼                                              │
│         ┌────────────────┐                                     │
│         │ Output         │                                     │
│         │ ├── JSON       │                                     │
│         │ ├── DOT        │  Graphviz format                    │
│         │ └── Docs       │  SVG/PNG + markdown                 │
│         └────────────────┘                                     │
└──────────────────────────────────────────────────────────────┘
```

## Web Components

### Event Filter HTML Pages

**Location**: `web/event-filter/`

Three standalone HTML files providing browser-based event log viewing:

```
┌─────────────────────────────────────────────────────────┐
│              Event Filter Web UI                         │
│              (client-side only, no server)                │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ event-filter-file-select.html                       │ │
│  │   User selects local JSON file via <input type=file>│ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ event-filter-local-ajax.html                        │ │
│  │   Loads events via AJAX from local URL              │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ event-filter-local-jsonstr.html                     │ │
│  │   Loads events from inline JSON string              │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  Tech: jQuery + jqGrid                                   │
│  Features: Column search, filtering by Type/Component/   │
│            Reason/Message/timestamps/involvedObject/Count │
└─────────────────────────────────────────────────────────┘
```

## Build System

### Makefile Structure

```
Makefile (7 lines)
  │
  └── includes vendor/github.com/openshift/build-machinery-go/make/golang.mk
       │
       └── Provides auto-discovered targets:
            ├── build          -- Compile all Go binaries (auto-discovers cmd/)
            ├── test-unit      -- Run go test on all packages
            ├── verify         -- Run verify-gofmt, verify-govet, verify-golang-versions
            ├── verify-gofmt   -- Check Go formatting
            ├── verify-govet   -- Run go vet
            ├── update-gofmt   -- Auto-format Go source
            ├── clean          -- Remove compiled binaries
            └── vulncheck      -- Run vulnerability scan
```

The build machinery auto-discovers `cmd/kubectl-dev_tool/main.go` as the build package and produces a binary named `kubectl-dev_tool`.

### Dependency Management

```
go.mod (Go 1.22.0, toolchain go1.22.7)
  │
  ├── Direct dependencies managed via go.mod
  ├── Vendored via go mod vendor
  └── vendor/modules.txt tracks vendored versions
```

## Design Decisions

### Why a Single Binary with Multiple Subcommands?

**Decision**: Bundle all tools into one `kubectl-dev_tool` binary.

**Rationale:**
- **kubectl Plugin**: Single binary installs as one kubectl plugin
- **Shared Dependencies**: Avoids duplicating vendored libraries across multiple binaries
- **Discoverability**: All debugging tools available from one entry point
- **Simplicity**: One build target, one install, one binary to distribute

**Trade-offs:**
- Binary size includes all command dependencies even if only one is used
- All commands share the same Go module dependency versions

### Why Flat Command Architecture?

**Decision**: Each command is self-contained in its own package under `pkg/cmd/`. No shared data flows between commands.

**Rationale:**
- **Independence**: Commands solve unrelated problems (audit logs vs certificates vs PSA)
- **Maintainability**: Changes to one command cannot break another
- **Contributor Friendliness**: Easy to understand and modify one command without knowing others

**Trade-offs:**
- Some code duplication across commands (e.g., output formatting)
- Shared utilities limited to `pkg/util/` and `pkg/tools/`

### Why Unauthenticated GCS Access?

**Decision**: All GCS access uses `option.WithoutAuthentication()`.

**Rationale:**
- **CI artifacts are public**: OpenShift CI artifacts in `origin-ci-test` bucket are publicly readable
- **Zero configuration**: No credentials, service accounts, or environment variables needed
- **Simplicity**: Users can download artifacts without any GCS setup

**Trade-offs:**
- Cannot access private GCS buckets
- No rate limiting or quota management

### Why build-machinery-go?

**Decision**: Use OpenShift's `build-machinery-go` for the build system instead of custom Makefile targets.

**Rationale:**
- **Consistency**: Matches other OpenShift repositories
- **Convention**: Standard targets (`build`, `test-unit`, `verify`) work the same everywhere
- **Maintenance**: Build infrastructure maintained centrally by OpenShift team

**Trade-offs:**
- Implicit targets can be confusing (Makefile is only 7 lines)
- Build behavior depends on the vendored `build-machinery-go` version

### Why kubectl cli-runtime?

**Decision**: Use `k8s.io/cli-runtime` for resource access patterns instead of raw `client-go`.

**Rationale:**
- **kubectl Compatibility**: Supports all standard kubectl flags (`--namespace`, `--selector`, `--local -f`)
- **Visitor Pattern**: Clean separation between resource discovery and processing
- **Hybrid Mode**: Same code works with local files and live cluster resources

**Trade-offs:**
- More complex than direct API calls for simple queries
- ResourceBuilder API has a learning curve

### Why No Tests for Most Commands?

**Decision**: Minimal test coverage (only `psa/parser_test.go`).

**Rationale:**
- **Internal Tool**: Provided "with no guarantees" -- lower bar for test coverage
- **Output-Oriented**: Most commands format and display data, making unit testing less valuable than integration testing
- **Rapid Iteration**: Tool evolves quickly as debugging needs change

**Trade-offs:**
- Regressions can go unnoticed
- Refactoring is riskier without test coverage
- Contributing is harder without test examples

### Why Not Register api-request-count?

The `api-request-count` command (`pkg/cmd/api-request-count/`) is fully implemented but not registered in `main.go`. It generates static HTML dashboards from `APIRequestCount` resources using the Nivo charting library. It appears to have been an experiment or was superseded by other tools.

---

**Document Version**: 1.0
**Last Updated**: 2026-08-07
**Maintained By**: OpenShift Control Plane Team
