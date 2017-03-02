package process

import (
	"fmt"
	"github.com/0-complexity/ORK/utils"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/process"
	"strings"
)

var log = logging.MustGetLogger("ORK")

// whiteListNames is slice of processes names that should never be killed.
var whitelistNames = []string{"jsagent.py", "volumedriver"}

// Processes is a struct of a list of process.Process and a function to be
// used to sort the list.
type Processes struct {
	Processes []*process.Process
	Sort      func([]*process.Process, int, int) bool
}

func (p *Processes) Len() int { return len(p.Processes) }
func (p *Processes) Swap(i, j int) {
	p.Processes[i], p.Processes[j] = p.Processes[j], p.Processes[i]
}

func (p *Processes) Less(i, j int) bool {

	return p.Sort(p.Processes, i, j)
}

// ProcessesByMem sorts processes by memory consumption in a descending order
func ProcessesByMem(p []*process.Process, i, j int) bool {
	iInfo, err := p[i].MemoryInfo()
	if err != nil {
		panic(err)
	}
	jInfo, err := p[j].MemoryInfo()
	if err != nil {
		panic(err)
	}
	return iInfo.RSS > jInfo.RSS
}

// MakeProcesses returns a list or process.Process instances
func MakeProcesses() ([]*process.Process, error) {
	processesIds, err := process.Pids()
	if err != nil {
		return nil, err
	}

	processes := make([]*process.Process, len(processesIds))
	for index, pid := range processesIds {
		p, _ := process.NewProcess(pid)
		processes[index] = p
	}
	return processes, nil
}

// MakeProcessesMap returns a map of process pid and process.Process instance for all running processes
func MakeProcessesMap(processes []*process.Process) map[int32]*process.Process {
	processesMap := make(map[int32]*process.Process)

	for _, p := range processes {
		processesMap[p.Pid] = p

	}
	return processesMap
}

// SetupWhiteList returns a map of pid and process.Process instance for whitelisted processes.
func SetupWhitelist(processes []*process.Process) (map[int32]*process.Process, error) {
	whiteList := make(map[int32]*process.Process)

	for _, p := range processes {
		processName, err := p.Name()
		if err != nil {
			log.Debug("Erorr getting process name")
			return nil, err
		}

		for _, name := range whitelistNames {
			if match := strings.Contains(processName, name); match {
				whiteList[p.Pid] = p
				break
			}
		}
	}
	kernelProcess, err := process.NewProcess(int32(2))
	if err != nil {
		log.Debug("Error getting kernel process with pid 2")
		return nil, err
	}

	whiteList[kernelProcess.Pid] = kernelProcess

	return whiteList, nil
}

// IsProcessKillable checks if a process can be killed or not.
// A process can't be killed if it is a member of the whiteList or if it is a child of a process in the
// whiteList.
func IsProcessKillable(p *process.Process, pMap map[int32]*process.Process, whiteList map[int32]*process.Process) (bool, error) {
	_, whiteListed := whiteList[p.Pid]
	if whiteListed || p.Pid == 1 {
		return false, nil
	}
	pPid, err := p.Ppid()

	for pPid != 0 {
		if err != nil {
			log.Debug("Error getting parent pid for pid", p.Pid)
			return false, err
		}
		if pPid == 1 {
			return true, nil
		}

		if _, ok := whiteList[pPid]; ok {
			return false, nil
		}

		p, inMap := pMap[pPid]
		if inMap != true {
			message := fmt.Sprintf("Error getting process %v from process map", p.Pid)
			log.Debug(message)
			return false, fmt.Errorf(message)

		}
		pPid, err = p.Ppid()
	}
	return true, nil
}

// KillProcesses kills processes to free up memory.
// systemCheck is the function used to determine if the system state is ok or not.
// sorter is the sorting function used to sort processes.
func KillProcesses(systemCheck func() (bool, error), sorter func([]*process.Process, int, int) bool) error {
	processes, err := MakeProcesses()
	if err != nil {
		log.Debug("Error listing processes")
		return nil

	}
	processesMap := MakeProcessesMap(processes)
	whiteListPids, err := SetupWhitelist(processes)

	if err != nil {
		log.Debug("Error setting up processes whitelist")
		return err
	}

	processesStruct := Processes{processes, sorter}
	utils.Sort(&processesStruct)

	for _, p := range processesStruct.Processes {
		if memOk, memErr := systemCheck(); memErr != nil {
			return memErr
		} else if memOk {
			return nil
		}

		killable, err := IsProcessKillable(p, processesMap, whiteListPids)
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
		if err != nil {
			log.Warning("Error killing process", p.Pid)
			continue
		}

		log.Info("Successfully killed process", p.Pid, name)
	}

	return nil
}
