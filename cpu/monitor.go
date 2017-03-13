// Package cpu implements cpu monitoring
package cpu

import (
	"github.com/0-complexity/ORK/domain"
	"github.com/0-complexity/ORK/process"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	ps_cpu "github.com/shirou/gopsutil/cpu"
)

const cpuThreshold float64 = 20.0 // ThresholdPercent hold the value of the memory threshold
var log = logging.MustGetLogger("ORK")

// isCPUOk returns a true if the CPU consumption is below the defined threshold
func isCPUOk() (bool, error) {
	percent, err := ps_cpu.Percent(0, false)
	if err != nil {
		log.Debug("Error getting available memory")
		return false, err
	}

	if percent[0] < cpuThreshold {
		log.Debug("CPU consumption is below threshold: ", percent[0])
		return true, nil
	}

	log.Warning("CPU consumption is above threshold", percent[0])
	return false, nil

}

// Monitor checks the cpu consumption and if it exceeds the defined threshold it kills
// virtual machines and processes until the consumption is bellow the threshold.
func Monitor(c *cache.Cache) error {
	log.Info("Monitoring CPU")

	// Check if the CPUDiff cache exists
	timeMap, ok := c.Get(domain.CPUDiffKey)

	if ok == false {
		log.Info("Domains CPU Diff hasn't been computed yet")
		return nil
	}
	domainCPUDiff := timeMap.(domain.DomainCPUMap)

	if cpuOk, err := isCPUOk(); err != nil {
		return err
	} else if cpuOk {
		// CPU consumption is below threshold, there is no need to kill domains/processes
		return nil
	}

	// CPU consumption is above threshold.
	// Destroy domains and re-check the cpu usage
	log.Info(domainCPUDiff)
	if err := domain.DestroyDomains(isCPUOk, domain.DomainsByCPU, domainCPUDiff); err != nil {
		return err
	}

	if cpuOk, err := isCPUOk(); err != nil {
		return err
	} else if cpuOk {
		return nil
	}

	// Check if the CPUDiff cache exists
	usageMap, ok := c.Get(process.ProcessesCPUKey)

	if ok == false {
		log.Info("Prcesses CPU Usage hasn't been computed yet")
		return nil
	}

	processesCPUUsage, _ := usageMap.(process.CPUUsageMap)

	// CPU consumption is still above threshold.
	// Kill processes and re-check the memory
	if err := process.KillProcesses(isCPUOk, process.ProcessesByMem, processesCPUUsage); err != nil {
		return err
	}

	if _, err := isCPUOk(); err != nil {
		return err
	} else {
		return nil
	}
}
