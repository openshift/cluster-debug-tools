# Contributing to Cluster Debug Tools

This document serves as a guide for contributing to the cluster-debug-tools repository, which is maintained by the OpenShift Control Plane group.

This document is explicitly for contributions to the cluster-debug-tools repository and not for high-level feature proposals within OpenShift.

Feature proposals should follow the OpenShift Enhancement Proposal process outlined in https://github.com/openshift/enhancements/blob/master/dev-guide/feature-zero-to-hero.md#openshift-feature-development-zero-to-hero-guide.
If you are looking for a review on an OpenShift Enhancement Proposal that involves changes to cluster-debug-tools, please request a review in the [`#forum-ocp-apiserver`](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) Slack channel.

## Table of Contents

- [Code Conventions](#code-conventions)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process and Guidelines](#pull-request-process-and-guidelines)
- [Review Expectations](#review-expectations)
- [Local Development Guide](#local-development-guide)

---

## Code Conventions

We largely follow the [Kubernetes Code Conventions](https://github.com/kubernetes/community/blob/main/contributors/guide/coding-conventions.md#code-conventions).

Review both the Kubernetes Code Conventions and the ones specified here. If any conventions are at odds with one another, prefer the conventions explicitly documented here.

### Bash

- Follow the [shell styleguide](https://google.github.io/styleguide/shellguide.html).
- Use [`shellcheck`](https://github.com/koalaman/shellcheck) to identify common mistakes or caveats.
- Ensure that all scripts run consistently across Linux and MacOS.

### Golang (Go)

- Review [Effective Go](https://go.dev/doc/effective_go).
- Review common [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Review and avoid [Go Landmines](https://gist.github.com/lavalamp/4bd23295a9f32706a48f)
- Comment your code following the [Go comment conventions](https://go.dev/doc/comment).
    - Comments should be meaningful and add context and/or explain choices that cannot be expressed through clear code.
    - All exported types, functions, and methods must have descriptive comments.
    - All unexported types, functions, and methods should have descriptive comments.
- When adding command-line flags, use dashes/hyphens (`-`) and not underscores (`_`).
- Naming
    - Please consider package name when selecting an interface name, and avoid redundancy. For example, `storage.Interface` is better than `storage.StorageInterface`.
    - Do not use uppercase characters, underscores, or dashes in package names.
    - Please consider parent directory name when choosing a package name. For example, `pkg/cmd/audit/audit.go` should say `package audit` not `package auditcmd`.
        - Unless there's a good reason, the package foo line should match the name of the directory in which the .go file exists.
        - Importers can use a different name if they need to disambiguate.
    - Locks should be called `lock` and should never be embedded (always `lock sync.Mutex`). When multiple locks are present, give each lock a distinct name following Go conventions: `stateLock`, `mapLock` etc.
- Error handling
    - Wrap errors with meaningful context before returning or logging them.
    - Return errors to cobra -- do not call `os.Exit()` from command handlers.
- When logging, follow the [Kubernetes Logging Conventions](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-instrumentation/logging.md).
- Use `genericclioptions.IOStreams` for all output -- write to `o.Out`/`o.ErrOut`, not `os.Stdout`/`os.Stderr` directly.

### General

Regardless of the programming language, make sure to take the following into consideration:
- Keep readability / maintainability in mind when writing code.
    - Clever code and abstractions are often harder to reason about after the fact. Keep clever code and abstractions to the minimum necessary to accomplish the end-goal.
- Do not reinvent the wheel. Where possible, use existing standard library or vendored library functionality. If you are adding a net-new dependency, stop and think if you _really_ need to add the new dependency to achieve your goals.
- When writing tests, focus on testing the functional behaviors your code exercises. Avoid writing tests that are testing that the standard library works as expected or is trivial. Do not write tests just for line coverage.

### Directory and File Conventions

- Avoid package sprawl. Find an appropriate subdirectory for new packages.
    - Libraries with no appropriate home belong in new package subdirectories of `pkg/util`.
- Avoid general utility packages. Packages called "util" are suspect. Instead, derive a name that describes your desired function. For example, the utility functions dealing with string filtering are in `pkg/util/filtering.go`.
- All filenames should be lowercase.
- Go source files and directories use underscores, not dashes.
    - Package directories should generally avoid using separators as much as possible. When package names are multiple words, they usually should be in nested subdirectories.

---

## Testing Guidelines

These are high-level testing guidelines. The cluster-debug-tools repository has additional testing specifics documented in the [Testing Your Changes](#testing-your-changes) section below.

- All changes should include unit test additions/changes where practical.
    - Exceptions are at reviewer/approver discretion.
- Table-driven unit tests are preferred for testing multiple scenarios/inputs. For an example, see `pkg/cmd/psa/parser_test.go`.
- Unit tests must pass on all platforms (at the very least, Linux + MacOS).
- Do not expect an asynchronous thing to happen immediately. Do not wait for one second and expect a pod to be running. Wait and retry instead.

Currently, this repository has minimal test coverage. Only `pkg/cmd/psa/parser_test.go` exists. Adding tests to existing commands is welcome and encouraged.

```bash
# Run unit tests
make test-unit
```

---

## Pull Request Process and Guidelines

This section assumes that you have a functional understanding of `git` and how to create a pull request on GitHub.

If you do not, start with [GitHub's "Getting Started" guide](https://docs.github.com/en/get-started/start-your-journey).

### Prerequisites

Before you commit any changes or create any pull requests, you must adhere to OpenShift contribution policies.
Currently, that means enabling commit signature verification.

See https://docs.google.com/document/d/1184EPSGunUkcSQYUK8T4a6iyawwi6f2zxdbB2jtG9nQ/edit?usp=sharing for more details on how to adhere to the commit signature verification policy of OpenShift.

**To enable commit signing**:

```bash
git config --global user.signingkey <your-gpg-key-id>
git config --global commit.gpgsign true
```

### Creating a Pull Request

When creating a pull request, include the following:

- A brief, but descriptive, title.
    - All pull requests _should_ link to a Jira ticket associated with the work. There is automation that performs this linking when prefixing the title with the Jira ticket identifier like: `CNTRLPLANE-XXXX: my pull request title`. For pull requests that have no Jira ticket associated with it, you can prefix it with `NO-JIRA:` to signal that there is not a Jira ticket associated with it.
- A useful description of the changes being made and why they are important. Include links to supporting documents and any additional context that reviewers may need.

**Example PR Title**:
```
CNTRLPLANE-1234: add latency distribution output mode for audit command
```

or

```
NO-JIRA: fix typo in CONTRIBUTING.md
```

### CI / CD

For CI/CD, OpenShift uses Prow to run various checks. This can include unit tests, linters, etc.

The jobs configured for each repository are in https://github.com/openshift/release/tree/main/ci-operator/config/openshift . If you find yourself needing to add additional jobs, review the documentation at https://docs.ci.openshift.org/how-tos/contributing-openshift-release/ .

There are often a mixture of required and optional checks as well as merge criteria that must be met before a pull request can merge.
When any of these checks fail, the GitHub Prow bot will leave a comment on the PR with links to the run of that check that failed.

As the PR author, it is your responsibility to evaluate the failed checks and determine if there are any changes necessary to pass the checks.
If you suspect that the check failure was a flake, you can trigger retests by commenting `/retest` (or `/retest-required` for retesting only the required checks) on the PR.

**Common Prow Commands**:
- `/retest` - Rerun all failed tests
- `/retest-required` - Rerun only required failed tests
- `/test <job-name>` - Run a specific test job

### Verifying your changes

While there are a lot of scenarios where the existing CI/CD checks may be sufficient to verify your changes are working (and can be denoted by commenting `/verified by ci`),
there may be scenarios where manual verification is required.

For cluster-debug-tools, manual verification typically involves:
1. Building the binary locally (`make build`)
2. Running the relevant command against sample CI artifacts or a test cluster
3. Verifying the output is correct and no regressions are introduced

Once you've verified your changes work as expected, you can mark the PR as verified by commenting `/verified by @{your_github_handle}` on the PR.

### Additional Resources

For more information regarding more general OpenShift pull request processes, the following resources are helpful:

- https://docs.ci.openshift.org/architecture/jira
- https://docs.ci.openshift.org/
- https://steps.ci.openshift.org/

---

## Review Expectations

### Requesting a review

If you are not a member of the OpenShift control plane team and you need a review on a PR, post it in the [#forum-ocp-apiserver](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) Slack channel or reach out to folks outlined in the OWNERS file directly.

If you are a member of the OpenShift control plane team, reviews should come from your feature team. In the event your feature team does not have someone that can approve a PR, post it in the [#control-plane](https://redhat.enterprise.slack.com/archives/CC3CZCQHM) Slack channel.

OpenShift uses AI code review tools as part of the code review process.
Before requesting a review, address all feedback from the code review agent(s).
It is up to your discretion as the contributor how you would like to address that feedback.
Responding with an explanation as to why you are not going to take action on a comment made by the agent is an acceptable way to "address" its feedback.

### Interacting with reviewers

When interacting with reviewers/approvers:

- Be professional.
- Be respectful of differing opinions, viewpoints, and experiences.
- Gracefully give and receive constructive feedback.
- Focus on what is best for the product/organization, not just us as individuals.

A special note on the usage of AI - to respect the time of those that are reviewing your contribution, please do not use AI to respond to review comments.

---

## Local Development Guide

This section provides detailed guidance for local development, building, and debugging cluster-debug-tools.

### Prerequisites

#### Required Tools

- **Go 1.22+**: [Installation guide](https://go.dev/doc/install)
- **Make**: For build automation
- **Git**: For version control with commit signing enabled

#### Optional Tools (for specific commands)

- **oc CLI**: Required for commands that connect to a live cluster (`psa-check`, `revision-status`, `certs locate-incluster-certs`)
- **Access to an OpenShift cluster**: For live cluster commands
- **Graphviz**: For rendering DOT output from `certs locate-incluster-certs`

#### Verify Prerequisites

```bash
# Check Go version
go version  # Should be 1.22 or later

# Verify commit signing is enabled
git config --get user.signingkey
git config --get commit.gpgsign  # Should return 'true'

# Optional: Check cluster access (for live cluster commands)
oc version
oc whoami
```

### Getting Started

#### Fork and Clone

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/<your-username>/cluster-debug-tools.git
   cd cluster-debug-tools
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/openshift/cluster-debug-tools.git
   git fetch upstream
   ```

#### Understand the Codebase

Review the documentation:
- [AGENTS.md](./AGENTS.md) - Code patterns and conventions for AI agents
- [ARCHITECTURE.md](./ARCHITECTURE.md) - System design and architecture
- [README.md](./README.md) - Quick overview

Key directories:
```
cluster-debug-tools/
├── cmd/kubectl-dev_tool/    # Main entrypoint
├── pkg/cmd/                 # One subpackage per CLI command
├── pkg/tools/               # GCS client utilities
├── pkg/util/                # Shared utilities
├── web/event-filter/        # Browser-based event viewer
└── vendor/                  # Vendored dependencies
```

### Building

#### Build the Binary

```bash
# Build using the Makefile (uses build-machinery-go)
make build

# The binary is produced as kubectl-dev_tool
```

#### Install as kubectl Plugin

```bash
# Copy to a directory in your PATH
cp kubectl-dev_tool ${HOME}/bin/

# Or install directly via go
go install github.com/openshift/cluster-debug-tools/cmd/kubectl-dev_tool@latest
```

Once installed, the binary is available as:
```bash
# As a standalone binary
kubectl-dev_tool audit ...

# As a kubectl/oc plugin
oc dev_tool audit ...
kubectl dev_tool audit ...
```

#### Verify the Build

```bash
# Run verification checks
make verify

# This runs:
#   verify-gofmt    -- Check Go formatting
#   verify-govet    -- Run go vet
#   verify-golang-versions -- Check Go version compatibility
```

#### Format Code

```bash
# Auto-format Go source files
make update-gofmt
```

### Testing Your Changes

#### Unit Tests

```bash
# Run all unit tests
make test-unit

# Run tests for a specific package
go test ./pkg/cmd/psa/...

# Run with verbose output
go test -v ./pkg/...

# Run with coverage
go test -coverprofile=coverage.out ./pkg/... ./cmd/...
go tool cover -html=coverage.out
```

#### Manual Testing

Since cluster-debug-tools is primarily a CLI tool that processes files and cluster data, manual testing is often the most effective approach.

##### Testing audit Command

```bash
# Download sample audit logs from a CI run (URL is a positional argument)
kubectl-dev_tool download \
  "https://prow.ci.openshift.org/view/gs/origin-ci-test/<job-path>" \
  --regex="audit_logs" \
  --dir=/tmp/audit-test

# Test audit analysis
kubectl-dev_tool audit -f /tmp/audit-test/ \
  --verb=create,update \
  --namespace=openshift-kube-apiserver \
  --output=wide
```

##### Testing event Command

```bash
# Test with local event files
kubectl-dev_tool event -f /path/to/events.json \
  --warning-only \
  --component=kubelet
```

##### Testing psa-check Command (requires cluster)

```bash
# Check a specific namespace
kubectl-dev_tool psa-check -n <namespace> --level=restricted

# Check all namespaces
kubectl-dev_tool psa-check --all-namespaces --quiet
```

##### Testing revision-status Command (requires cluster)

```bash
# Check kube-apiserver static pod revisions
kubectl-dev_tool revision-status -n openshift-kube-apiserver
```

##### Testing download Command

```bash
# Download specific artifacts from a CI run (URL is a positional argument)
kubectl-dev_tool download "<prow-ci-url>" \
  --regex="clusteroperators" \
  --dir=/tmp/download-test
```

##### Testing certs Commands

```bash
# Inspect certs from a local must-gather
kubectl-dev_tool certs inspect-certs --local -f /path/to/must-gather/

# Inspect certs from a live cluster (requires cluster access)
kubectl-dev_tool certs inspect-certs --all-namespaces
```

### Adding a New Command

Step-by-step guide for adding a new CLI command:

#### 1. Create the Command Package

```bash
mkdir -p pkg/cmd/mycommand
```

#### 2. Implement the Command

Create `pkg/cmd/mycommand/mycommand.go`:

```go
package mycommand

import (
    "fmt"

    "github.com/spf13/cobra"
    "k8s.io/cli-runtime/pkg/genericclioptions"
)

type MyCommandOptions struct {
    // Add command-specific fields here
    myFlag string

    genericclioptions.IOStreams
}

func NewMyCommandOptions(streams genericclioptions.IOStreams) *MyCommandOptions {
    return &MyCommandOptions{
        IOStreams: streams,
    }
}

func NewCmdMyCommand(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
    o := NewMyCommandOptions(streams)

    cmd := &cobra.Command{
        Use:          "mycommand",
        Short:        "Description of what it does.",
        Example:      fmt.Sprintf("  %s mycommand --flag=value", parentName),
        SilenceUsage: true,
        RunE: func(c *cobra.Command, args []string) error {
            if err := o.Complete(c, args); err != nil {
                return err
            }
            if err := o.Validate(); err != nil {
                return err
            }
            return o.Run()
        },
    }

    cmd.Flags().StringVar(&o.myFlag, "flag", "", "Description of the flag")

    return cmd
}

func (o *MyCommandOptions) Complete(cmd *cobra.Command, args []string) error {
    // Resolve flags, prepare runtime state
    return nil
}

func (o *MyCommandOptions) Validate() error {
    // Validate flag combinations and required inputs
    return nil
}

func (o *MyCommandOptions) Run() error {
    // Execute command logic
    // Use o.Out for output, o.ErrOut for errors
    fmt.Fprintf(o.Out, "Result: %s\n", o.myFlag)
    return nil
}
```

#### 3. Register in main.go

Edit `cmd/kubectl-dev_tool/main.go`:

```go
import (
    // ... existing imports
    "github.com/openshift/cluster-debug-tools/pkg/cmd/mycommand"
)

// In NewCmdDevHelpers():
cmd.AddCommand(mycommand.NewCmdMyCommand("openshift-dev-helpers", streams))
```

#### 4. Add Tests

Create `pkg/cmd/mycommand/mycommand_test.go` with table-driven tests.

#### 5. Update Documentation

Update `README.md` to include the new command in the available commands list.

### Debugging

#### Build Issues

```bash
# Clean and rebuild
make clean
make build

# Check for formatting issues
make verify-gofmt

# Auto-fix formatting
make update-gofmt

# Check for go vet issues
make verify-govet
```

#### Dependency Issues

```bash
# Tidy go.mod
go mod tidy

# Re-vendor dependencies
go mod vendor

# Verify vendor consistency
go mod verify
```

#### Common Issues

**Issue**: Build fails with missing dependency
```bash
# Re-vendor all dependencies
go mod tidy
go mod vendor
```

**Issue**: `verify-gofmt` fails
```bash
# Auto-format all Go files
make update-gofmt
```

**Issue**: GCS download fails with "not gcs link"
```
# Verify your URL matches one of the supported patterns:
# - https://prow.ci.openshift.org/view/gs/<bucket>/<path>
# - https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/<bucket>/<path>
```

**Issue**: Commands requiring cluster access fail
```bash
# Verify cluster connectivity
oc whoami
oc get nodes

# Check kubeconfig
echo $KUBECONFIG
```

---

## Additional Resources

- [OpenShift CI Documentation](https://docs.ci.openshift.org/)
- [kubectl Plugin Documentation](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
- [Kubernetes Code Conventions](https://github.com/kubernetes/community/blob/main/contributors/guide/coding-conventions.md)
- [OpenShift build-machinery-go](https://github.com/openshift/build-machinery-go)

---

**Questions?**

- Open an issue on GitHub
- Review the OWNERS file for maintainer contacts
- Post in [#forum-ocp-apiserver](https://redhat.enterprise.slack.com/archives/CB48XQ4KZ) on Slack

**Thank you for contributing!**
