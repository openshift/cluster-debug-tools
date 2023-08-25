package locateinclustercerts

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/duration"
)

func toCertKeyPair(certificate *x509.Certificate) (*certgraphapi.CertKeyPair, error) {
	ret := &certgraphapi.CertKeyPair{
		Name: fmt.Sprintf("%v::%v", certificate.Subject.CommonName, certificate.SerialNumber),
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: nil,
			CertMetadata:    toCertKeyMetadata(certificate),
		},
	}

	details, err := toCertKeyPairDetails(certificate)
	ret.Spec.Details = details
	if err != nil {
		ret.Status.Errors = append(ret.Status.Errors, err.Error())
	}

	return ret, nil
}

func toCertKeyPairDetails(certificate *x509.Certificate) (certgraphapi.CertKeyPairDetails, error) {
	count := 0
	isClient := false
	isServing := false
	isSigner := (certificate.KeyUsage & x509.KeyUsageCertSign) != 0
	if isSigner {
		count++
	}
	for _, curr := range certificate.ExtKeyUsage {
		if curr == x509.ExtKeyUsageClientAuth {
			isClient = true
			count++
		}
		if curr == x509.ExtKeyUsageServerAuth {
			isServing = true
			count++
		}
	}
	var typeError error
	if count == 0 {
		typeError = fmt.Errorf("you have a cert for nothing?")
	}
	if count > 1 {
		typeError = fmt.Errorf("you have a cert for more than one?  We don't do that. :(")
	}

	ret := certgraphapi.CertKeyPairDetails{}
	if isClient {
		ret.CertType = "ClientCertDetails"
		ret.ClientCertDetails = toClientCertDetails(certificate)
	}
	if isServing {
		ret.CertType = "ServingCertDetails"
		ret.ServingCertDetails = toServingCertDetails(certificate)
	}
	if isSigner {
		ret.CertType = "SignerCertDetails"
		ret.SignerDetails = toSignerDetails(certificate)
	}

	if count > 1 {
		ret.CertType = "Multiple"
	}

	return ret, typeError
}

func toClientCertDetails(certificate *x509.Certificate) *certgraphapi.ClientCertDetails {
	return &certgraphapi.ClientCertDetails{
		Organizations: certificate.Subject.Organization,
	}
}

func toServingCertDetails(certificate *x509.Certificate) *certgraphapi.ServingCertDetails {
	ret := &certgraphapi.ServingCertDetails{
		DNSNames:    certificate.DNSNames,
		IPAddresses: nil,
	}

	for _, curr := range certificate.IPAddresses {
		ret.IPAddresses = append(ret.IPAddresses, curr.String())
	}

	return ret
}

func toSignerDetails(certificate *x509.Certificate) *certgraphapi.SignerCertDetails {
	return &certgraphapi.SignerCertDetails{}
}

