package util

import (
	"os"
	"path/filepath"
	"regexp"
)

type FilterFunc func(s string) bool

// RegexFilter filter based off of regex.MatchString
func RegexFilter(re *regexp.Regexp) FilterFunc {
	return func(s string) bool {
		return re.MatchString(s)
	}
}

// ListFilesInDir will filter list of files based on full path of the files
// Ex. path /a/b/c/d.txt will be sent to filter function to be used
func ListFilesInDir(dir string, match FilterFunc) ([]string, error) {
	fileList := []string{}
	walk := func(p string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		if match(p) {
			fileList = append(fileList, p)
		}
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	return fileList, filepath.Walk(dir, walk)
}
