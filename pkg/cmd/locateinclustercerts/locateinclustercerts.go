package locateinclustercerts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/cluster-debug-tools/pkg/cmd/locateinclustercerts/certdocs"

	"github.com/openshift/cluster-debug-tools/pkg/cmd/locateinclustercerts/certgraph"
	"github.com/openshift/cluster-debug-tools/pkg/cmd/locateinclustercerts/certgraphapi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/util/cert"
)

var (
	example = `
    1. Build an image to collect the certs, keys, and ca bundles from the host.
       1. Something like -- docker build pkg/cmd/locateinclustercerts/ -t docker.io/deads2k/cert-collection:latest -f Dockerfile
       2. Push to dockerhub
	2. Gather data.
       1. oc adm inspect clusteroperators -- this will gather all the in-cluster certificates and ca bundles
       2. run pods on the masters.  Something like oc debug --image=docker.io/deads2k/cert-collection:08 node/ci-ln-z2l4snt-f76d1-prqp5-master-2
       3. in those pods, run master-cert-collection.sh to collect the data from the host
       4. pull the on-disk data locally. Something like oc rsync ci-ln-z2l4snt-f76d1-prqp5-master-2-debug:/must-gather ../sample-data/master-2/
    3. Be sure dot is installed locally
    4. Run code like kubectl-dev_tool certs locate-incluster-certs --local -f ../sample-data/ --additional-input-dir ../sample-data/ -odoc to produce the doc.

	# look at certs on the cluster in the "openshift-kube-apiserver" namespace
	openshift-dev-helpers inspect-certs -n openshift-kube-apiserver secrets,configmaps

	# look at certs from CSRs
	openshift-dev-helpers inspect-certs csr

	# create a fake secret from a file to inspect its content
	oc create secret generic --dry-run -oyaml kubelet --from-file=tls.crt=/home/deads/Downloads/kubelet-client-current.pem | openshift-dev-helpers inspect-certs --local -f -

	# look at a dumped file of resources for inspection
	openshift-dev-helpers inspect-certs --local -f 'path/to/core/configmaps.yaml'
`
)

type LocateInClusterCertsOptions struct {
	builderFlags       *genericclioptions.ResourceBuilderFlags
	configFlags        *genericclioptions.ConfigFlags
	additionalInputDir string
	outputMode         string

	resourceFinder genericclioptions.ResourceFinder

	outputDir string

	genericclioptions.IOStreams
}

func NewLocateInClusterCertsOptions(streams genericclioptions.IOStreams) *LocateInClusterCertsOptions {
	return &LocateInClusterCertsOptions{
		builderFlags: genericclioptions.NewResourceBuilderFlags().
			WithAll(true).
			WithAllNamespaces(false).
			WithFieldSelector("").
			WithLabelSelector("").
			WithLocal(false),
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}
}

