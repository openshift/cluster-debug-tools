package api_request_count

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"
)

func serializeToHTMLTemplate(data []byte, htmlTemplatePath string, outputFilePath string) error {
	htmlTemplate, err := ioutil.ReadFile(htmlTemplatePath)
	if err != nil {
		return err
	}
	// use string replacer for simple things
	r := strings.NewReplacer(
		"DATA_GOES_HERE", string(data),
	)
	htmlPage := []byte(r.Replace(string(htmlTemplate)))

	return ioutil.WriteFile(outputFilePath, htmlPage, 0644)
}

func serializeDataWithWriteOrder(data map[string]map[string]int64, writeOrder []string) ([]byte, error) {
	buffer := bytes.Buffer{}

	writeChunkFn := func(rawData []byte) error {
		_, err := buffer.Write(rawData)
		return err
	}

	if len(writeOrder) == 0 {
		for k, _ := range data {
			writeOrder = append(writeOrder, k)
		}
	}

	for _, writeKey := range writeOrder {
		if buffer.Len() != 0 {
			if err := writeChunkFn([]byte(",")); err != nil {
				return nil, err
			}
		}
		nestedMap := data[writeKey]
		rawData, err := serializeChunk(writeKey, nestedMap)
		if err != nil {
			return nil, err
		}
		if err := writeChunkFn(rawData); err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func serializeChunk(key string, nestedMap map[string]int64) ([]byte, error) {
	item := map[string]interface{}{}
	item["key"] = key
	for nestedKey, nestedValue := range nestedMap {
		item[nestedKey] = nestedValue
	}
	rawData, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	return rawData, nil
}
