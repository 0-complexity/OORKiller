package domain

import (
	"time"

	"github.com/VividCortex/ewma"
	"github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/utils"
)

const connectionURI string = "qemu:///system"

var log = logging.MustGetLogger("ORK")

type DomainCPUMap map[string]uint64

type Domain struct {
	domain   libvirt.Domain
	memUsage uint64
	netUsage utils.NetworkUsage
	cpuTime  ewma.MovingAverage
	cpuDelta func(uint64) uint64
}

func (d Domain) GetDomain() libvirt.Domain {
	return d.domain
}

func (d Domain) CPU() float64 {
	return d.cpuTime.Value()
}

func (d Domain) Memory() uint64 {
	return d.memUsage
}

func (d Domain) Network() utils.NetworkUsage {
	return d.netUsage
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
	log.Info("Successfully destroyed domain ", name)
	return
}

func UpdateCache(c *cache.Cache) error {
	domains, err := getDomains()
	if err != nil {
		log.Debug("Error getting domains")
		return err
	}

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
		var cachedDomain Domain
		log.Info(name)
		d, ok := c.Get(name)
		if ok {
			cachedDomain = d.(Domain)
			log.Info(cachedDomain.domain)
			cachedDomain.domain.Free()
			cachedDomain.cpuTime.Add(float64(cachedDomain.cpuDelta(info.CpuTime)))
		} else {
			cachedDomain = Domain{
				cpuDelta: utils.Delta(info.CpuTime),
				cpuTime:  ewma.NewMovingAverage(60),
			}
		}

		cachedDomain.domain = domain
		cachedDomain.memUsage = info.MaxMem
		c.Set(name, cachedDomain, time.Minute)
	}

	return nil
}

func getDomains() ([]libvirt.Domain, error) {
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Error("Error connecting to qemu")
		return nil, err
	}
	defer conn.Close()

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		log.Error("Error listing domains")
		return nil, err
	}

	return domains, nil
}
