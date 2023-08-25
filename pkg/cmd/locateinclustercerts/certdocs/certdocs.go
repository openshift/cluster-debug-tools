package certdocs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gonum/graph"

	"k8s.io/apimachinery/pkg/util/errors"

	"github.com/gonum/graph/topo"
	"github.com/openshift/cluster-debug-tools/pkg/cmd/locateinclustercerts/certgraph"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type ClusterCerts struct {
	inClusterPKI certgraphapi.PKIList
	onDiskPKI    certgraphapi.PKIList
	combinedPKI  certgraphapi.PKIList

	// disjointPKIs is a list of non-intersecting PKILists
	disjointPKIs []certgraphapi.PKIList
}

func WriteDoc(pkiList *certgraphapi.PKIList, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	graphDOT, err := certgraph.DOTForPKIList(pkiList)
	if err != nil {
		return err
	}
	dotFile := filepath.Join(outputDir, "cert-flow.dot")
	if err := ioutil.WriteFile(dotFile, []byte(graphDOT), 0644); err != nil {
		return err
	}

	svg := exec.Command("dot", "-Kdot", "-T", "svg", "-o", filepath.Join(outputDir, "cert-flow.svg"), dotFile)
	//svg.Stderr = os.Stderr
	if err := svg.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}
	png := exec.Command("dot", "-Kdot", "-T", "png", "-o", filepath.Join(outputDir, "cert-flow.png"), dotFile)
	//png.Stderr = os.Stderr
	if err := png.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
	}

	jsonBytes, err := json.MarshalIndent(pkiList, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(outputDir, "cert-flow.json"), jsonBytes, 0644); err != nil {
		return err
	}

	graph, err := certgraph.GraphForPKIList(pkiList)
	if err != nil {
		return err
	}
	title := pkiList.LogicalName
	if len(title) == 0 {
		title = outputDir
	}
	markdown, err := GetMarkdownForPKILIst(title, pkiList.Description, outputDir, graph)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(outputDir, "README.md"), []byte(markdown), 0644); err != nil {
		return err
	}

	return nil
}

func WriteDocs(pkiList *certgraphapi.PKIList, outputDir string) error {
	disjointPKILists, err := SeparateDisjointPKI(pkiList)
	if err != nil {
		return err
	}

	errs := []error{}
	for i, currPKI := range disjointPKILists {
		filename := fmt.Sprintf("%d", i)
		if len(currPKI.LogicalName) > 0 {
			filename = currPKI.LogicalName
		}

		err := WriteDoc(currPKI, filepath.Join(outputDir, filename))
		if err != nil {
			errs = append(errs, err)
		}
	}

	readme := GetMarkdownForSummary(disjointPKILists)
	if err := ioutil.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}

	return errors.NewAggregate(errs)
}

func SeparateDisjointPKI(pkiList *certgraphapi.PKIList) ([]*certgraphapi.PKIList, error) {
	pkiGraph, err := certgraph.GraphForPKIList(pkiList)
	if err != nil {
		return nil, err
	}
	lists := []*certgraphapi.PKIList{}
	subgraphNodes := topo.ConnectedComponents(graph.Undirect{G: pkiGraph})

	for i := range subgraphNodes {
		curr := &certgraphapi.PKIList{}
		for j := range subgraphNodes[i] {
			currNode := subgraphNodes[i][j]
			if item := currNode.(graphNode).GetCABundle(); item != nil {
				curr.CertificateAuthorityBundles.Items = append(curr.CertificateAuthorityBundles.Items, *item)
			}
			if item := currNode.(graphNode).GetCertKeyPair(); item != nil {
				curr.CertKeyPairs.Items = append(curr.CertKeyPairs.Items, *item)
			}
		}
		curr.LogicalName = guessLogicalNamesForPKIList(*curr)
		curr.Description = guessLogicalDescriptionsForPKIList(*curr)
		lists = append(lists, curr)
	}

	return lists, nil
}

type graphNode interface {
	GetCABundle() *certgraphapi.CertificateAuthorityBundle
	GetCertKeyPair() *certgraphapi.CertKeyPair
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

var logicalComponentNamesToPKILogicalName = map[string]logicalMeaning{
	"aggregator-front-proxy-ca":       newMeaning("Aggregated API Server Certificates", "Used to secure connections between the kube-apiserver and aggregated API Servers."),
	"kube-apiserver-total-client-ca":  newMeaning("kube-apiserver Client Certificates", "Used by the kube-apiserver to recognize clients using mTLS."),
	"etcd-metrics-ca":                 newMeaning("etcd Metrics Certificates", "Used to access etcd metrics using mTLS."),
	"etcd-ca":                         newMeaning("etcd Certificates", "Used to secure etcd internal communication and by apiservers to access etcd."), // 4.8 version
	"etcd-signer":                     newMeaning("etcd Certificates", "Used to secure etcd internal communication and by apiservers to access etcd."), // 4.7 version
	"service-ca":                      newMeaning("Service Serving Certificates", "Used to secure inter-service communication on the local cluster."),
	"kube-apiserver-total-serving-ca": newMeaning("kube-apiserver Serving Certificates", "Used by kube-apiserver clients to recognize the kube-apiserver."),
	"mco-mystery-cert":                newMeaning("MachineConfig Operator Certificates", "TODO need to work out who and what."),
	"proxy-ca":                        newMeaning("Proxy Certificates", "Used by the OpenShift platform to recognize the proxy.  Other usages are side-effects which work by accident and not by principled design."),
}

func guessLogicalNamesForPKIList(in certgraphapi.PKIList) string {
	possibleLogicalNames := sets.NewString()
	for _, curr := range in.CertKeyPairs.Items {
		possibleLogicalNames.Insert(logicalComponentNamesToPKILogicalName[curr.LogicalName].name)
	}
	for _, curr := range in.CertificateAuthorityBundles.Items {
		possibleLogicalNames.Insert(logicalComponentNamesToPKILogicalName[curr.LogicalName].name)
	}
	possibleLogicalNames.Delete("")

	return strings.Join(possibleLogicalNames.List(), " | ")
}

func guessLogicalDescriptionsForPKIList(in certgraphapi.PKIList) string {
	possibleLogicalNames := sets.NewString()
	for _, curr := range in.CertKeyPairs.Items {
		possibleLogicalNames.Insert(logicalComponentNamesToPKILogicalName[curr.LogicalName].description)
	}
	for _, curr := range in.CertificateAuthorityBundles.Items {
		possibleLogicalNames.Insert(logicalComponentNamesToPKILogicalName[curr.LogicalName].description)
	}
	possibleLogicalNames.Delete("")

	return strings.Join(possibleLogicalNames.List(), " | ")
}
