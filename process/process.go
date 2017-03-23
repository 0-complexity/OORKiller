package process

import (
	"fmt"
	"strings"
	"time"

	"github.com/0-complexity/ORK/utils"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/shirou/gopsutil/process"
)

var log = logging.MustGetLogger("ORK")

// whiteListNames is slice of processes names that should never be killed.
var whitelistNames = []string{"jsagent.py", "volumedriver", "ORK"}

type processesMap map[int32]*process.Process
type whiteListMap map[int32]bool

// Processes is a struct of a list of process.Process and a function to be
// used to sort the list.
type Process struct {
	process  *process.Process
	memUsage uint64
	cpuUsage float64
}

func (p Process) CPU() float64 {
	return p.cpuUsage
}

func (p Process) Memory() uint64 {
	return p.memUsage
}

func (p Process) Priority() int {
	return 10
}

func (p Process) Kill() {
	proc := p.process
	pid := proc.Pid

	name, err := proc.Name()
	if err != nil {
		log.Errorf("Error getting name of process %v", pid)
		name = "unknown"
	}

	utils.LogToKernel("ORK: attempting to kill process with pid %v and name %v\n", pid, name)

	if err = proc.Kill(); err != nil {
		utils.LogToKernel("ORK: error killing process with pid %v and name %v\n", pid, name)
		log.Errorf("Error killing process %v", pid)
		return
	}

	utils.LogToKernel("ORK: successfully killed process with pid %v and name %v\n", pid, name)
	log.Infof("Successfully killed process %v", pid)
	return
}
func UpdateCache(c *cache.Cache) error {
	pMap, err := makeProcessesMap()
	if err != nil {
		log.Error("Error getting processes")
		return err
	}

	whiteList, err := setupWhiteList(pMap)
	if err != nil {
		log.Error("Error setting up whitelisted processes")
		return err
	}

	for pid, proc := range pMap {
		if kill, err := isProcessKillable(proc, pMap, whiteList); err != nil {
			log.Errorf("Error checking if process %v is killable", pid)
			continue
		} else if kill == false {
			continue
		}

		key := string(pid)
		if p, ok := c.Get(key); ok == true {
			proc = p.(Process).process
		}

		percent, err := proc.Percent(0)
		if err != nil {
			log.Errorf("Error getting process %v cpu percentage", pid)
			continue
		}

		memory, err := proc.MemoryInfo()
		if err != nil {
			log.Errorf("Error getting process %v memory info", pid)
			continue
		}

		c.Set(key, Process{proc, memory.RSS, percent}, time.Minute)
	}

	return nil
}

// MakeProcessesMap returns a map of process pid and process.Process instance for all running processes
func makeProcessesMap() (processesMap, error) {
	pMap := make(processesMap)

	processesIds, err := process.Pids()
	if err != nil {
		return nil, err
	}

	for _, pid := range processesIds {
		p, err := process.NewProcess(pid)
		if err != nil {
			return nil, err
		}
		pMap[p.Pid] = p
	}

	return pMap, nil
}

// SetupWhiteList returns a map of pid and process.Process instance for whitelisted processes.
func setupWhiteList(pMap processesMap) (whiteListMap, error) {
	whiteList := make(whiteListMap)

	for _, p := range pMap {
		processName, err := p.Name()
		if err != nil {
			log.Errorf("Erorr getting process %v name", p.Pid)
			return nil, err
		}

		for _, name := range whitelistNames {
			if match := strings.Contains(processName, name); match {
				whiteList[p.Pid] = true
				break
			}
		}
	}
	whiteList[int32(2)] = true
	whiteList[int32(1)] = true

	return whiteList, nil
}

// IsProcessKillable checks if a process can be killed or not.
// A process can't be killed if it is a member of the whiteList or if it is a child of a process in the
// whiteList.
func isProcessKillable(p *process.Process, pMap processesMap, whiteList whiteListMap) (bool, error) {
	if whiteList[p.Pid] {
		return false, nil
	}
	return isParentKillable(p, pMap, whiteList)
}

func isParentKillable(p *process.Process, pMap processesMap, whiteList whiteListMap) (bool, error) {
	pPid, err := p.Ppid()
	if err != nil {
		log.Errorf("Error getting parent pid for process %v", p.Pid)
		return false, err
	}

	if pPid == 1 {
		return true, nil
	}

	if whiteList[pPid] {
		return false, nil
	}

	parent, inMap := pMap[pPid]
	if inMap != true {
		message := fmt.Sprintf("Error getting process %v from process map", p.Pid)
		log.Error(message)
		return false, fmt.Errorf(message)
	}
	return isParentKillable(parent, pMap, whiteList)
}
