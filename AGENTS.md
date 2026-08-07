# Cluster Debug Tools - AI Agent Development Guide

This comprehensive guide provides critical context for AI agents working with the cluster-debug-tools codebase, covering architecture, workflows, and common pitfalls.

## Quick Reference

**Essential Commands:**
- Build: `make build`
- Test: `make test-unit`
- Verify: `make verify` (includes `verify-gofmt`, `verify-govet`, `verify-golang-versions`)
- Format: `make update-gofmt`

**Additional Documentation:**
- [ARCHITECTURE.md](./ARCHITECTURE.md) - System design and data flows
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Development workflow and conventions
- [README.md](./README.md) - Quick start guide

## Overview

Cluster Debug Tools (`kubectl-dev_tool`) is a collection of CLI debugging and inspection utilities for OpenShift developer teams. It is designed as a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) that integrates directly into `oc` or `kubectl` workflows. The tool provides post-hoc debugging capabilities for OpenShift CI test runs and cluster state, including audit log inspection, event analysis, TLS certificate examination, pod security compliance checking, CI artifact downloading, and e2e test result analysis.

The project is maintained by the OpenShift Control Plane team and lives at `github.com/openshift/cluster-debug-tools`.

## Architecture Patterns

### Cobra CLI Pattern

All commands follow the standard kubectl/cobra three-phase execution pattern:

```go
type CommandOptions struct {
    builderFlags *genericclioptions.ResourceBuilderFlags
    configFlags  *genericclioptions.ConfigFlags
    genericclioptions.IOStreams
    // command-specific fields
}

func NewCmdX(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
    o := NewCommandOptions(streams)
    cmd := &cobra.Command{
        RunE: func(c *cobra.Command, args []string) error {
            if err := o.Complete(c, args); err != nil { return err }
            if err := o.Validate(); err != nil { return err }
            return o.Run()
        },
    }
    // bind flags
    return cmd
}
```

- **Complete**: Resolve flags, build resource finders, prepare runtime state
- **Validate**: Check flag combinations and required inputs
- **Run**: Execute the command logic

### kubectl Resource Builder Pattern

Commands that read Kubernetes resources (`event`, `revision-status`, `certs inspect-certs`, `certs locate-incluster-certs`) use `genericclioptions.ResourceBuilderFlags` and the kubectl visitor pattern. This gives automatic support for:
- `--local -f <file>` for local file analysis
- `--namespace` / `--all-namespaces` for cluster queries
- `--selector` for label-based filtering
- Standard kubeconfig and context flags

### Filter Chain Pattern

Both `audit` and `events` use composable filter chains accumulated based on CLI flags, but with different interfaces:

- **audit**: `EventFilterPredicate` interface with `Matches(*auditv1.Event) bool`. Composed into `AuditFilters` slice, applied sequentially via `FilterEvents()`.
- **events**: `EventFilter` interface with `FilterEvents(events ...*corev1.Event) []*corev1.Event`. Each filter returns a new filtered slice. Composed into `EventFilters` slice.

### Negation/Wildcard Pattern

The shared `pkg/util/filtering.AcceptString()` function supports:
- Exact match: `--resource=pods`
- Prefix wildcard: `--user=system:service*`
- Negation: `--namespace=-kube-system`
- Negation with wildcard: `--resource=-subjectaccessreviews.*`
- All-negative-means-accept-all semantics

## Code Organization

```
cluster-debug-tools/
├── cmd/kubectl-dev_tool/           # Entrypoint - assembles all subcommands
│   └── main.go
├── pkg/
│   ├── cmd/                        # One subpackage per CLI subcommand
│   │   ├── revision_status.go      # revision-status command (root pkg)
│   │   ├── audit/                  # Audit log inspection
│   │   ├── events/                 # Kubernetes event analysis
│   │   ├── certinspection/         # Simple cert inspection
│   │   ├── locateinclustercerts/   # Deep in-cluster cert discovery
│   │   ├── certs/                  # Parent "certs" command (groups cert subcommands)
│   │   ├── psa/                    # Pod Security Admission checking
│   │   ├── analyze-e2e/            # E2E test artifact analysis
│   │   ├── download/               # GCS artifact downloading
│   │   └── api-request-count/      # API request dashboard (NOT registered in main.go)
│   ├── tools/                      # GCS client utilities
│   │   ├── gcs.go                  # Google Cloud Storage client
│   │   └── tools.go                # Build tool dependency import
│   └── util/                       # Shared utilities
│       ├── filtering.go            # String matching with negation/wildcard
│       └── writer.go               # Multi-source file writer
├── web/event-filter/               # Client-side HTML event viewer (jqGrid)
├── vendor/                         # Vendored dependencies
├── Makefile                        # Build system (delegates to build-machinery-go)
├── OWNERS                          # Prow ownership
└── OWNERS_ALIASES                  # control-plane-approvers alias
```

