package process

import (
	"errors"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/process"
	"regexp"
)

var log = logging.MustGetLogger("ORK")

var whitelistNames = []string{"jsagent.py", "volumedriver"}

// Processes is a struct of a list of process.Process and a sorting function
type Processes struct {
	Processes []*process.Process
	Sort      func([]*process.Process, int, int) bool
}

func (p Processes) Len() int { return len(p.Processes) }
func (p Processes) Swap(i, j int) {
	p.Processes[i], p.Processes[j] = p.Processes[j], p.Processes[i]
}

func (p Processes) Less(i, j int) bool {

	return p.Sort(p.Processes, i, j)
}

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

func MakeProcessesMap(processes []*process.Process) map[int32]*process.Process {
	processesMap := make(map[int32]*process.Process)

	for _, p := range processes {
		processesMap[p.Pid] = p

	}
	return processesMap
}

func SetupWhitelist(processes []*process.Process) (map[int32]*process.Process, error) {
	whiteList := make(map[int32]*process.Process)

	for _, p := range processes {
		processName, err := p.Name()
		if err != nil {
			log.Debug("Erorr getting process name.")
			return nil, err
		}

		for _, name := range whitelistNames {

			match, err := regexp.MatchString(name, processName)
			if err != nil {
				log.Debug("Erorr matching process name.")
				return nil, err
			}
			if match {
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
		_, whiteListed = whiteList[pPid]

		if whiteListed {
			return false, nil
		}
		var inMap bool
		p, inMap = pMap[pPid]

		if inMap != true {
			log.Debug("Error getting process from map")
			return false, errors.New("Error")
		}
		pPid, err = p.Ppid()
	}
	return true, nil
}
