package requestcounts

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/ghodss/yaml"
	apiv1 "github.com/openshift/api/apiserver/v1"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type APIRequestCountOptions struct {
	filename       string
	outputFilename string

	by string

	genericclioptions.IOStreams
}

func NewAPIRequestCountOptions(streams genericclioptions.IOStreams) *APIRequestCountOptions {
	return &APIRequestCountOptions{
		outputFilename: "requests-by-user.html",
		IOStreams:      streams,
	}
}

func NewCmdAPIRequestCount(parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAPIRequestCountOptions(streams)

	cmd := &cobra.Command{
		Use:          "request-count -f path",
		Short:        "todo",
		Example:      "todo",
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

	// TODO make this operate aginst an apiserver too.
	cmd.Flags().StringVarP(&o.filename, "filename", "f", o.filename, "directory containing apirequestcounts")
	cmd.Flags().StringVar(&o.by, "by", o.by, "todo")

	return cmd
}

func (o *APIRequestCountOptions) Complete(command *cobra.Command, args []string) error {
	return nil
}

func (o *APIRequestCountOptions) Validate() error {
	return nil
}

type resourceUsage struct {
	resource string
	count    int64
}

func (o *APIRequestCountOptions) Run() error {
	// TODO: read flags
	// TODO: prepare filters

	// copy/paste the actual request count structs I think
	userToResourceToCount := map[string]map[string]int64{}
	allResources := sets.String{}
	fmt.Println(walkData(o.filename, func(counter *apiv1.APIRequestCount) error {
		resource := counter.Name
		allResources.Insert(resource)
		resourceRequests := showAllRequestHistory(counter.Status)
		currResourceUserCounts := pivotUserToResource(resource, resourceRequests)

		for user, count := range currResourceUserCounts {
			if _, ok := userToResourceToCount[user]; !ok {
				userToResourceToCount[user] = map[string]int64{}
			}
			userToResourceToCount[user][resource] = userToResourceToCount[user][resource] + count.count
		}
		return nil
	}))

	orderedUsers, orderedResources, orderedData := toData(userToResourceToCount, allResources)
	jsReplacer := dataToJavaScriptReplacer(orderedUsers, orderedResources, orderedData)
	byUserHTMLPage := jsReplacer.Replace(byUserHTML)

	err := ioutil.WriteFile(o.outputFilename, []byte(byUserHTMLPage), 0644)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %q\n", o.outputFilename)

	// TODO: serialize to a file
	//fmt.Println(fmt.Sprintf("%v", ret))
	//serializeToAFile(ret, "/Users/lszaszki/go/src/github.com/p0lyn0mial/processor/ret.json")
	return nil
}

type userUsage struct {
	userKey   string
	userCount int64
}

func dataToJavaScriptReplacer(orderedUsers, orderedResources []string, orderedData [][]string) *strings.Replacer {
	quotedUsers := []string{}
	for _, user := range orderedUsers {
		quotedUsers = append(quotedUsers, fmt.Sprintf("%q", user))
	}
	orderedUserString := "[" + strings.Join(quotedUsers, ", ") + "]"

	quotedResources := []string{}
	for _, user := range orderedResources {
		quotedResources = append(quotedResources, fmt.Sprintf("%q", user))
	}
	orderedResourceString := "[" + strings.Join(quotedResources, ", ") + "]"

	orderDataStrings := []string{}
	for i, rowData := range orderedData {
		if i == 0 {
			orderDataStrings = append(orderDataStrings, orderedResourceString)
			continue
		}
		orderDataStrings = append(orderDataStrings, fmt.Sprintf("[%v], // %v", strings.Join(rowData, ", "), orderedUsers[i-1]))
	}
	orderedDataString := strings.Join(orderDataStrings, "\n\t\t")

	return strings.NewReplacer(
		"USERS_GO_HERE", orderedUserString,
		"RESOURCES_GO_HERE", orderedResourceString,
		"DATA_GOES_HERE", orderedDataString,
	)
}

// returns the ordered users, ordered categories, ordered data rows
func toData(userToResourceToCount map[string]map[string]int64, allResources sets.String) ([]string, []string, [][]string) {
	userUsages := []userUsage{}
	for user, resourceCount := range userToResourceToCount {
		userUsage := userUsage{userKey: user}
		for _, count := range resourceCount {
			userUsage.userCount += count
		}
		userUsages = append(userUsages, userUsage)
	}
	sort.Sort(sort.Reverse(userByMost(userUsages)))

	orderedUsers := []string{}
	orderedResources := allResources.List()
	data := [][]string{}
	data = append(data, orderedResources)
	for _, user := range orderedUsers {
		row := []string{}
		for _, resource := range allResources.List() {
			fmt.Printf("#### %q %v %d\n", user, resource, userToResourceToCount[user][resource])
			row = append(row, fmt.Sprintf("%d", userToResourceToCount[user][resource]))
		}
		data = append(data, row)
	}

	return orderedUsers, orderedResources, data
}

type userByMost []userUsage

func (a userByMost) Len() int           { return len(a) }
func (a userByMost) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a userByMost) Less(i, j int) bool { return a[i].userCount < a[j].userCount }

