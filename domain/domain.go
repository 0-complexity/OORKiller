package domain

import (
	"time"

	"github.com/0-complexity/ORK/utils"
	"github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/shirou/gopsutil/cpu"
)

const connectionURI string = "qemu:///system"

var log = logging.MustGetLogger("ORK")

type DomainCPUMap map[string]uint64

type Domain struct {
	domain         libvirt.Domain
	memUsage       uint64
	cpuUtilization float64
	cpuTime        float64
	cpuAvailable   float64
}

func (d Domain) GetDomain() libvirt.Domain {
	return d.domain
}

func (d Domain) CPU() float64 {
	return d.cpuUtilization
}

func (d Domain) Memory() uint64 {
	return d.memUsage
}

func (d Domain) Priority() int {
	return 100
}

func (d Domain) Kill() {
	dom := d.domain
	name, err := dom.GetName()
	if err != nil {
		log.Error("Error getting domain name")
		name = "unknown"
	}

	utils.LogToKernel("ORK: attempting to destroy machine %v\n", name)

	if err = dom.DestroyFlags(1); err != nil {
		utils.LogToKernel("ORK: error destroying machine %v\n", name)
		log.Error("Error destroying machine", name)
		return
	}

	utils.LogToKernel("ORK: successfully destroyed machine %v\n", name)
	log.Debug("Successfully destroyed domain ", name)
	return
}

func UpdateCache(c *cache.Cache) error {
	domains, err := getDomains()
	if err != nil {
		log.Debug("Error getting domains")
		return err
	}

	var cpuUtilization float64

	for _, domain := range domains {
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
		domainCpuTime := float64(info.CpuTime)
		hostAvailableCPU, err := cpu.Times(false)
		if err != nil {
			log.Error("Error getting host cpu info")
			continue
		}
		totalAvailable := hostAvailableCPU[0].Total()

		if d, ok := c.Get(name); ok {
			oldDomain := d.(Domain)
			oldDomain.domain.Free()

			cpuUtilization = (domainCpuTime - oldDomain.cpuTime) / (totalAvailable - oldDomain.cpuAvailable)
		}

		c.Set(name, Domain{domain, info.MaxMem, cpuUtilization, domainCpuTime, totalAvailable}, time.Minute)
	}

	return nil
}

func getDomains() ([]libvirt.Domain, error) {
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
