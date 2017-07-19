package process

import (
	"fmt"
	"strings"
	"time"

	"github.com/zero-os/0-ork/utils"
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
	netUsage utils.NetworkUsage
}

func (p Process) CPU() float64 {
	return p.cpuUsage
}

func (p Process) Memory() uint64 {
	return p.memUsage
}

func (p Process) Network() utils.NetworkUsage {
	return p.netUsage
}

func (p Process) Priority() int {
	return 10
}

func (p Process) Kill() {
	proc := p.process
	pid := proc.Pid

	name, err := proc.Name()
	if err != nil {
		log.Error("Error getting process name")
		name = "unknown"
	}

	utils.LogToKernel("ORK: attempting to kill process with pid %v and name %v\n", pid, name)

	if err = proc.Kill(); err != nil {
		utils.LogToKernel("ORK: error killing process with pid %v and name %v\n", pid, name)
		log.Error("Error killing process", pid)
		return
	}

	utils.LogToKernel("ORK: successfully killed process with pid %v and name %v\n", pid, name)
	log.Debug("Successfully killed process", pid, name)
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
		log.Error("Error setting up processes ")
		return err
	}

	for pid, proc := range pMap {
		if killable, err := isProcessKillable(proc, pMap, whiteList); err != nil {
			log.Error("Error checking if process is killable")
			continue
		} else if killable == false {
			continue
		}

		key := string(pid)
		if p, ok := c.Get(key); ok == true {
			proc = p.(Process).process
		}

		percent, err := proc.Percent(0)
		if err != nil {
			log.Error("Error getting process cpu percentage")
			continue
		}

		memory, err := proc.MemoryInfo()
		if err != nil {
			log.Error("Error getting process memory info")
			continue
		}

		c.Set(key, Process{
			process:  proc,
			memUsage: memory.RSS,
			cpuUsage: percent,
		}, time.Minute)
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
			log.Debug("Erorr getting process name")
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
		log.Debug("Error getting parent pid for pid", p.Pid)
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
		log.Debug(message)
		return false, fmt.Errorf(message)
	}
	return isParentKillable(parent, pMap, whiteList)
}