func serializeToAFile(data map[string]map[string]int64, filePath string) error {
	jsonFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	preamble := []byte("export const ResourcesData = [")
	_, err = jsonFile.Write(preamble)
	if err != nil {
		return err
	}

	for key, nestedMap := range data {
		item := map[string]interface{}{}
		item["key"] = key
		for nestedKey, nestedValue := range nestedMap {
			item[nestedKey] = nestedValue
		}
		rawData, err := json.Marshal(item)
		if err != nil {
			return err
		}

		rawData = append(rawData, []byte(",")...)
		_, err = jsonFile.Write(rawData)
		if err != nil {
			return err
		}
	}

	_, err = jsonFile.Write([]byte("]"))
	return err
}

func userKey(in apiv1.PerUserAPIRequestCount) string {
	return in.UserName + "---" + in.UserAgent
}

func pivotUserToResource(resourceName string, resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]resourceUsage {
	userToResourceCount := map[string]resourceUsage{}

	for _, resourceRequest := range resourceRequests {
		for _, resourceRequestByNode := range resourceRequest.ByNode {
			for _, resourceRequestByUser := range resourceRequestByNode.ByUser {
				key := userKey(resourceRequestByUser)
				currentRequestCount := userToResourceCount[key]
				currentRequestCount.resource = resourceName
				currentRequestCount.count += resourceRequestByUser.RequestCount
				userToResourceCount[key] = currentRequestCount
			}
		}
	}

	return userToResourceCount
}

// filter
func byResource(resourceName string, resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]map[string]int64 {
	ret := map[string]map[string]int64{}

	byUserMap := byUser(resourceRequests)
	ret[resourceName] = byUserMap

	return ret
}

// helper
func byUser(resourceRequests []apiv1.PerResourceAPIRequestLog) map[string]int64 {
	ret := map[string]int64{}

	for _, resourceRequest := range resourceRequests {
		for _, resourceRequestByNode := range resourceRequest.ByNode {
			for _, resourceRequestByUser := range resourceRequestByNode.ByUser {
				currentRequestCount := ret[resourceRequestByUser.UserAgent]
				ret[resourceRequestByUser.UserName] = currentRequestCount + resourceRequestByUser.RequestCount
			}
		}
	}

	return ret
}

// helper
func mergeMaps(prev map[string]map[string]int64, current map[string]map[string]int64) {
	for currentKey, currentNestedMap := range current {
		prevNestedMap := prev[currentKey]
		if prevNestedMap == nil {
			prevNestedMap = map[string]int64{}
		}
		for currentNestedKey, currentNestedValue := range currentNestedMap {
			prevNestedValue := prevNestedMap[currentNestedKey]
			prevNestedMap[currentNestedKey] = prevNestedValue + currentNestedValue
		}
		prev[currentKey] = prevNestedMap
	}
}

func showAllRequestHistory(requestStatus apiv1.APIRequestCountStatus) []apiv1.PerResourceAPIRequestLog {
	return requestStatus.Last24h
}
func showCurrentHourOnly(requestStatus apiv1.APIRequestCountStatus) []apiv1.PerResourceAPIRequestLog {
	return []apiv1.PerResourceAPIRequestLog{requestStatus.CurrentHour}
}

func walkData(rootpath string, callBackFn func(counter *apiv1.APIRequestCount) error) error {
	err := filepath.Walk(rootpath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		rawData, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		counter := &apiv1.APIRequestCount{}
		if err := yaml.Unmarshal(rawData, counter); err != nil {
			return err
		}
		if err := callBackFn(counter); err != nil {
			return err
		}
		return nil
	})
	return err
}
