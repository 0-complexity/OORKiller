package utils

import (
	"github.com/op/go-logging"
	"sort"
	"fmt"
	"os/exec"
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
	echo := fmt.Sprintf("echo \"%v\" > /dev/kmsg", message)
	log.Info(echo)
	cmdName := "sh"
	cmdArgs := []string{"-c", echo}
	if err := exec.Command(cmdName, cmdArgs...).Run(); err != nil {
		log.Warning("Error logging message in /dev/kmsg:", message)
	}
}
