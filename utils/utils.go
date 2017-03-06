package utils

import (
	"github.com/op/go-logging"
	"os"
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

func LogToKernel(message string) {
	f, err := os.OpenFile("/dev/kmsg", os.O_WRONLY|os.O_APPEND, 0544)

	if err != nil {
		log.Error("Error opening /dev/kmsg")
		return
	}

	defer f.Close()

	if _, err := f.WriteString(message); err != nil {
		log.Error("Error writing logs bla to /dev/kmsg", err)
	}
}
