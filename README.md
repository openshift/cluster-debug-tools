cluster-debug-tools
===================

**WARNING**: The following tool is provided with no guarantees and might (and will) be changed at any time. Please do not rely on anything below in your scripts or automatization.

`openshift-dev-helpers` binary combines various tools useful for the OpenShift developers teams.

To list all events recorded during a test run and stored in `events.json` file, you can run this command:
```bash
./bin/openshift-dev-helpers events https://storage.googleapis.com/origin-ci-test/pr-logs/.../artifacts/e2e-aws/events.json --component=openshift-apiserver-operator
```

### Building

Place in GOPATH under `src/github.com/openshift/cluster-debug-tools`.

Build with:
```
$ make
```