## CLI Commands Reference

| Command | Package | Description | Requires Cluster |
|---------|---------|-------------|:---:|
| `audit` | `pkg/cmd/audit/` | Inspect API server audit logs from CI runs. Rich filtering by verb, resource, namespace, user, HTTP status, time ranges. Output: plain, wide, JSON, top-N, stats, latency-dist (gnuplot). | No |
| `event` | `pkg/cmd/events/` | Analyze Kubernetes event log files. Filter by namespace, kind, reason, component, `--around HH:MM[:SS]` time window (default 10min radius). Sort by time or frequency. Output: plain, wide (identical to plain), JSON, components list. | No |
| `certs inspect-certs` | `pkg/cmd/certinspection/` | Inspect certs, keys, and CA bundles in Kubernetes resources (Secrets, ConfigMaps, CSRs). Works locally or against a live cluster. | Optional |
| `certs locate-incluster-certs` | `pkg/cmd/locateinclustercerts/` | Deep in-cluster certificate discovery and mapping. Outputs JSON, DOT (Graphviz), or full documentation with SVG/PNG/markdown. | Yes |
| `psa-check` | `pkg/cmd/psa/` | Check namespaces for pod security admission violations via server-side dry-run. Supports `--level`, `--all-namespaces`, JSON/YAML output. | Yes |
| `revision-status` | `pkg/cmd/` | Count failed installer pods and current static pod revisions by reading `revision-status-*` ConfigMaps. | Yes |
| `analyze-e2e` | `pkg/cmd/analyze-e2e/` | Fetch and analyze e2e-aws CI artifacts (ClusterOperator conditions, Pod statuses, container restart details). Takes URL as positional argument. Uses direct HTTP (not GCS SDK). | No |
| `download` | `pkg/cmd/download/` | Download artifacts from Prow CI or GCS URLs. Takes URL as positional argument. Supports `--regex` filtering and tar/gzip extraction. Uses GCS SDK. | No |

**Note**: The `api-request-count` command exists in code (`pkg/cmd/api-request-count/`) but is **NOT registered** in `main.go`.

## Common Workflows

### Adding a New CLI Command

1. Create a new package under `pkg/cmd/<command-name>/`
2. Define an `Options` struct embedding `genericclioptions.IOStreams`
3. Implement `NewCmd<Name>()` returning `*cobra.Command` with Complete/Validate/Run
4. Register in `cmd/kubectl-dev_tool/main.go` via `cmd.AddCommand()`
5. Add flags using `cmd.Flags()` or `genericclioptions.ResourceBuilderFlags`

### Adding a New Filter to audit or event Commands

1. Define a new filter predicate function matching the existing filter interface
2. Add a CLI flag in the command's flag binding section
3. Accumulate the filter in `Complete()` if the flag is set
4. The filter is automatically applied during event processing

### Modifying GCS Download Logic

1. Edit `pkg/tools/gcs.go`
2. Update URL regex patterns if CI infrastructure URLs change
3. Add new archive format handling in `SaveArtifacts()` if needed
4. GCS access is unauthenticated -- no credentials required

### Adding Tests

1. Create `*_test.go` files in the same package as the code under test
2. Use table-driven tests with `t.Run()` for multiple scenarios
3. Run with `make test-unit`
4. Currently only `pkg/cmd/psa/parser_test.go` exists -- test coverage is minimal

## Development Rules

### Always Do

