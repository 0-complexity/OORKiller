package utils

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
	"sort"
)

var log = logging.MustGetLogger("ORK")

type NetworkUsage struct {
	Rxb, Txb, Rxp, Txp float64
}

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
		log.Error("Error writing logs bla to /dev/kmsg", err)
	}
}

//InList checks if x is in l
func InList(x string, l []string) bool {
	for i := 0; i < len(l); i++ {
		if l[i] == x {
			return true
		}
	}

	return false
}
