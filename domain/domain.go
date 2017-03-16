package domain

import (
	"fmt"
	"github.com/0-complexity/ORK/utils"
	"github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"time"
)

const connectionURI string = "qemu:///system"

var log = logging.MustGetLogger("ORK")

type DomainCPUMap map[string]uint64

type Domain struct {
	domain      libvirt.Domain
	memUsage    uint64
	cpuTime     float64
	cpuTimeDiff float64
}

func (d Domain) GetDomain() libvirt.Domain {
	return d.domain
}

func (d Domain) CPU() float64 {
	return d.cpuTimeDiff
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

	utils.LogToKernel(fmt.Sprintf("ORK: attempting to destroy machine %v\n", name))

	if err = dom.DestroyFlags(1); err != nil {
		utils.LogToKernel(fmt.Sprintf("ORK: error destroying machine %v\n", name))
		log.Error("Error destroying machine", name)
		return
	}

	utils.LogToKernel(fmt.Sprintf("ORK: successfully destroyed machine %v\n", name))
	log.Debug("Successfully destroyed domain ", name)
	return
}

func UpdateCache(c *cache.Cache) error {
	domains, err := getDomains()
	if err != nil {
		log.Debug("Error getting domains")
		return err
	}

	var cpuTimeDiff float64

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

		if d, ok := c.Get(name); ok {
			oldDomain := d.(Domain)
			defer oldDomain.domain.Free()
			cpuTimeDiff = domainCpuTime - oldDomain.cpuTime
		}

		c.Set(name, Domain{domain, info.MaxMem, domainCpuTime, cpuTimeDiff}, time.Minute)
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