1. Follow the cobra Complete/Validate/Run pattern for new commands
2. Embed `genericclioptions.IOStreams` for testable I/O (write to `o.Out`/`o.ErrOut`, not `os.Stdout`/`os.Stderr`)
3. Use `genericclioptions.ResourceBuilderFlags` for commands reading Kubernetes resources
4. Use `pkg/util/filtering.AcceptString()` for user-facing string filtering with negation/wildcard support
5. Handle `gzip` compressed files in audit/event log processing
6. Return errors up the call chain to cobra (don't silently swallow errors)
7. Run `make verify` before submitting changes
8. Use `sync.WaitGroup` and `sync.Mutex` for concurrent operations (see `pkg/tools/gcs.go` and audit `io.go`)
9. Support both Prow CI URLs and direct GCS URLs for artifact commands

### Never Do

1. Never use `os.Stdout`/`os.Stderr` directly -- use `IOStreams` for testability
2. Never hard-code GCS bucket names -- use constants and URL parsing
3. Never skip error returns from I/O operations
4. Never use `panic()` in new code (some legacy code uses it -- do not extend the pattern)
5. Never add authenticated GCS access -- the tool uses unauthenticated public bucket access
6. Never edit files under `vendor/` directly -- use `go mod` and `go mod vendor`
7. Never modify `build-machinery-go` includes -- the Makefile delegates entirely to it
8. Never introduce interactive prompts -- this is a non-interactive CLI tool
9. Never add dependencies without checking if existing vendored libraries suffice

### Common Agent Mistakes

- Don't suggest using `os.Exit()` in command handlers -- return errors to cobra instead
- Don't create new utility packages without checking if `pkg/util/` already has what you need
- Don't propose new output formats without matching existing patterns (plain, wide, JSON)
- Don't forget that `audit` and `event` commands work on local files, not live clusters
- Don't assume the `api-request-count` command is active -- it's not registered in `main.go`
- Don't use `ioutil.*` functions -- they are deprecated (though legacy code uses them)
- Don't create Dockerfile or CI configuration files -- CI is configured externally in the `openshift/release` repository

## Tech Stack

- **Go 1.22+** (toolchain go1.22.7)
- **spf13/cobra** + **spf13/pflag**: CLI framework and flag parsing
- **k8s.io/cli-runtime**: kubectl-style resource builder, config flags, IOStreams
- **k8s.io/client-go**: Kubernetes client for live cluster commands
- **k8s.io/api** + **k8s.io/apimachinery**: Core Kubernetes types (audit events, pods, configmaps)
- **k8s.io/apiserver**: Audit event types
- **k8s.io/pod-security-admission**: PSA policy types
- **openshift/api**: OpenShift-specific types (e.g., `APIRequestCount`)
- **openshift/library-go**: Certificate inspection utilities
- **openshift/build-machinery-go**: Build system framework (Makefile targets)
- **cloud.google.com/go/storage**: GCS SDK for artifact downloads
- **gonum/graph**: Graph library for cert relationship rendering (DOT/SVG)
- **ghodss/yaml**: YAML parsing
- **klog/v2**: Structured logging

## Key Constants and URL Patterns

```go
// GCS bucket for CI artifacts
const defaultBucket = "origin-ci-test"

// Supported CI URL patterns (pkg/tools/gcs.go)
prowBaseURL = `prow\.ci\.openshift\.org`
gcsBaseURL  = `gcsweb-ci\.apps\.ci\.l2s4\.p1\.openshiftapps\.com`
```

## Security Notes

- GCS access is explicitly unauthenticated (`option.WithoutAuthentication()`)
- `analyze-e2e` uses `InsecureSkipVerify: true` for TLS when fetching artifacts via HTTP -- this is a known issue in the code
- The tool reads cluster resources but does not modify cluster state (except `psa-check` which uses server-side dry-run)
- No secrets or credentials are stored or transmitted
- Certificate inspection reads but does not export private key material
- File write operations in `download` and `analyze-e2e` create files with `0755` permissions
- Some production code uses `panic()` (audit URI parsing, cert DOT encoding, certgraph DeepCopy) -- do not extend this pattern

## Testing

Testing coverage is minimal. Only one test file exists:
- `pkg/cmd/psa/parser_test.go` -- Tests `parseWarnings()` for pod security violation parsing (4 test cases)

Run tests with:
```bash
make test-unit
```

The build machinery provides JUnit output support via `gotest2junit` for CI integration.

---

**Document Version**: 1.0
**Last Updated**: 2026-08-07
**Maintained By**: OpenShift Control Plane Team
