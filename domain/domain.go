package domain

import (
	"fmt"
	"github.com/0-complexity/ORK/utils"
	libvirt "github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"os/exec"
	"strings"
)

const connectionURI string = "qemu:///system"
const CPUTimeKey = "domainsCPUTime"
const CPUDiffKey = "domainsCPUDiff"

var log = logging.MustGetLogger("ORK")

type Sorter func(*Domains, int, int) bool
type DomainCPUMap map[string]uint64

// Domains is a list of libvirt.Domains
type Domains struct {
	Domains []libvirt.Domain
	Sort    Sorter
	CPUDiff DomainCPUMap
}

func (d *Domains) Free() {
	for _, domain := range d.Domains {
		domain.Free()
	}
}

func (d *Domains) Len() int { return len(d.Domains) }
func (d *Domains) Swap(i, j int) {
	d.Domains[i], d.Domains[j] = d.Domains[j], d.Domains[i]
}

func (d *Domains) Less(i, j int) bool {

	return d.Sort(d, i, j)
}

// DomainByMem sorts domains by memory consumption in a descending order
func DomainsByMem(d *Domains, i, j int) bool {
	diInfo, err := d.Domains[i].GetInfo()
	if err != nil {
		panic(err)
	}
	djInfo, err := d.Domains[j].GetInfo()
	if err != nil {
		panic(err)
	}
	return diInfo.MaxMem > djInfo.MaxMem
}

// DomainsByCPU sorts domains by cpu consumption in a descending order
func DomainsByCPU(d *Domains, i, j int) bool {
	diName, err := d.Domains[i].GetName()
	if err != nil {
		panic(err)
	}
	djName, err := d.Domains[j].GetName()
	if err != nil {
		panic(err)
	}
	diCPUDiff, inMap := d.CPUDiff[diName]
	if inMap == false {
		diCPUDiff = 0
	}
	djCPUDiff, inMap := d.CPUDiff[djName]
	if inMap == false {
		djCPUDiff = 0
	}
	return diCPUDiff > djCPUDiff
}

func SetDomainCPUTime(c *cache.Cache) error {
	doms, err := GetDomains()
	if err != nil {
		log.Error("Error getting domains")
		return err
	}

	domains := Domains{doms, nil, nil}
	defer domains.Free()

	var oldCPUTime DomainCPUMap
	CPUTime := make(DomainCPUMap)
	CPUDiff := make(DomainCPUMap)
	timeMap, ok := c.Get(CPUTimeKey)

	if ok {
		oldCPUTime = timeMap.(DomainCPUMap)
	}

	for _, domain := range domains.Domains {
		name, err := domain.GetName()
		if err != nil {
			log.Error("Error getting domain name")
			continue
		}

		info, err := domain.GetInfo()
		if err != nil {
			log.Error("Error getting domain info")
			continue
		}

		CPUTime[name] = info.CpuTime

		if ok {
			if oldTime, found := oldCPUTime[name]; found == true {
				CPUDiff[name] = info.CpuTime - oldTime
			}
		}
	}
	if ok {
		c.Set(CPUDiffKey, CPUDiff, cache.NoExpiration)
	}
	c.Set(CPUTimeKey, CPUTime, cache.NoExpiration)

	return nil
}

func GetDomainCPUMap() (DomainCPUMap, error) {
	cmd := exec.Command("bash", "-c", "ps aux --sort=-pcpu | grep qemu-system | grep -v grep | sed 's/.*-name \\(\\w*\\) .*/\\1/'")
	out, err := cmd.Output()
	if err != nil {
		log.Error("Error running bash commans")
		return nil, err
	}

	domains := strings.Split(string(out), "\n")
	domainsMap := make(map[string]uint64)

	for index, domain := range domains {
		domainsMap[domain] = uint64(index)
	}
	return domainsMap, nil
}

func GetDomains() ([]libvirt.Domain, error) {
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Debug("Error connecting to qemu")
		return nil, err
	}
	defer conn.Close()

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		log.Debug("Error listing domains")
		return nil, err
	}

	return domains, nil
}

// DestroyDomains destroys domains to free up memory.
// systemCheck is the function used to determine if the system state is ok or not.
// sorter is the sorting function used to sort domains.
func DestroyDomains(systemCheck func() (bool, error), sorter Sorter, domainsCPUDiff DomainCPUMap) error {

	doms, err := GetDomains()
	if err != nil {
		log.Error("Error getting domains")
		return err
	}

	domains := Domains{doms, sorter, domainsCPUDiff}
	defer domains.Free()

	err = utils.Sort(&domains)
	if err != nil {
		log.Debug("Error sorting domains")
		return err
	}

	for _, d := range domains.Domains {
		if sysOk, sysErr := systemCheck(); sysErr != nil {
			return sysErr
		} else if sysOk {
			return nil
		}

		name, err := d.GetName()
		if err != nil {
			log.Error("Error getting domain name")
			name = "unknown"
		}

		utils.LogToKernel(fmt.Sprintf("ORK: attempting to destroy machine %v\n", name))
		err = d.DestroyFlags(1)
		if err != nil {
			utils.LogToKernel(fmt.Sprintf("ORK: error destroying machine %v\n", name))
			log.Error("Error destroying machine", name)
			continue
		}
		utils.LogToKernel(fmt.Sprintf("ORK: successfully destroyed machine %v\n", name))
		log.Info("Successfully destroyed", name)

	}
	return nil
}
