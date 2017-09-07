package domain

import (
	"fmt"
	"runtime"
	"time"

	"github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/zero-os/0-ork/utils"
)

const connectionURI string = "qemu:///system"
const overSubscription = 4

var log = logging.MustGetLogger("ORK")

type cpu struct {
	count int
	vms   map[string]int
}

var physCpus []int
var cpus map[int]*cpu
var quarantinedDomains map[string]interface{}
var totalCpus = runtime.NumCPU()

func init() {
	quarantinedDomains = make(map[string]interface{})
	cpus = make(map[int]*cpu)

	// Determine the number of cpu cores to be reserved for the host
	hostCpus := 4
	if totalCpus <= 16 {
		hostCpus = 1
	} else if totalCpus <= 32 {
		hostCpus = 2
	}
	log.Debugf("Reserving %v cpus for host", hostCpus)

	for i := hostCpus; i < totalCpus; i++ {
		physCpus = append(physCpus, i)
		cpus[i] = &cpu{vms: make(map[string]int)}
	}
	InitializeCPUs()
}

func InitializeCPUs() {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		log.Errorf("Failed to initialize cpus: %v", err)
		return
	}
	defer conn.Close()

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_RUNNING)
	if err != nil {
		log.Errorf("Failed to initialize cpus: %v", err)
		return
	}

	for _, domain := range domains {
		name, err := domain.GetName()
		if err != nil {
			log.Errorf("Error getting domain's name: %v", err)
			continue
		}
		vcpus, err := domain.GetVcpuPinInfo(libvirt.DOMAIN_AFFECT_LIVE)
		if err != nil {
			log.Errorf("Error getting vcpu pin info for domain %v: %v", name, err)
			continue
		}

		var pinCount int
		var physCpu int
		for i, pins := range vcpus {
			pinCount = 0
			for j, pinned := range pins {
				if pinned {
					pinCount += 1
					physCpu = j
				}
			}
			if pinCount != 1 {
				continue
			}

			quarantinedDomains[name] = struct{}{}
			if _, ok := cpus[physCpu]; ok {
				log.Debugf("vcpu %v of domain %v is pinned to cpu %v", i, name, physCpu)
				cpus[physCpu].increment(name, 1)
			}
		}
	}
}

func (c *cpu) decrement(name string) {
	count, ok := c.vms[name]
	if !ok {
		return
	}
	c.count -= count
	delete(c.vms, name)
}

func (c *cpu) increment(name string, count int) {
	c.count += count
	if _, ok := c.vms[name]; !ok {
		c.vms[name] = 0
	}
	c.vms[name] += count
}

type cpuUnit struct {
	timestamp int64
	totalTime float64
}

type cpuAggregation struct {
	start cpuUnit
	end   cpuUnit
}

type Domain struct {
	domain          libvirt.Domain
	memUsage        float64
	cpuTime         float64
	name            string
	threshold       bool
	thresholdStart  int64
	warn            bool
	warnStart       int64
	quarantine      bool
	quarantineStart int64
	release         bool
	releaseStart    int64
	releaseFactor   int64
	postRelease     bool
	cpuAgg          cpuAggregation
}

func (d *Domain) Limit(warn int64, quarantine int64) {
	now := time.Now().Unix()

	if !d.threshold {
		log.Debugf("Domain %v is in threshold state", d.name)
		d.threshold = true
		d.thresholdStart = now
		return
	}

	if !d.warn && (now-d.thresholdStart) >= warn {
		log.Debugf("Domain %v is in warning state", d.name)
		utils.LogAction(utils.Quarantine, d.name, utils.Warning)
		d.warn = true
		d.warnStart = now
		return
	}

	if d.warn && !d.quarantine && (now-d.warnStart) >= quarantine {
		log.Debugf("Domain %v is in quarantine state", d.name)
		d.quarantine = true
		d.quarantineStart = time.Now().Unix()
		if _, ok := quarantinedDomains[d.name]; !ok {
			if err := d.startQuarantine(); err != nil {
				d.quarantine = false
			} else {
				utils.LogAction(utils.Quarantine, d.name, utils.Success)
			}
		}
	}
}

func (d *Domain) UnLimit(releaseTime int64, threshold float64) {
	now := time.Now().Unix()

	if !d.quarantine {
		// This domain was quarantined but the flags were reset due to ork restart
		// set quarantine to true and let it take the normal cycle
		if _, ok := quarantinedDomains[d.name]; ok {
			log.Debugf("setting quarantine flag")
			d.quarantine = true
			d.quarantineStart = now
			return
		}
		d.threshold = false
		d.warn = false
		d.release = false
		return
	}

	// Check if the domain is quarantined and is ready to be released
	if d.quarantine && !d.release && (now-d.quarantineStart) >= releaseTime*d.releaseFactor {
		log.Debugf("Testing domain %v release", d.name)
		d.release = true
		d.releaseStart = now
		if err := d.stopQuarantine(); err != nil {
			log.Debugf("Failed to release domain %v", d.name)
			d.release = false
		}
		return
	}

	// Check if the domain behaved well during the release window and release it for good if it did
	// or quarantine it again if it didn't
	if !d.postRelease && d.release && d.cpuAgg.end.timestamp != 0 {
		d.postRelease = true
		agg := float64(d.cpuAgg.end.totalTime-d.cpuAgg.start.totalTime) / float64(d.cpuAgg.end.timestamp-d.cpuAgg.start.timestamp)
		if agg >= threshold {
			d.releaseFactor = d.releaseFactor * 2
			log.Debugf("Domain %v is still misbehaving after release and will be put in quarantine again.", d.name)
			if err := d.startQuarantine(); err != nil {
				d.quarantine = false
			} else {
				d.quarantineStart = now
				utils.LogAction(utils.Quarantine, d.name, utils.Success)
			}
		} else {
			log.Debugf("Domain %v is released for good.", d.name)
			utils.LogAction(utils.UnQuarantine, d.name, utils.Success)
			d.quarantine = false
			d.threshold = false
			d.warn = false
		}
		d.release = false
		d.postRelease = false
		d.cpuAgg = cpuAggregation{}
	}
}

