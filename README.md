cluster-debug-tools
===================

> :warning:**WARNING**:warning: The following tool is provided with no guarantees and might (and will) be changed at any time. Please do not rely on anything below in your scripts or automatization.

`kubectl-dev_tools` binary combines various tools useful for the OpenShift developers teams. This binary is used as [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).

### Available commands

* `analyze-e2e`     inspects the artifacts gathered during e2e-aws run and analyze them.
* `audit`           inspects the audit logs captured during CI test run.
* `event`           inspects the event logs captured during CI test run.
* `event-filter`    filters the event logs captured during CI test run on a webpage (based on jqgrid).
* `inspect-certs`   inspects the certs, keys, and ca-bundles in a set of resources.
* `revision-status` counts failed installer pods and current revision of static pods.

### Building and Installing

To make this plugin available in `oc` or `kubectl`, just run: `go get github.com/openshift/cluster-debug-tools/cmd/kubectl-dev_tool`

Alternatively, you can build and install it manually:

```go
$ make
$ cp kubectl-dev_tool ${HOME}/bin/
```

### License

cluster-debug-tools is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).

