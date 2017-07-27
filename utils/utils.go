package utils

import (
	"fmt"
	"github.com/op/go-logging"
	"os"
	"sort"
	"encoding/json"
)

var log = logging.MustGetLogger("ORK")

type NetworkUsage struct {
	Rxb, Txb, Rxp, Txp float64
}

type state string

const Success state = "SUCCESS"
const Error state = "ERROR"

type action string

const NicShutdown action = "NIC_SHUTDOWN"

type message struct {
	action action `json:"action"`
	name   string `json:"name"`
	state  state  `json:"state"`
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

func LogAction(action action, name string, state state) {
	message := message{
		action,
		name,
		state,
	}
	msg, err := json.Marshal(&message)
	if err != nil {
		return
	}

	fmt.Println(fmt.Sprintf("20::%v" , string(msg)))
	return

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
