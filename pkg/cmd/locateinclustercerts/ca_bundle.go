package locateinclustercerts

import (
	"crypto/x509"
	"os"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func toCABundle(certificates []*x509.Certificate) (*certgraphapi.CertificateAuthorityBundle, error) {
	ret := &certgraphapi.CertificateAuthorityBundle{
		Spec: certgraphapi.CertificateAuthorityBundleSpec{},
	}

	certNames := []string{}
	for _, cert := range certificates {
		metadata := toCertKeyMetadata(cert)
		ret.Spec.CertificateMetadata = append(ret.Spec.CertificateMetadata, metadata)
		certNames = append(certNames, metadata.CertIdentifier.CommonName)
	}
	ret.Name = strings.Join(certNames, "|")

	return ret, nil
}

func addConfigMapLocation(in *certgraphapi.CertificateAuthorityBundle, namespace, name string) *certgraphapi.CertificateAuthorityBundle {
	secretLocation := certgraphapi.InClusterConfigMapLocation{
		Namespace: namespace,
		Name:      name,
	}
	out := in.DeepCopy()
	for _, curr := range in.Spec.ConfigMapLocations {
		if curr == secretLocation {
			return out
		}
	}

	out.Spec.ConfigMapLocations = append(out.Spec.ConfigMapLocations, secretLocation)
	return out
}

func addCABundleFileLocation(in *certgraphapi.CertificateAuthorityBundle, masterLocation, path string, info os.FileInfo) *certgraphapi.CertificateAuthorityBundle {
	caBundleListing, err := parseListing(path + ".listing.txt")
	if err != nil {
		panic(err)
	}
	location := certgraphapi.OnDiskLocation{
		Path:           masterLocation,
		User:           caBundleListing.user,
		Group:          caBundleListing.group,
		Permissions:    caBundleListing.permissions,
		SELinuxOptions: caBundleListing.selinux,
	}
	out := in.DeepCopy()
	for _, curr := range in.Spec.OnDiskLocations {
		if curr == location {
			return out
		}
	}

	out.Spec.OnDiskLocations = append(out.Spec.OnDiskLocations, location)
	return out
}

func combineConfigMapLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.InClusterConfigMapLocation) *certgraphapi.CertificateAuthorityBundle {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.ConfigMapLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.ConfigMapLocations = append(out.Spec.ConfigMapLocations, curr)
		}
	}

	return out
}

func combineCABundleOnDiskLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.OnDiskLocation) *certgraphapi.CertificateAuthorityBundle {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.OnDiskLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.OnDiskLocations = append(out.Spec.OnDiskLocations, curr)
		}
	}

	return out
}

func deduplicateCABundles(in []*certgraphapi.CertificateAuthorityBundle) []*certgraphapi.CertificateAuthorityBundle {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	for _, currIn := range in {
		found := false
		for j, currOut := range ret {
			if currIn == nil {
				panic("one")
			}
			if currOut == nil {
				panic("two")
			}
			if currOut.Name == currIn.Name {
				ret[j] = combineConfigMapLocations(ret[j], currIn.Spec.ConfigMapLocations)
				ret[j] = combineCABundleOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
				found = true
				break
			}
		}

		if !found {
			ret = append(ret, currIn.DeepCopy())
		}
	}

	return ret
}

func guessLogicalNamesForCABundleList(in *certgraphapi.CertificateAuthorityBundleList) {
	for i := range in.Items {
		meaning := guessMeaningForCABundle(in.Items[i])
		in.Items[i].LogicalName = meaning.name
		in.Items[i].Description = meaning.description
	}
}

func newConfigMapLocation(namespace, name string) certgraphapi.InClusterConfigMapLocation {
	return certgraphapi.InClusterConfigMapLocation{
		Namespace: namespace,
		Name:      name,
	}
}

type logicalMeaning struct {
	name        string
	description string
}

func newMeaning(name, description string) logicalMeaning {
	return logicalMeaning{
		name:        name,
		description: description,
	}
}

