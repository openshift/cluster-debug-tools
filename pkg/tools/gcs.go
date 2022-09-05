package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	defaultBucket = "origin-ci-test"
	prowBaseURL   = `prow\.ci\.openshift\.org`
	gcsBaseURL    = `gcsweb-ci\.apps\.ci\.l2s4\.p1\.openshiftapps\.com`
	gcsRegVar     = "GCSPath"
)

var (
	linkURLRegex = regexp.MustCompile(fmt.Sprintf(`https:\/\/(%s|%s)(\/view)?/(gcs|gs)/(?P<%s>.*)`, prowBaseURL, gcsBaseURL, gcsRegVar))
)

type GCSClient struct {
	clientOptions []option.ClientOption
}

func NewGCSClient() *GCSClient {
	options := []option.ClientOption{
		option.WithoutAuthentication(),
	}
	return &GCSClient{
		clientOptions: options,
	}
}

func (g *GCSClient) FetchAndSaveArtifacts(link, dirName string, filter *regexp.Regexp) error {
	data, err := g.FetchArtifacts(link, filter)
	if err != nil {
		return fmt.Errorf("error handling link: %s", err)
	}
	return g.SaveArtifacts(dirName, data)
}

func (g *GCSClient) FetchArtifacts(link string, filter *regexp.Regexp) (foundObjects map[string]io.Reader, err error) {
	if !IsGCSLink(link) {
		return nil, fmt.Errorf("not gcs link")
	}

	client, err := storage.NewClient(context.Background(), g.clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("error creating GCS Client: %s", err)
	}

	bucket, artifactPath := getBucketAndObject(link)
	foundObjects = map[string]io.Reader{}
	foundObjectName := []string{}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	it := client.Bucket(bucket).Objects(ctx, &storage.Query{
		Prefix: artifactPath,
	})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if filter.MatchString(attrs.Name) {
			foundObjectName = append(foundObjectName, attrs.Name)
		}
	}

	if len(foundObjectName) == 0 {
		err = fmt.Errorf("could not find objects with specified regex: %s", filter.String())
		return
	}

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(foundObjectName))
	for _, objectPath := range foundObjectName {
		go func(bucket, op string) {
			data, err := g.getObject(client, bucket, op)
			if err != nil {
				fmt.Printf("error getObject(): %s\n", err)
			} else {
				mu.Lock()
				foundObjects[op] = data
				mu.Unlock()
			}
			wg.Done()
		}(bucket, objectPath)
	}
	wg.Wait()
	return
}

func (g *GCSClient) SaveArtifacts(dirName string, fileData map[string]io.Reader) error {
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return err
	}

	for fileName, data := range fileData {
		baseFileName := path.Base(fileName)
		trimmedFileName := stripJobPrefix(fileName)
		if err := os.MkdirAll(path.Join(dirName, path.Dir(trimmedFileName)), 0755); err != nil {
			return err
		}

		switch strings.ToLower(path.Ext(baseFileName)) {
		case ".tar":
			tarData := bytes.Buffer{}
			tarFile := io.TeeReader(data, &tarData)
			gfz, err := gzip.NewReader(tarFile)
			if err != nil {
				return err
			}
			tarReader := tar.NewReader(gfz)

			for {
				header, err := tarReader.Next()

				if err == io.EOF {
					break
				}

				if err != nil {
					return err
				}

				name := header.Name
				filePath := path.Join(dirName, trimmedFileName, name)
				switch header.Typeflag {
				case tar.TypeDir:
					if err := os.MkdirAll(filePath, 0755); err != nil {
						return err
					}
				case tar.TypeReg:
					if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
						return err
					}
					outFile, err := os.Create(filePath)
					if err != nil {
						return fmt.Errorf("tar: create failed: %s", err.Error())
					}
					if _, err := io.Copy(outFile, tarReader); err != nil {
						return fmt.Errorf("tar: copy failed: %s", err.Error())
					}
					outFile.Close()
				default:
					fmt.Printf("Skipping! Unable to figure out type : %c in file %s\n", header.Typeflag, name)
				}
			}
		default:
			filePath := path.Join(dirName, trimmedFileName)
			outFile, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("file: create failed: %s", err.Error())
			}
			if _, err := io.Copy(outFile, data); err != nil {
				return fmt.Errorf("file: copy failed: %s", err.Error())
			}
			outFile.Close()
		}
	}
	return nil
}

func (g *GCSClient) getObject(client *storage.Client, bucket, object string) (*storage.Reader, error) {
	obj := client.Bucket(bucket).Object(object)
	objReader, err := obj.NewReader(context.Background())
	if err != nil {
		return nil, fmt.Errorf("couldn't create the object reader: %w", err)
	}
	return objReader, nil
}

func IsGCSLink(link string) bool {
	return linkURLRegex.MatchString(link)
}

func stripJobPrefix(path string) string {
	parts := strings.SplitN(path, "/", 4)
	if len(parts) == 4 {
		return parts[3]
	}
	return path
}

func getBucketAndObject(link string) (string, string) {
	// Find index of our regex var
	varIndex := linkURLRegex.SubexpIndex(gcsRegVar)
	// Get list of all matches and make sure a value exists with in our bounds
	stringSubmatch := linkURLRegex.FindAllStringSubmatch(link, -1)
	if len(stringSubmatch) < 1 {
		return "", ""
	}
	if len(stringSubmatch[0])-1 < varIndex {
		return "", ""
	}
	// Use that index to get our value
	path := stringSubmatch[0][varIndex]
	path = strings.Trim(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