func NewCmdLocateInClusterCerts(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewLocateInClusterCertsOptions(streams)

	cmd := &cobra.Command{
		Use:          "locate-incluster-certs <resource>",
		Short:        "Inspects the certs, keys, and ca-bundles in a set of resources.",
		Example:      example,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	o.builderFlags.AddFlags(cmd.Flags())
	o.configFlags.AddFlags(cmd.Flags())

	cmd.Flags().StringVarP(&o.outputMode, "output", "o", o.outputMode, "Choose your output format")
	cmd.Flags().StringVar(&o.additionalInputDir, "additional-input-dir", o.additionalInputDir, "Additional directory of certs.")

	return cmd
}

func (o *LocateInClusterCertsOptions) Complete(command *cobra.Command, args []string) error {
	o.resourceFinder = o.builderFlags.ToBuilder(o.configFlags, args)

	if len(o.outputDir) == 0 {
		var err error
		o.outputDir, err = os.Getwd()
		if err != nil {
			return err
		}
		o.outputDir = filepath.Join(o.outputDir, "docs")
	}

	return nil
}

func (o *LocateInClusterCertsOptions) Validate() error {
	switch o.outputMode {
	case "", "json":
	case "dot":
	case "doc":
	default:
		return fmt.Errorf("unknown output mode")
	}

	return nil
}

func (o *LocateInClusterCertsOptions) Run() error {
	pkiList, err := o.GatherCerts()
	if err != nil {
		return err
	}

	switch o.outputMode {
	case "", "json":
		jsonBytes, err := json.MarshalIndent(pkiList, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%v\n\n", string(jsonBytes))
	case "dot":
		graphDOT, err := certgraph.DOTForPKIList(pkiList)
		if err != nil {
			return err
		}
		fmt.Println(graphDOT)
	case "doc":
		if err := certdocs.WriteDocs(pkiList, o.outputDir); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown output mode")
	}
	return err
}

func (o *LocateInClusterCertsOptions) GatherCerts() (*certgraphapi.PKIList, error) {
	visitor := o.resourceFinder.Do()

	certs := []*certgraphapi.CertKeyPair{}
	caBundles := []*certgraphapi.CertificateAuthorityBundle{}
	errs := []error{}
	err := visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			if strings.Contains(err.Error(), "is registered for version") {
				return nil
			}
			return err
		}

		var convertedObj runtime.Object
		switch castObj := info.Object.(type) {
		case *unstructured.Unstructured:
			switch castObj.GetObjectKind().GroupVersionKind() {
			case schema.GroupVersionKind{Version: "v1", Kind: "Secret"}:
				obj := &corev1.Secret{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(castObj.UnstructuredContent(), obj)
				if err != nil {
					return err
				}
				convertedObj = obj
			case schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}:
				obj := &corev1.ConfigMap{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(castObj.UnstructuredContent(), obj)
				if err != nil {
					return err
				}
				convertedObj = obj
			default:
				// do nothing
			}

		case *unstructured.UnstructuredList:
			return fmt.Errorf("unexpected %T", castObj)
		default:
			return nil
		}

		switch castObj := convertedObj.(type) {
		case *corev1.ConfigMap:
			details, err := inspectConfigMap(castObj)
			if details != nil {
				caBundles = append(caBundles, details)
			}
			if err != nil {
				errs = append(errs, err)
			}
		case *corev1.Secret:
			details, err := inspectSecret(castObj)
			if details != nil {
				certs = append(certs, details)
			}
			if err != nil {
				errs = append(errs, err)
			}
		default:
			klog.V(2).Infof("ignoring: %T", castObj)
			return nil
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	filesystemCerts, err := o.GatherCertsFromFilesystem()
	if err != nil {
		return nil, err
	}
	for i := range filesystemCerts.CertKeyPairs.Items {
		certs = append(certs, &filesystemCerts.CertKeyPairs.Items[i])
	}
	for i := range filesystemCerts.CertificateAuthorityBundles.Items {
		caBundles = append(caBundles, &filesystemCerts.CertificateAuthorityBundles.Items[i])
	}

	certs = deduplicateCertKeyPairs(certs)
	certList := &certgraphapi.CertKeyPairList{}
	for i := range certs {
		certList.Items = append(certList.Items, *certs[i])
	}
	addErrorMarkers(certList)
	guessLogicalNamesForCertKeyPairList(certList)

	caBundles = deduplicateCABundles(caBundles)
	caBundleList := &certgraphapi.CertificateAuthorityBundleList{}
	for i := range caBundles {
		caBundleList.Items = append(caBundleList.Items, *caBundles[i])
	}
	guessLogicalNamesForCABundleList(caBundleList)

	return &certgraphapi.PKIList{
		CertificateAuthorityBundles: *caBundleList,
		CertKeyPairs:                *certList,
	}, errors.NewAggregate(errs)
}

func inspectConfigMap(obj *corev1.ConfigMap) (*certgraphapi.CertificateAuthorityBundle, error) {
	caBundle, ok := obj.Data["ca-bundle.crt"]
	if !ok {
		return nil, nil
	}
	if len(caBundle) == 0 {
		return nil, nil
	}

	certificates, err := cert.ParseCertsPEM([]byte(caBundle))
	if err != nil {
		return nil, err
	}
	caBundleDetail, err := toCABundle(certificates)
	if err != nil {
		return nil, err
	}
	caBundleDetail = addConfigMapLocation(caBundleDetail, obj.Namespace, obj.Name)

	return caBundleDetail, nil
}

func inspectSecret(obj *corev1.Secret) (*certgraphapi.CertKeyPair, error) {
	resourceString := fmt.Sprintf("secrets/%s[%s]", obj.Name, obj.Namespace)
	tlsCrt, isTLS := obj.Data["tls.crt"]
	if !isTLS {
		return nil, nil
	}
	//fmt.Printf("%s - tls (%v)\n", resourceString, obj.CreationTimestamp.UTC())
	if len(tlsCrt) == 0 {
		return nil, fmt.Errorf("%s MISSING tls.crt content\n", resourceString)
	}

	certificates, err := cert.ParseCertsPEM([]byte(tlsCrt))
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		detail = addSecretLocation(detail, obj.Namespace, obj.Name)
		return detail, nil
	}
	return nil, fmt.Errorf("didn't see that coming")
}
