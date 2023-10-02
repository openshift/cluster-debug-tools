package certinspection

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/cert"
)

var (
	example = `
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

type CertInspectionOptions struct {
	builderFlags *genericclioptions.ResourceBuilderFlags
	configFlags  *genericclioptions.ConfigFlags

	resourceFinder genericclioptions.ResourceFinder

	genericclioptions.IOStreams
}

func NewCertInspectionOptions(streams genericclioptions.IOStreams) *CertInspectionOptions {
	return &CertInspectionOptions{
		builderFlags: genericclioptions.NewResourceBuilderFlags().
			WithAll(true).
			WithAllNamespaces(false).
			WithFieldSelector("").
			WithLabelSelector("").
			WithLocal(false).
			WithScheme(scheme.Scheme),
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}
}

func NewCmdCertInspection(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCertInspectionOptions(streams)

	cmd := &cobra.Command{
		Use:          "inspect-certs <resource>",
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

	return cmd
}

func (o *CertInspectionOptions) Complete(command *cobra.Command, args []string) error {
	o.resourceFinder = o.builderFlags.ToBuilder(o.configFlags, args)

	return nil
}

func (o *CertInspectionOptions) Validate() error {
	return nil
}

func (o *CertInspectionOptions) Run() error {
	visitor := o.resourceFinder.Do()

	err := visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		switch castObj := info.Object.(type) {
		case *corev1.ConfigMap:
			inspectConfigMap(castObj)
		case *corev1.Secret:
			inspectSecret(castObj)
		// case *certificatesv1beta1.CertificateSigningRequest:
		// 	inspectCSR(castObj)
		default:
			return fmt.Errorf("unhandled resource: %T", castObj)
		}

		fmt.Println()
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func inspectConfigMap(obj *corev1.ConfigMap) {
	resourceString := fmt.Sprintf("configmaps/%s[%s]", obj.Name, obj.Namespace)
	caBundle, err := certgraphanalysis.InspectConfigMap(obj)
	if err != nil {
		fmt.Printf("%s ERROR - %v\n", resourceString, err)
		return
	}
	if caBundle == nil {
		fmt.Printf("%s - not a caBundle\n", resourceString)
		return
	}
	fmt.Printf("%s - ca bundle (%v)\n", resourceString, obj.CreationTimestamp.UTC())
	for _, curr := range caBundle.Spec.CertificateMetadata {
		fmt.Printf("    %s\n", certMetadataDetail(curr))
	}
}

func inspectSecret(obj *corev1.Secret) {
	resourceString := fmt.Sprintf("secrets/%s[%s]", obj.Name, obj.Namespace)
	secret, err := certgraphanalysis.InspectSecret(obj)
	if err != nil {
		fmt.Printf("%s ERROR - %v\n", resourceString, err)
		return
	}
	if secret == nil {
		fmt.Printf("%s - not a secret\n", resourceString)
		return
	}
	fmt.Printf("%s - tls (%v)\n", resourceString, obj.CreationTimestamp.UTC())
	fmt.Printf("    %s\n", certMetadataDetail(secret.Spec.CertMetadata))
}

func inspectCSR(obj *certificatesv1beta1.CertificateSigningRequest) {
	resourceString := fmt.Sprintf("csr/%s", obj.Name)
	if len(obj.Status.Certificate) == 0 {
		fmt.Printf("%s NOT SIGNED\n", resourceString)
		return
	}

	fmt.Printf("%s - (%v)\n", resourceString, obj.CreationTimestamp.UTC())
	certificates, err := cert.ParseCertsPEM([]byte(obj.Status.Certificate))
	if err != nil {
		fmt.Printf("    ERROR - %v\n", err)
		return
	}
	for _, curr := range certificates {
		fmt.Printf("    %s\n", certDetail(curr))
	}
}

func certDetail(certificate *x509.Certificate) string {
	humanName := certificate.Subject.CommonName
	signerHumanName := certificate.Issuer.CommonName
	if certificate.Subject.CommonName == certificate.Issuer.CommonName {
		signerHumanName = "<self>"
	}

	usages := []string{}
	for _, curr := range certificate.ExtKeyUsage {
		if curr == x509.ExtKeyUsageClientAuth {
			usages = append(usages, "client")
			continue
		}
		if curr == x509.ExtKeyUsageServerAuth {
			usages = append(usages, "serving")
			continue
		}

		usages = append(usages, fmt.Sprintf("%d", curr))
	}

	validServingNames := []string{}
	for _, ip := range certificate.IPAddresses {
		validServingNames = append(validServingNames, ip.String())
	}
	for _, dnsName := range certificate.DNSNames {
		validServingNames = append(validServingNames, dnsName)
	}
	servingString := ""
	if len(validServingNames) > 0 {
		servingString = fmt.Sprintf(" validServingFor=[%s]", strings.Join(validServingNames, ","))
	}

	groupString := ""
	if len(certificate.Subject.Organization) > 0 {
		groupString = fmt.Sprintf(" groups=[%s]", strings.Join(certificate.Subject.Organization, ","))
	}

	return fmt.Sprintf("%q [%s]%s%s issuer=%q (%v to %v)", humanName, strings.Join(usages, ","), groupString, servingString, signerHumanName, certificate.NotBefore.UTC(), certificate.NotAfter.UTC())
}

func certMetadataDetail(certKeyMetadata certgraphapi.CertKeyMetadata) string {
	issuer := ""
	if certKeyMetadata.CertIdentifier.Issuer != nil {
		issuer = certKeyMetadata.CertIdentifier.Issuer.CommonName
	}
	if issuer == certKeyMetadata.CertIdentifier.CommonName {
		issuer = "<self>"
	}
	return fmt.Sprintf(
		"%q [%s] issuer=%q (%v)",
		certKeyMetadata.CertIdentifier.CommonName,
		strings.Join(certKeyMetadata.Usages, ","),
		issuer,
		certKeyMetadata.ValidityDuration,
	)
}
