package api_request_count

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"
)

// this file contains methods that serialize data to a format required by the Nivo chart component.
//
// visit https://nivo.rocks/bar to find out more about the component.

func serializeToHTMLTemplate(data []byte, htmlTemplatePath string, outputFilePath string) error {
	htmlTemplate, err := ioutil.ReadFile(htmlTemplatePath)
	if err != nil {
		return err
	}

	r := strings.NewReplacer(
		"DATA_GOES_HERE", string(data),
	)
	htmlPage := []byte(r.Replace(string(htmlTemplate)))

	return ioutil.WriteFile(outputFilePath, htmlPage, 0644)
}

func serializeDataWithWriteOrder(data map[string]map[string]int64, primaryKeyWriteOrder []string, secondaryKeyOrderFn func(map[string]int64)[]string) ([]byte, error) {
	buffer := bytes.Buffer{}

	writeChunkFn := func(rawData []byte) error {
		_, err := buffer.Write(rawData)
		return err
	}

	for _, writeKey := range primaryKeyWriteOrder {
		if buffer.Len() != 0 {
			if err := writeChunkFn([]byte(",")); err != nil {
				return nil, err
			}
		}
		nestedMap := data[writeKey]
		rawData, err := serializeChunkWithWriteOrder(writeKey, nestedMap, secondaryKeyOrderFn(nestedMap))
		if err != nil {
			return nil, err
		}
		if err := writeChunkFn(rawData); err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func serializeChunkWithWriteOrder(primaryKey string, dataToSerialize map[string]int64, writeKeyOrder []string) ([]byte, error) {
	orderedItems := make(orderedMap, 0, len(dataToSerialize))
	orderedItems = append(orderedItems, kv{Key: "key", Val: primaryKey})

	for _, writeKey  := range writeKeyOrder {
		orderedItems = append(orderedItems, kv{Key: writeKey, Val: dataToSerialize[writeKey]})
	}

	rawData, err := json.Marshal(orderedItems)
	if err != nil {
		return nil, err
	}
	return rawData, nil
}
