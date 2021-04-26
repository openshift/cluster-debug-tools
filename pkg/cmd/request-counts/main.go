package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	apiv1 "github.com/openshift/api/apiserver/v1"
	"github.com/spf13/cobra"
)

func main() {
	// todo: add a list of prefixes that will be excluded
	cmd := NewCmdAPIRequestCount()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type APIRequestCountOptions struct {
	by string
}

func NewCmdAPIRequestCount() *cobra.Command {
	o := APIRequestCountOptions{}

	cmd := &cobra.Command{
		Use:          "todo",
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

	cmd.Flags().StringVar(&o.by, "by", o.by, "todo")

	return cmd
}

func (o *APIRequestCountOptions) Complete(command *cobra.Command, args []string) error {
	return nil
}

func (o *APIRequestCountOptions) Validate() error {
	return nil
}

func (o *APIRequestCountOptions) Run() error {
	// TODO: read flags
	// TODO: prepare filters

	ret := map[string]map[string]int64{}
	fmt.Println(walkData("/Users/lszaszki/go/src/github.com/p0lyn0mial/processor/apirequestcounts", func(counter *apiv1.APIRequestCount) error {
		resourceRequests := getRequestHistoryForTheLast(0, 0, true, counter.Status)
		byResourceRet := byResource(counter.Name, resourceRequests)
		mergeMaps(ret, byResourceRet)
		return nil
	}))

	// TODO: serialize to a file
	//fmt.Println(fmt.Sprintf("%v", ret))
	serializeToAFile(ret, "/Users/lszaszki/go/src/github.com/p0lyn0mial/processor/ret.json")
	fmt.Println("OK")
	return nil
}

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

// TODO: add support for specific range
func getRequestHistoryForTheLast(startIndex int, endIndex int, onlyCurrentHour bool, requestStatus apiv1.APIRequestCountStatus) []apiv1.PerResourceAPIRequestLog {
	if onlyCurrentHour {
		return []apiv1.PerResourceAPIRequestLog{requestStatus.CurrentHour}
	}
	ret := []apiv1.PerResourceAPIRequestLog{}
	return ret
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
