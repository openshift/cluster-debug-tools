module github.com/openshift/cluster-debug-tools

go 1.16

require (
	github.com/ghodss/yaml v1.0.0
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/openshift/api v0.0.0-20210423140644-156ca80f8d83
	github.com/openshift/build-machinery-go v0.0.0-20210209125900-0da259a2c359
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.21.0-rc.0
	k8s.io/apimachinery v0.21.0-rc.0
	k8s.io/apiserver v0.21.0-rc.0
	k8s.io/cli-runtime v0.21.0-rc.0
	k8s.io/client-go v0.21.0-rc.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0
)
