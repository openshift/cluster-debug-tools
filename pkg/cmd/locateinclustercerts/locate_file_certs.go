package locateinclustercerts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/client-go/util/cert"
)

func (o *LocateInClusterCertsOptions) GatherCertsFromFilesystem() (*certgraphapi.PKIList, error) {
	ret := &certgraphapi.PKIList{}
	err := filepath.Walk(o.additionalInputDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".crt") {
				return nil
			}
			if info.Size() == 0 {
				return nil
			}
			if strings.HasSuffix(path, "/ca-bundle.crt") {
				data, err := inspectCAFile(path, info)
				if err != nil {
					return err
				}
				ret.CertificateAuthorityBundles.Items = append(ret.CertificateAuthorityBundles.Items, *data)
				return nil
			}
			data, err := inspectCertFile(path, info)
			if err != nil {
				return err
			}
			ret.CertKeyPairs.Items = append(ret.CertKeyPairs.Items, *data)
			return nil
		})
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func inspectCertFile(path string, info os.FileInfo) (*certgraphapi.CertKeyPair, error) {
	masterLocationIndex := strings.Index(path, "/host") + len("/host")
	masterLocation := filepath.Join(path[masterLocationIndex:], info.Name())

	certContent, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	certificates, err := cert.ParseCertsPEM([]byte(certContent))
	if err != nil {
		return nil, err
	}
	for _, certificate := range certificates {
		detail, err := toCertKeyPair(certificate)
		if err != nil {
			return nil, err
		}
		detail = addCertFileLocation(detail, masterLocation, path, info)
		return detail, nil
	}
	return nil, fmt.Errorf("didn't see that coming")
}

func inspectCAFile(path string, info os.FileInfo) (*certgraphapi.CertificateAuthorityBundle, error) {
	masterLocationIndex := strings.Index(path, "/host") + len("/host")
	masterLocation := filepath.Join(path[masterLocationIndex:], info.Name())

	caBundle, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	certificates, err := cert.ParseCertsPEM(caBundle)
	if err != nil {
		return nil, err
	}
	caBundleDetail, err := toCABundle(certificates)
	if err != nil {
		return nil, err
	}
	caBundleDetail = addCABundleFileLocation(caBundleDetail, masterLocation, path, info)

	return caBundleDetail, nil
}

type fileListing struct {
	permissions string
	user        string
	group       string
	selinux     string
}

func parseListing(listingFile string) (*fileListing, error) {
	listingBytes, err := ioutil.ReadFile(listingFile)
	if err != nil {
		return nil, err
	}
	if len(listingBytes) == 0 {
		return nil, nil
	}

	tokens := strings.Split(string(listingBytes), " ")
	return &fileListing{
		permissions: tokens[0],
		user:        tokens[2],
		group:       tokens[3],
		selinux:     tokens[4],
	}, nil
}
