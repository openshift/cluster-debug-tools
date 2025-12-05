cluster-debug-tools
===================

> :warning:**WARNING**:warning: The following tool is provided with no guarantees and might (and will) be changed at any time. Please do not rely on anything below in your scripts or automatization.

`kubectl-dev_tools` binary combines various tools useful for the OpenShift developers teams. This binary is used as [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).

### Available commands

* `psa-check`       checks for pod security violations in a given namespace.
* `analyze-e2e`     inspects the artifacts gathered during e2e-aws run and analyze them.
* `audit`           inspects the audit logs captured during CI test run.
* `event`           inspects the event logs captured during CI test run.
* `event-filter`    filters the event logs captured during CI test run on a webpage (based on jqgrid).
* `inspect-certs`   inspects the certs, keys, and ca-bundles in a set of resources.
* `revision-status` counts failed installer pods and current revision of static pods.
* `download`        downloads artifacts from a Prow CI or GCS Link for a specified regex or all contents.

### Building and Installing

To make this plugin available in `oc` or `kubectl`, just run: `go get github.com/openshift/cluster-debug-tools/cmd/kubectl-dev_tool`

Alternatively, you can build and install it manually:

```go
$ make
$ cp kubectl-dev_tool ${HOME}/bin/
```

### Claude Agent Definition

This repository defines a Claude agent that can operate the kubectl-dev_tool CLI.
You can use natural language to ask the agent to analyze audit logs, inspect certificates, or run any other supported command.
The agent will translate your request into the correct CLI invocation and execute it.

#### Example Interactions with the Claude Agent

Below is an example demonstrating how Claude uses this agent in practice.

```
> (Me) Find all failed calls to the kube-system namespace in my audit logs.

⏺ (Claude) Sure — please provide the audit log path.

> (Me) /Users/me/workspace/audit-logs/kube-apiserver

⏺ (Claude) Running analysis...
Summary:
  Total failed calls: 281
  404: 229 events
  429: 45 events
  504: 3 events
  500: 2 events
  403: 2 events
```


### License

cluster-debug-tools is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).