var configmapLocationToLogicalName = map[certgraphapi.InClusterConfigMapLocation]logicalMeaning{
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-aggregator-client-ca"):          newMeaning("aggregator-front-proxy-ca", "CA for aggregated apiservers to recognize kube-apiserver as front-proxy."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "node-system-admin-ca"):                newMeaning("kube-apiserver-per-master-debugging-client-ca", "CA for kube-apiserver to recognize local system:masters rendered to each master."),
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-client-ca"):                     newMeaning("kube-apiserver-total-client-ca", "CA for kube-apiserver to recognize all known certificate based clients."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "kube-control-plane-signer-ca"):        newMeaning("kube-apiserver-kcm-and-ks-client-ca", "CA for kube-apiserver to recognize the kube-controller-manager and kube-scheduler client certificates."),
	newConfigMapLocation("openshift-config", "initial-kube-apiserver-server-ca"):                     newMeaning("kube-apiserver-from-installer-client-ca", "CA for the kube-apiserver to recognize clients created by the installer."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "kube-apiserver-to-kubelet-client-ca"): newMeaning("kubelet-to-recognize-kube-apiserver-client-ca", "CA for the kubelet to recognize the kube-apiserver client certificate."),
	newConfigMapLocation("openshift-kube-controller-manager-operator", "csr-controller-signer-ca"):   newMeaning("kube-controller-manager-csr-signer-signer-ca", "CA to recognize the kube-controller-manager's signer for signing new CSR signing certificates."),
	newConfigMapLocation("openshift-config-managed", "csr-controller-ca"):                            newMeaning("kube-controller-manager-csr-ca", "CA to recognize the CSRs (both serving and client) signed by the kube-controller-manager."),
	newConfigMapLocation("openshift-config", "etcd-ca-bundle"):                                       newMeaning("etcd-ca", "CA for recognizing etcd serving, peer, and client certificates."),
	newConfigMapLocation("openshift-config-managed", "service-ca"):                                   newMeaning("service-ca", "CA for recognizing serving certificates for services that were signed by our service-ca controller."),
	newConfigMapLocation("openshift-kube-controller-manager", "serviceaccount-ca"):                   newMeaning("service-account-token-ca.crt", "CA for recognizing kube-apiserver.  This is injected into each service account token secret at ca.crt."),
	newConfigMapLocation("openshift-config-managed", "default-ingress-cert"):                         newMeaning("router-wildcard-serving-ca", "REVIEW: CA for recognizing the default router wildcard serving certificate."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "localhost-recovery-serving-ca"):       newMeaning("kube-apiserver-recovery-serving-ca", "CA for recognizing the kube-apiserver when connecting via the localhost recovery SNI ServerName."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "service-network-serving-ca"):          newMeaning("kube-apiserver-service-network-serving-ca", "CA for recognizing the kube-apiserver when connecting via the service network (kuberentes.default.svc)."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "localhost-serving-ca"):                newMeaning("kube-apiserver-localhost-serving-ca", "CA for recognizing the kube-apiserver when connecting via localhost."),
	newConfigMapLocation("openshift-kube-apiserver-operator", "loadbalancer-serving-ca"):             newMeaning("kube-apiserver-load-balancer-serving-ca", "CA for recognizing the kube-apiserver when connecting via the internal or external load balancers."),
	newConfigMapLocation("openshift-config-managed", "kube-apiserver-server-ca"):                     newMeaning("kube-apiserver-total-serving-ca", "CA for recognizing the kube-apiserver when connecting via any means."),
	newConfigMapLocation("openshift-config", "admin-kubeconfig-client-ca"):                           newMeaning("kube-apiserver-admin-kubeconfig-client-ca", "CA for kube-apiserver to recognize the system:master created by the installer."),
	newConfigMapLocation("openshift-etcd", "etcd-metrics-proxy-client-ca"):                           newMeaning("etcd-metrics-ca", "CA used to recognize etcd metrics serving and client certificates."), // 4.8 version
	newConfigMapLocation("openshift-config", "etcd-metric-serving-ca"):                               newMeaning("etcd-metrics-ca", "CA used to recognize etcd metrics serving and client certificates."), // 4.7 version
	newConfigMapLocation("openshift-config-managed", "trusted-ca-bundle"):                            newMeaning("proxy-ca", "CA used to recognize proxy servers.  By default this will contain standard root CAs on the cluster-network-operator pod."),
	newConfigMapLocation("", ""): newMeaning("", ""),
}

func guessMeaningForCABundle(in certgraphapi.CertificateAuthorityBundle) logicalMeaning {
	for _, loc := range in.Spec.ConfigMapLocations {
		if meaning, ok := configmapLocationToLogicalName[loc]; ok {
			return meaning
		}
	}
	return logicalMeaning{}
}
