package memory

import (
	"github.com/0-complexity/ORK/domain"
	"github.com/0-complexity/ORK/process"
	libvirt "github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/mem"
	"sort"
)

// thresholdPercent is the percent at which ORK should kill processes
const thresholdPercent float64 = 20.0

const connectionURI string = "qemu:///system"

var log = logging.MustGetLogger("ORK")

func Sort(i interface{}) error {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err, _ = r.(error)
		}
	}()

	switch t := i.(type) {
	case *domain.Domains:
		sort.Sort(domain.Domains(*t))
	case *process.Processes:
		sort.Sort(process.Processes(*t))
	}

	return err
}

func Monitor() error {
	log.Info("Monitoring memory")

	v, err := mem.VirtualMemory()

	if err != nil {
		log.Debug("Debug listing virtual memory")
		return err
	}

	if v.UsedPercent > thresholdPercent {
		log.Info("ALERT: reached memory threshold!!!")

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

		err = Sort(&domains)
		if err != nil {
			log.Debug("Error sorting domains")
			return err
		}

		for _, d := range domains.Domains {
			name, err := d.GetName()

			if err != nil {
				log.Debug("Error getting domain name")
				return err
			}

			err = d.DestroyFlags(1)
			if err != nil {
				log.Debug("Error destroying machine", name)
				return err
			}
			log.Info("Successfully destroyed", name)

			v, _ = mem.VirtualMemory()

			if v.UsedPercent < thresholdPercent {
				return nil
			}
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
		Sort(&processesStruct)

		for _, p := range processesStruct.Processes {
			killable, err := process.IsProcessKillable(p, processesMap, whiteListPids)
			if err != nil {
				log.Debug("Error checking is process is killable")
				return err
			}

			if !killable {
				continue
			}

			name, err := p.Name()
			if err != nil {
				log.Debug("Error getting process name")
				return err
			}

			err = p.Kill()
			if err != nil {
				log.Debug("Error killing process", p.Pid)
				return err
			}

			log.Info("Successfully killed process", p.Pid, name)

			v, _ = mem.VirtualMemory()

			if v.UsedPercent < thresholdPercent {
				log.Info("Memory consumption back to normal")
				return nil
			}
		}
	}
	return nil
}
