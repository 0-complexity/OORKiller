package utils

import (
	"encoding/json"
	"fmt"
	"github.com/google/shlex"
	"github.com/op/go-logging"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strings"
)

var log = logging.MustGetLogger("ORK")

type NetworkUsage struct {
	Rxb, Txb, Rxp, Txp float64
}

type state string

const Success state = "SUCCESS"
const Error state = "ERROR"
const Warning state = "WARNING"

type action string

const NicShutdown action = "NIC_SHUTDOWN"
const Quarantine action = "VM_QUARANTINE"
const UnQuarantine action = "VM_UNQUARANTINE"

type message struct {
	Action action `json:"action"`
	Name   string `json:"name"`
	State  state  `json:"state"`
}

type kernelOptions map[string][]string

var kernelArgs kernelOptions
var dev bool = false
var monitorMem bool = true
var monitorCPU bool = true
var monitorNetwork bool = true
var monitorFairUsage bool = true

func init() {
	kernelArgs := getKernelOptions()

	log.Debugf("Kernel Args: %v", kernelArgs)

	if args, ok := kernelArgs["ork"]; ok {
		for _, arg := range args {
			if match, err := regexp.MatchString(`development`, arg); err != nil {
				log.Error(err)
				os.Exit(1)
			} else if match {
				dev = true
			}

			if match, err := regexp.MatchString(`nomem`, arg); err != nil {
				log.Error(err)
				os.Exit(1)
			} else if match {
				monitorMem = false
			}

			if match, err := regexp.MatchString(`nocpu`, arg); err != nil {
				log.Error(err)
				os.Exit(1)
			} else if match {
				monitorCPU = false
			}

			if match, err := regexp.MatchString(`nonetwork`, arg); err != nil {
				log.Error(err)
				os.Exit(1)
			} else if match {
				monitorNetwork = false
			}

			if match, err := regexp.MatchString(`nofairusage`, arg); err != nil {
				log.Error(err)
				os.Exit(1)
			} else if match {
				monitorFairUsage = false
			}
		}
	}
}

func MonitorCPU() bool {
	return monitorCPU
}

func MonitorMem() bool {
	return monitorMem
}

func MonitorNetwork() bool {
	return monitorNetwork
}

func MonitorFairUsage() bool {
	return monitorFairUsage
}

func Development() bool {
	return dev
}

// delta is a small closure over the counters, returning the delta against previous
// first = initial value
func Delta(first uint64) func(uint64) uint64 {
	keep := first
	return func(delta uint64) uint64 {
		v := delta - keep
		keep = delta
		return v
	}
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
		log.Errorf("Error writing to /dev/kmsg: %v", err)
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

	fmt.Println(fmt.Sprintf("20::%v", string(msg)))
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

func parseKernelOptions(content string) kernelOptions {
	options := kernelOptions{}
	cmdline, _ := shlex.Split(strings.TrimSpace(content))
	for _, option := range cmdline {
		kv := strings.SplitN(option, "=", 2)
		key := kv[0]
		value := ""
		if len(kv) == 2 {
			value = kv[1]
		}
		options[key] = append(options[key], value)
	}
	return options
}


func getKernelOptions() kernelOptions {
	content, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		log.Warning("Failed to read /proc/cmdline", err)
		return kernelOptions{}
	}

	return parseKernelOptions(string(content))
}
