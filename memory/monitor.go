// Package memory implements a memory monitor
package memory

import (
	"github.com/0-complexity/ORK/domain"
	"github.com/0-complexity/ORK/process"
	"github.com/0-complexity/ORK/utils"
	libvirt "github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/mem"
)

// memoryThreshold is the percent at which ORK should kill processes
const memoryThreshold uint64 = 100

const connectionURI string = "qemu:///system"
const normalConsumption = "Memory consumption back to normal"

var log = logging.MustGetLogger("ORK")

//getAvailableMemory returns available memory in MB
func getAvailableMemory() (uint64, error) {

	if v, err := mem.VirtualMemory(); err != nil {
		return 0, err
	} else {
		return v.Available / (1024 * 1024), nil
	}
}

// Monitor checks the memory consumption and if it exceeds the defined threshold it kills
// virtual machines and processes until the consumption is bellow the threshold.
func Monitor() error {
	log.Info("Monitoring memory")

	availableMemory, memErr := getAvailableMemory()
	if memErr != nil {
		log.Debug("Error getting available memory")
		return memErr
	}

	if availableMemory > memoryThreshold {
		return nil
	}

	log.Warning("Memory threshold reached")

	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Debug("Error connecting to qemu")
		return err
	}
	defer conn.Close()

	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		log.Debug("Error listing domains")
		return err
	}

	domains := domain.Domains{doms, domain.DomainsByMem}
	defer domains.Free()

	err = utils.Sort(&domains)
	if err != nil {
		log.Debug("Error sorting domains")
		return err
	}

	for i := 0; i < len(domains.Domains) && availableMemory <= memoryThreshold; i++ {
		if memErr!= nil {
			log.Debug("Error getting available memory")

		}

		d := domains.Domains[i]
		name, err := d.GetName()

		if err != nil {
			log.Warning("Error getting domain name")
			name = "unknown"
		}

		err = d.DestroyFlags(1)
		availableMemory, memErr = getAvailableMemory()

		if err != nil {
			log.Warning("Error destroying machine", name)
			continue
		}
		log.Info("Successfully destroyed", name)

	}

	if availableMemory > memoryThreshold {
		log.Info(normalConsumption)
		return nil
	}

	processes, err := process.MakeProcesses()
	if err != nil {
		log.Debug("Error listing processes")
		return nil

	}
	processesMap := process.MakeProcessesMap(processes)
	whiteListPids, err := process.SetupWhitelist(processes)

	if err != nil {
		log.Debug("Error setting up processes whitelist")
		return err
	}

	processesStruct := process.Processes{processes, process.ProcessesByMem}
	utils.Sort(&processesStruct)

	for i := 0; i < len(processesStruct.Processes) && availableMemory <= memoryThreshold; i++ {
		if memErr!= nil {
			log.Debug("Error getting available memory")

		}

		p := processesStruct.Processes[i]
		killable, err := process.IsProcessKillable(p, processesMap, whiteListPids)
		if err != nil {
			log.Debug("Error checking is process is killable")
			continue
		}

		if !killable {
			continue
		}

		name, err := p.Name()
		if err != nil {
			log.Warning("Error getting process name")
			name = "unknown"
		}

		err = p.Kill()
		availableMemory, memErr = getAvailableMemory()

		if err != nil {
			log.Warning("Error killing process", p.Pid)
			continue
		}

		log.Info("Successfully killed process", p.Pid, name)
	}

	if availableMemory > memoryThreshold {
		log.Info(normalConsumption)
	} else {
		log.Warning("Memory consumption still above threshold")
	}
	return nil
}