func toCertKeyMetadata(certificate *x509.Certificate) certgraphapi.CertKeyMetadata {
	ret := certgraphapi.CertKeyMetadata{
		CertIdentifier: certgraphapi.CertIdentifier{
			CommonName:   certificate.Subject.CommonName,
			SerialNumber: certificate.SerialNumber.String(),
		},
		SignatureAlgorithm: certificate.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: certificate.PublicKeyAlgorithm.String(),
		ValidityDuration:   duration.HumanDuration(certificate.NotAfter.Sub(certificate.NotBefore)),
	}

	switch publicKey := certificate.PublicKey.(type) {
	case *ecdsa.PublicKey:
		ret.PublicKeyBitSize = fmt.Sprintf("%d bit, %v curve", publicKey.Params().BitSize, publicKey.Params().Name)
	case *rsa.PublicKey:
		ret.PublicKeyBitSize = fmt.Sprintf("%d bit", publicKey.Size()*8)
	default:
		fmt.Fprintf(os.Stderr, "%T\n", publicKey)
	}

	signerHumanName := certificate.Issuer.CommonName
	ret.CertIdentifier.Issuer = &certgraphapi.CertIdentifier{
		CommonName:   signerHumanName,
		SerialNumber: certificate.Issuer.SerialNumber,
	}

	humanUsages := []string{}
	if (certificate.KeyUsage & x509.KeyUsageDigitalSignature) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDigitalSignature")
	}
	if (certificate.KeyUsage & x509.KeyUsageContentCommitment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageContentCommitment")
	}
	if (certificate.KeyUsage & x509.KeyUsageKeyEncipherment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageKeyEncipherment")
	}
	if (certificate.KeyUsage & x509.KeyUsageDataEncipherment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDataEncipherment")
	}
	if (certificate.KeyUsage & x509.KeyUsageKeyAgreement) != 0 {
		humanUsages = append(humanUsages, "KeyUsageKeyAgreement")
	}
	if (certificate.KeyUsage & x509.KeyUsageCertSign) != 0 {
		humanUsages = append(humanUsages, "KeyUsageCertSign")
	}
	if (certificate.KeyUsage & x509.KeyUsageCRLSign) != 0 {
		humanUsages = append(humanUsages, "KeyUsageCRLSign")
	}
	if (certificate.KeyUsage & x509.KeyUsageEncipherOnly) != 0 {
		humanUsages = append(humanUsages, "KeyUsageEncipherOnly")
	}
	if (certificate.KeyUsage & x509.KeyUsageDecipherOnly) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDecipherOnly")
	}
	ret.Usages = humanUsages

	humanExtendedUsages := []string{}
	for _, curr := range certificate.ExtKeyUsage {
		switch curr {
		case x509.ExtKeyUsageAny:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageAny")
		case x509.ExtKeyUsageServerAuth:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageServerAuth")
		case x509.ExtKeyUsageClientAuth:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageClientAuth")
		case x509.ExtKeyUsageCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageCodeSigning")
		case x509.ExtKeyUsageEmailProtection:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageEmailProtection")
		case x509.ExtKeyUsageIPSECEndSystem:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECEndSystem")
		case x509.ExtKeyUsageIPSECTunnel:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECTunnel")
		case x509.ExtKeyUsageIPSECUser:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECUser")
		case x509.ExtKeyUsageTimeStamping:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageTimeStamping")
		case x509.ExtKeyUsageOCSPSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageOCSPSigning")
		case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftServerGatedCrypto")
		case x509.ExtKeyUsageNetscapeServerGatedCrypto:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageNetscapeServerGatedCrypto")
		case x509.ExtKeyUsageMicrosoftCommercialCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftCommercialCodeSigning")
		case x509.ExtKeyUsageMicrosoftKernelCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftKernelCodeSigning")
		default:
			panic(fmt.Sprintf("unrecognized %v", curr))
		}
	}
	ret.ExtendedUsages = humanExtendedUsages

	return ret
}

func addSecretLocation(in *certgraphapi.CertKeyPair, namespace, name string) *certgraphapi.CertKeyPair {
	secretLocation := certgraphapi.InClusterSecretLocation{
		Namespace: namespace,
		Name:      name,
	}
	out := in.DeepCopy()
	for _, curr := range in.Spec.SecretLocations {
		if curr == secretLocation {
			return out
		}
	}

	out.Spec.SecretLocations = append(out.Spec.SecretLocations, secretLocation)
	return out
}

func addCertFileLocation(in *certgraphapi.CertKeyPair, masterLocation, path string, info os.FileInfo) *certgraphapi.CertKeyPair {
	certListing, err := parseListing(path + ".listing.txt")
	if err != nil {
		panic(err)
	}
	keyListing, err := parseListing(path + ".keylisting.txt")
	if err != nil {
		panic(err)
	}
	location := certgraphapi.OnDiskCertKeyPairLocation{
		Cert: certgraphapi.OnDiskLocation{
			Path:           masterLocation,
			User:           certListing.user,
			Group:          certListing.group,
			Permissions:    certListing.permissions,
			SELinuxOptions: certListing.selinux,
		},
	}
	if keyListing != nil {
		location.Key = certgraphapi.OnDiskLocation{
			Path:           masterLocation[:len(masterLocation)-4] + ".key",
			User:           keyListing.user,
			Group:          keyListing.group,
			Permissions:    keyListing.permissions,
			SELinuxOptions: keyListing.selinux,
		}
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

func combineSecretLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.InClusterSecretLocation) *certgraphapi.CertKeyPair {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.SecretLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.SecretLocations = append(out.Spec.SecretLocations, curr)
		}
	}

	return out
}

func combineCertOnDiskLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.OnDiskCertKeyPairLocation) *certgraphapi.CertKeyPair {
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

func deduplicateCertKeyPairs(in []*certgraphapi.CertKeyPair) []*certgraphapi.CertKeyPair {
	ret := []*certgraphapi.CertKeyPair{}
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
				ret[j] = combineSecretLocations(ret[j], currIn.Spec.SecretLocations)
				ret[j] = combineCertOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
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

// addErrorMarkers mutates the input to attach error markers
func addErrorMarkers(in *certgraphapi.CertKeyPairList) {
	for i := range in.Items {
		issuerCommonName := in.Items[i].Spec.CertMetadata.CertIdentifier.Issuer.CommonName
		found := false
		for _, curr := range in.Items {
			if curr.Spec.CertMetadata.CertIdentifier.CommonName == issuerCommonName {
				found = true
				break
			}
		}
		if !found {
			in.Items[i].Status.Errors = append(
				in.Items[i].Status.Errors,
				fmt.Sprintf("issuer %q not present in the cluster", issuerCommonName),
			)
		}
	}
}

func guessLogicalNamesForCertKeyPairList(in *certgraphapi.CertKeyPairList) {
	for i := range in.Items {
		meaning := guessMeaningForCertKeyPair(in.Items[i])
		in.Items[i].LogicalName = meaning.name
		in.Items[i].Description = meaning.description
	}
}

func newSecretLocation(namespace, name string) certgraphapi.InClusterSecretLocation {
	return certgraphapi.InClusterSecretLocation{
		Namespace: namespace,
		Name:      name,
	}
}

var secretLocationToLogicalName = map[certgraphapi.InClusterSecretLocation]logicalMeaning{
	newSecretLocation("openshift-kube-apiserver", "aggregator-client"):                                newMeaning("aggregator-front-proxy-client", "Client certificate used by the kube-apiserver to communicate to aggregated apiservers."),
	newSecretLocation("openshift-kube-apiserver-operator", "aggregator-client-signer"):                newMeaning("aggregator-front-proxy-signer", "Signer for the kube-apiserver to create client certificates for aggregated apiservers to recognize as a front-proxy."),
	newSecretLocation("openshift-kube-apiserver-operator", "node-system-admin-client"):                newMeaning("per-master-debugging-client", "Client certificate (system:masters) placed on each master to allow communication to kube-apiserver for debugging."),
	newSecretLocation("openshift-kube-apiserver-operator", "node-system-admin-signer"):                newMeaning("per-master-debugging-signer", "Signer for the per-master-debugging-client."),
	newSecretLocation("openshift-kube-apiserver-operator", "kube-control-plane-signer"):               newMeaning("kube-control-plane-signer", "Signer for kube-controller-manager and kube-scheduler client certificates."),
	newSecretLocation("openshift-kube-controller-manager", "kube-controller-manager-client-cert-key"): newMeaning("kube-controller-manager-client", "Client certificate used by the kube-controller-manager to authenticate to the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver", "check-endpoints-client-cert-key"):                  newMeaning("kube-apiserver-check-endpoints", "Client certificate used by the network connectivity checker of the kube-apiserver."),
	newSecretLocation("openshift-kube-scheduler", "kube-scheduler-client-cert-key"):                   newMeaning("kube-scheduler-client", "Client certificate used by the kube-scheduler to authenticate to the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver-operator", "kube-apiserver-to-kubelet-signer"):        newMeaning("kube-apiserver-to-kubelet-signer", "Signer for the kube-apiserver-to-kubelet-client so kubelets can recognize the kube-apiserver."),
	newSecretLocation("openshift-kube-apiserver", "kubelet-client"):                                   newMeaning("kube-apiserver-to-kubelet-client", "Client certificate used by the kube-apiserver to authenticate to the kubelet for requests like exec and logs."),
	newSecretLocation("openshift-kube-controller-manager-operator", "csr-signer-signer"):              newMeaning("kube-controller-manager-csr-signer-signer", "Signer used by the kube-controller-manager-operator to sign signing certificates for the CSR API."),
	newSecretLocation("openshift-kube-controller-manager", "csr-signer"):                              newMeaning("kube-controller-manager-csr-signer", "Signer used by the kube-controller-manager to sign CSR API requests."),
	newSecretLocation("openshift-service-ca", "signing-key"):                                          newMeaning("service-serving-signer", "Signer used by service-ca to sign serving certificates for internal service DNS names."),
	newSecretLocation("openshift-kube-apiserver-operator", "loadbalancer-serving-signer"):             newMeaning("kube-apiserver-load-balancer-signer", "Signer used by the kube-apiserver operator to create serving certificates for the kube-apiserver via internal and external load balancers."),
	newSecretLocation("openshift-kube-apiserver", "internal-loadbalancer-serving-certkey"):            newMeaning("kube-apiserver-internal-load-balancer-serving", "Serving certificate used by the kube-apiserver to terminate requests via the internal load balancer."),
	newSecretLocation("openshift-kube-apiserver", "external-loadbalancer-serving-certkey"):            newMeaning("kube-apiserver-external-load-balancer-serving", "Serving certificate used by the kube-apiserver to terminate requests via the external load balancer."),
	newSecretLocation("openshift-kube-apiserver-operator", "localhost-recovery-serving-signer"):       newMeaning("kube-apiserver-recovery-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the localhost recovery SNI ServerName"),
	newSecretLocation("openshift-kube-apiserver", "localhost-recovery-serving-certkey"):               newMeaning("kube-apiserver-recovery-serving", "Serving certificate used by the kube-apiserver to terminate requests via the localhost recovery SNI ServerName."),
	newSecretLocation("openshift-kube-apiserver-operator", "service-network-serving-signer"):          newMeaning("kube-apiserver-service-network-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via the service network."),
	newSecretLocation("openshift-kube-apiserver", "service-network-serving-certkey"):                  newMeaning("kube-apiserver-service-network-serving", "Serving certificate used by the kube-apiserver to terminate requests via the service network."),
	newSecretLocation("openshift-kube-apiserver-operator", "localhost-serving-signer"):                newMeaning("kube-apiserver-localhost-signer", "Signer used by the kube-apiserver to create serving certificates for the kube-apiserver via localhost."),
	newSecretLocation("openshift-kube-apiserver", "localhost-serving-cert-certkey"):                   newMeaning("kube-apiserver-localhost-serving", "Serving certificate used by the kube-apiserver to terminate requests via localhost."),
	newSecretLocation("openshift-machine-config-operator", "machine-config-server-tls"):               newMeaning("mco-mystery-cert", "TODO: team needs to make description"),
	newSecretLocation("openshift-config", "etcd-signer"):                                              newMeaning("etcd-signer", "Signer for etcd to create client and serving certificates."),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
	newSecretLocation("", ""): newMeaning("", ""),
}

func guessMeaningForCertKeyPair(in certgraphapi.CertKeyPair) logicalMeaning {
	for _, loc := range in.Spec.SecretLocations {
		if meaning, ok := secretLocationToLogicalName[loc]; ok {
			return meaning
		}
	}

	for _, loc := range in.Spec.SecretLocations {
		if loc.Namespace != "openshift-etcd" {
			continue
		}
		if !strings.HasPrefix(loc.Name, "etcd-serving-metrics-") {
			continue
		}
		master := loc.Name[len("etcd-serving-metrics-"):]
		return newMeaning("etcd-metrics-for-master-"+master, "")
	}

	// service serving certs
	if in.Spec.CertMetadata.CertIdentifier.Issuer != nil &&
		strings.HasPrefix(in.Spec.CertMetadata.CertIdentifier.Issuer.CommonName, "openshift-service-serving-signer") {
		return newMeaning(in.Spec.CertMetadata.CertIdentifier.CommonName, "")
	}

	return newMeaning("", "")
}
