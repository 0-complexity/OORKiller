package utils

import (
	"fmt"
	"github.com/op/go-logging"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
)

var log = logging.MustGetLogger("ORK")

// Sort is a wrapper for the sort.Sort function that recovers
// panic and returns it as an error.
func Sort(i sort.Interface) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
		}
	}()

	sort.Sort(i)
	return err
}

func LogToKernel(message string, a ...interface{}) {

	f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY|os.O_APPEND, 0544)

	if err != nil {
		log.Error("Error opening /dev/kmsg")
		return
	}

	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf(message, a...)); err != nil {
		log.Error("Error writing logs to /dev/kmsg:", err)
	}
}

func RemoveDirContents(path string) {
	log.Debugf("Deleting contents of %v", path)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Errorf("Error listing contents of dir %v: %v", path, err)
		return
	}

	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		if err = os.RemoveAll(filePath); err != nil {
			log.Errorf("Error removing %v", filePath)
		} else {
			log.Debugf("Successfully deleted %v", filePath)
		}
	}
}

func RemoveFilesWithPattern(pattern string) {
	log.Debugf("Removing files with pattern %v", pattern)
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Errorf("Error listing files matching the pattern %v: %v", pattern, err)
	}

	for _, file := range files {
		if info, err := os.Stat(file); err != nil {
			log.Errorf("Error getting stat for %v: %v", file, err)
			continue
		} else if info.IsDir() {
			log.Debugf("Skip deleting %v, it is not a file.")
			continue
		}

		if err = os.RemoveAll(file); err != nil {
			log.Errorf("Error removing %v", file)
			continue
		}
		log.Debugf("Successfully deleted %v", file)
	}

}