func (d *Domain) stopQuarantine() error {
	conn, err := libvirt.NewConnect(connectionURI)
	if err != nil {
		log.Errorf("Error removing %v from quarantine: %v", d.name, err)
		return err
	}
	defer conn.Close()

	dom, err := conn.LookupDomainByName(d.name)
	if err != nil {
		log.Errorf("Error removing %v from quarantine: %v", d.name, err)
		return err
	}
	defer dom.Free()

	vcpus, err := dom.GetVcpusFlags(libvirt.DOMAIN_VCPU_LIVE)
	if err != nil {
		log.Errorf("Error removing %v from quarantine: %v", d.name, err)
		return err
	}

	cpuMap := make([]bool, totalCpus, totalCpus)
	for i := range cpuMap {
		cpuMap[i] = true
	}

	for i := 0; i < int(vcpus); i++ {
		err := dom.PinVcpu(uint(i), cpuMap)
		if err != nil {
			log.Errorf("Error pining vcpu %v for domain %v: %v", i, d.name, err)
		}
	}
	for _, cpu := range cpus {
		cpu.decrement(d.name)
	}
	delete(quarantinedDomains, d.name)
	return nil
}

func (d *Domain) startQuarantine() error {
	conn, err := libvirt.NewConnect(connectionURI)
	if err != nil {
		log.Errorf("Error adding %v to quarantine: %v", d.name, err)
		return err
	}
	defer conn.Close()

	dom, err := conn.LookupDomainByName(d.name)
	if err != nil {
		log.Errorf("Error adding %v to quarantine: %v", d.name, err)
		return err
	}
	defer dom.Free()

	vcpus, err := dom.GetVcpusFlags(libvirt.DOMAIN_VCPU_LIVE)
	if err != nil {
		log.Errorf("Error adding %v to quarantine: %v", d.name, err)
		return err
	}

	vcpu := int32(0)
	cpuPins := make(map[int][]int32, vcpus)
Outer:
	for _, cpu := range physCpus {
		available := overSubscription - cpus[cpu].count
		if available <= 0 {
			continue
		}
		cpuPins[cpu] = make([]int32, 0)
		for j := 0; j < available; j++ {
			cpuPins[cpu] = append(cpuPins[cpu], vcpu)
			vcpu += 1
			if vcpu == vcpus {
				break Outer //from outer loop
			}
		}
	}

	if vcpu != vcpus {
		message := fmt.Sprintf("Not enough cpu to pin %v vcpu for domain %v", vcpus, d.name)
		log.Error(message)
		return fmt.Errorf(message)
	}

	for cpu, vcpus := range cpuPins {
		cpuMap := make([]bool, totalCpus, totalCpus)
		cpuMap[cpu] = true
		for _, vcpu := range vcpus {
			err := dom.PinVcpu(uint(vcpu), cpuMap)
			if err != nil {
				log.Errorf("Error pining vcpu %v for domain %v: %v", vcpu, d.name, err)
				d.stopQuarantine()
				return err
			}
		}

		cpus[cpu].increment(d.name, len(vcpus))
	}
	quarantinedDomains[d.name] = struct{}{}
	return nil
}

func (d *Domain) CPUAverage() float64 {
	return d.cpuTime
}

func (d *Domain) Memory() uint64 {
	return uint64(d.memUsage)
}

func (d *Domain) Priority() int {
	return 100
}

func (d *Domain) Name() string {
	return d.name
}

func (d *Domain) Kill() error {
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Error("Error connecting to qemu")
		return err
	}
	defer conn.Close()
	dom, err := conn.LookupDomainByName(d.name)
	if err != nil {
		log.Error("Error looking up domain by name")
		return err
	}
	defer dom.Free()

	utils.LogToKernel("ORK: attempting to destroy machine %v\n", d.name)

	if err = dom.DestroyFlags(1); err != nil {
		utils.LogToKernel("ORK: error destroying machine %v\n", d.name)
		log.Errorf("Error destroying machine %v: %v", d.name, err)
		return err
	}

	utils.LogToKernel("ORK: successfully destroyed machine %v\n", d.name)
	log.Infof("Successfully destroyed domain %v", d.name)
	return nil
}
