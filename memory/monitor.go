// Package memory implements a memory monitor
package memory

import (
	"github.com/0-complexity/ORK/domain"
	"github.com/0-complexity/ORK/process"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/mem"
)

// memoryThreshold is the value in MB at which ORK should free-up memory
const memoryThreshold uint64 = 100

var log = logging.MustGetLogger("ORK")

// isMemoryOk returns a true if the memory consumption is below the defined threshold
func isMemoryOk() (bool, error) {
	if v, err := mem.VirtualMemory(); err != nil {
		log.Debug("Error getting available memory")
		return false, err
	} else {
		if availableMem := v.Available / (1024 * 1024); availableMem > memoryThreshold {
			log.Debug("Memory consumption is below threshold")
			return true, nil
		}
		log.Warning("Memory consumption is above threshold")
		return false, nil
	}
}

// Monitor checks the memory consumption and if it exceeds the defined threshold it kills
// virtual machines and processes until the consumption is bellow the threshold.
func Monitor() error {
	log.Info("Monitoring memory")

	if memOk, err := isMemoryOk(); err != nil {
		return err
	} else if memOk {
		// Memory consumption is below threshold, there is no need to kill domains/processes
		return nil
	}

	// Memory consumption is above threshold.
	// Destroy domains and re-check the memory
	if err := domain.DestroyDomains(isMemoryOk, domain.DomainsByMem, nil); err != nil {
		return err
	}

	if memOk, err := isMemoryOk(); err != nil {
		return err
	} else if memOk {
		return nil
	}

	// Memory consumption is still above threshold.
	// Kill processes and re-check the memory
	if err := process.KillProcesses(isMemoryOk, process.ProcessesByMem, nil); err != nil {
		return err
	}

	if _, err := isMemoryOk(); err != nil {
		return err
	} else {
		return nil
	}
}
