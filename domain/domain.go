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
	name     string
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

func (d Domain) Name() string {
	return d.name
}

func (d Domain) Kill() error {
	dom := d.domain
	name, err := dom.GetName()
	if err != nil {
		log.Errorf("Error getting domain name: %v", err)
		name = "unknown"
	}

	utils.LogToKernel("ORK: attempting to destroy machine %v\n", name)

	if err = dom.DestroyFlags(1); err != nil {
		utils.LogToKernel("ORK: error destroying machine %v\n", name)
		log.Errorf("Error destroying machine %v: %v", name, err)
		return err
	}

	utils.LogToKernel("ORK: successfully destroyed machine %v\n", name)
	log.Infof("Successfully destroyed domain %v", name)
	return nil
}

func UpdateCache(c *cache.Cache) error {
	domains, err := getDomains()
	if err != nil {
		log.Error("Error getting domains")
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
		d, ok := c.Get(name)
		if ok {
			cachedDomain = d.(Domain)
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

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_RUNNING)
	if err != nil {
		log.Error("Error listing domains")
		return nil, err
	}

	return domains, nil
}
