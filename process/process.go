package process

import (
	"fmt"
	"strings"

	"github.com/0-complexity/ORK/utils"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/shirou/gopsutil/process"
)

const ProcessesKey = "processes"
const ProcessesCPUKey = "processesCPU"

var log = logging.MustGetLogger("ORK")

// whiteListNames is slice of processes names that should never be killed.
var whitelistNames = []string{"jsagent.py", "volumedriver", "ORK"}

type Sorter func(*Processes, int, int) bool
type processesMap map[int32]*process.Process
type CPUUsageMap map[int32]float64

// Processes is a struct of a list of process.Process and a function to be
// used to sort the list.
type Processes struct {
	Processes []*process.Process
	Sort      Sorter
	CPUUsage  CPUUsageMap
}

func (p *Processes) Len() int { return len(p.Processes) }
func (p *Processes) Swap(i, j int) {
	p.Processes[i], p.Processes[j] = p.Processes[j], p.Processes[i]
}

func (p *Processes) Less(i, j int) bool {

	return p.Sort(p, i, j)
}

// ProcessesByMem sorts processes by memory consumption in a descending order
func ProcessesByMem(p *Processes, i, j int) bool {
	iInfo, err := p.Processes[i].MemoryInfo()
	if err != nil {
		panic(err)
	}
	jInfo, err := p.Processes[j].MemoryInfo()
	if err != nil {
		panic(err)
	}
	return iInfo.RSS > jInfo.RSS
}

// ProcessesByCPU sorts processes by cpu consumption in a descending order
func ProcessesByCPU(p *Processes, i, j int) bool {
	piUsage, inMap := p.CPUUsage[p.Processes[i].Pid]
	if inMap == false {
		piUsage = 0
	}
	pjUsage, inMap := p.CPUUsage[p.Processes[j].Pid]
	if inMap == false {
		pjUsage = 0
	}
	return piUsage > pjUsage
}

func SetProcessCPUUsage(c *cache.Cache) error {
	processes, err := MakeProcesses()
	if err != nil {
		log.Error("Error getting processes")
		return err
	}
	newMap := MakeProcessesMap(processes)

	var oldMap processesMap
	cpuUsage := make(CPUUsageMap)
	pMap, ok := c.Get(ProcessesKey)

	if ok {
		oldMap = pMap.(processesMap)
	}

	for pid, _ := range newMap {
		if ok {
			if oldProcess, found := oldMap[pid]; found == true {
				newMap[pid] = oldProcess
			}
		}
		percent, err := newMap[pid].Percent(0)
		if err != nil {
			log.Error("Error getting process cpu percentage")
			continue
		}
		cpuUsage[pid] = percent
	}

	c.Set(ProcessesCPUKey, cpuUsage, cache.NoExpiration)
	c.Set(ProcessesKey, newMap, cache.NoExpiration)

	return nil
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
func MakeProcessesMap(processes []*process.Process) processesMap {
	pMap := make(processesMap)

	for _, p := range processes {
		pMap[p.Pid] = p

	}
	return pMap
}

// SetupWhiteList returns a map of pid and process.Process instance for whitelisted processes.
func SetupWhiteList(processes []*process.Process) (map[int32]*process.Process, error) {
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
func KillProcesses(systemCheck func() (bool, error), sorter Sorter, processescpuUsage CPUUsageMap) error {
	processes, err := MakeProcesses()
	if err != nil {
		log.Debug("Error listing processes")
		return nil

	}
	processesMap := MakeProcessesMap(processes)
	whiteListPids, err := SetupWhiteList(processes)

	if err != nil {
		log.Debug("Error setting up processes whitelist")
		return err
	}

	processesStruct := Processes{processes, sorter, processescpuUsage}
	utils.Sort(&processesStruct)

	for _, p := range processesStruct.Processes {
		if sysOk, sysErr := systemCheck(); sysErr != nil {
			return sysErr
		} else if sysOk {
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
			log.Error("Error getting process name")
			name = "unknown"
		}

		utils.LogToKernel(fmt.Sprintf("ORK: attempting to kill process with pid %v and name %v\n", p.Pid, name))
		err = p.Kill()
		if err != nil {
			utils.LogToKernel(fmt.Sprintf("ORK: error killing process with pid %v and name %v\n", p.Pid, name))
			log.Error("Error killing process", p.Pid)
			continue
		}

		utils.LogToKernel(fmt.Sprintf("ORK: successfully killed process with pid %v and name %v\n", p.Pid, name))
		log.Info("Successfully killed process", p.Pid, name)
	}

	return nil
}