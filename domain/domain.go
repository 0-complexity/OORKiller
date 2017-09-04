package domain

import (
	"time"

	"os/exec"
	"strings"

	"fmt"

	"github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/utils"
	"gopkg.in/yaml.v2"
	"runtime"
)

const connectionURI string = "qemu:///system"
const oversubscription = 1

var log = logging.MustGetLogger("ORK")
var physCpus []int
var cpus map[int]*cpu
var quarantinedDomains map[string]interface{}

type cpu struct {
	count int
	vms   map[string]int
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
	_, ok := c.vms[name]
	if !ok {
		c.vms[name] = 0
	}
	c.vms[name] += count
}

const aggSpan = 5

type cpuUnit struct {
	timestamp int64
	totalTime uint64
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
	cpuAgg          cpuAggregation
}

type Operation string
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Sample struct {
	Avg   float64 `json:"avg"`
	Total float64 `json:"total"`
	Max   float64 `json:"max"`
	Count uint    `json:"count"`
	Start int64   `json:"start"`
}
type Samples map[string]*Sample
type History map[string][]Sample

type State struct {
	Operation Operation `json:"op"`
	LastValue float64   `json:"last_value"`
	LastTime  int64     `json:"last_time"`
	Current   Samples   `json:"current"`
	History   History   `json:"history"`
}

func init() {
	InitializeCPUs()
}

func InitializeCPUs() error {
	totalCpus := runtime.NumCPU()
	cpus = make(map[int]*cpu, totalCpus)
	hostCpus := 4
	if totalCpus <= 16 {
		hostCpus = 1
	} else if totalCpus <= 32 {
		hostCpus = 2
	}
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	domains, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_RUNNING)
	if err != nil {
		fmt.Println(err)
	}
	for i := hostCpus; i < totalCpus; i++ {
		physCpus = append(physCpus, i)
		cpus[i] = &cpu{vms: make(map[string]int, len(domains))}
	}

	for _, domain := range domains {
		pins, err := domain.GetVcpuPinInfo(libvirt.DOMAIN_AFFECT_LIVE)
		if err != nil {
			fmt.Println(err)
			return err
		}

		var pinCount int
		var physCpu int
		for _, vcpu := range pins {
			pinCount = 0
			for j, pinned := range vcpu {
				if pinned {
					pinCount += 1
					physCpu = j
				}
			}
			if pinCount != 1 {
				continue
			}
			name, err := domain.GetName()
			if err != nil {
				continue
			}
			quarantinedDomains[name] = struct{}{}
			cpus[physCpu].increment(name, 1)
		}

	}
	return nil
}

func (d Domain) Limit(warn int64, quarantine int64) {
	if !d.threshold {
		d.threshold = true
		d.thresholdStart = time.Now().Unix()
		return
	}
	if !d.warn && (time.Now().Unix()-d.thresholdStart) >= warn {
		// @todo: send email
		d.warn = true
		d.warnStart = time.Now().Unix()
		return
	}
	if !d.quarantine && (time.Now().Unix()-d.warnStart) >= quarantine {
		d.quarantine = true
		d.quarantineStart = time.Now().Unix()
		if _, ok := quarantinedDomains[d.name]; !ok {
			d.startQuarantine()
		}
	}
}

func (d Domain) UnLimit(releaseTime int64, threshold float64) {
	if !d.quarantine {
		fmt.Println("domain not quarantined")
		return
	}
	now := time.Now().Unix()
	if d.quarantine && !d.release && (now-d.quarantineStart) >= releaseTime * d.releaseFactor  {
		fmt.Println("releasing")
		fmt.Println(now-d.quarantineStart)
		fmt.Println(releaseTime)
		d.release = true
		d.releaseStart = time.Now().Unix()
		d.stopQuarantine()
		return
	}
	fmt.Println("release time not reached yet")
	if d.release && d.cpuAgg.end.timestamp != 0 {
		d.release = false
		agg := float64(d.cpuAgg.end.totalTime-d.cpuAgg.start.totalTime) / float64(d.cpuAgg.end.timestamp-d.cpuAgg.start.timestamp)
		if agg > threshold {
			fmt.Println("quaranting again")
			d.startQuarantine()
			d.quarantineStart = time.Now().Unix()
			d.releaseFactor = d.releaseFactor * 2
		} else {
			fmt.Println("releasing for good")
			d.release = false
			d.quarantine = false
			d.threshold = false
			d.warn = false
		}

	}
}

func Print() {
	for _, cpu := range physCpus {
		fmt.Println(fmt.Sprintf("CPU:%v, count: %v", cpu, cpus[cpu].count))
	}

}

func Test() {
	c := cache.New(cache.NoExpiration, time.Minute)
	d := Domain{name: "web_devel", quarantine: true, quarantineStart: time.Now().Unix(), releaseFactor:1}
	c.Set(d.name, d, time.Minute)
	d.UnLimit(5, 50)
	time.Sleep(5 * time.Second)
	d.UnLimit(5, 50)
	addCpuAggregation(c)
	time.Sleep(aggSpan)
	addCpuAggregation(c)
	d.UnLimit(5, 50)

	//d.stopQuarantine()
	//d = Domain{name: "web_devel2"}
	//d.stopQuarantine()
	//d = Domain{name: "web_devel3"}
	//d.stopQuarantine()
	//d = Domain{name: "web_devel4"}
	//d.stopQuarantine()
}
func TestQ() {
	d := Domain{name: "web_devel"}
	d.startQuarantine()
	d = Domain{name: "web_devel2"}
	d.startQuarantine()
	d = Domain{name: "web_devel3"}
	d.startQuarantine()
	d = Domain{name: "web_devel4"}
	d.startQuarantine()
}

func (d Domain) stopQuarantine() error {
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
	vcpus, err := dom.GetVcpusFlags(libvirt.DOMAIN_VCPU_LIVE)
	if err != nil {
		return err
	}
	totalCpus := runtime.NumCPU()
	cpuMap := make([]bool, totalCpus, totalCpus)
	for i, _ := range cpuMap {
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

func (d Domain) startQuarantine() error {
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
	vcpus, err := dom.GetVcpusFlags(libvirt.DOMAIN_VCPU_LIVE)
	if err != nil {
		return err
	}

	vcpu := int32(0)

	cpuPins := make(map[int][]int32, vcpus)
Outer:
	for _, cpu := range physCpus {
		available := oversubscription - cpus[cpu].count
		if available <= 0 {
			continue
		}
		fmt.Printf("available %v \n", available)
		cpuPins[cpu] = make([]int32, 0)
		for j := 0; j < available; j++ {
			cpuPins[cpu] = append(cpuPins[cpu], vcpu)
			vcpu += 1
			fmt.Printf("vcpu %v\n", vcpu)
			if vcpu == vcpus {
				fmt.Printf("cpupins %v \n", len(cpuPins[cpu]))
				break Outer //from outer loop
			}
		}
	}
	if vcpu != vcpus {
		return fmt.Errorf("Not enough cpu to pin %v vcpu for domain %v", vcpus, d.name)
	}

	totalCpus := runtime.NumCPU()
	for cpu, vcpus := range cpuPins {
		cpuMap := make([]bool, totalCpus, totalCpus)
		cpuMap[cpu] = true
		for _, vcpu := range vcpus {
			err := dom.PinVcpu(uint(vcpu), cpuMap)
			if err != nil {
				log.Errorf("Error pining vcpu %v for domain %v: %v", vcpu, d.name, err)
			}
		}
		fmt.Printf("vcpus pins %v\n", vcpus)
		cpus[cpu].increment(d.name, len(vcpus))
	}
	quarantinedDomains[d.name] = struct{}{}
	return nil
}

func (d Domain) CPUAverage() float64 {
	return d.cpuTime
}

func (d Domain) Memory() uint64 {
	return uint64(d.memUsage)
}

func (d Domain) Priority() int {
	return 100
}

func (d Domain) Name() string {
	return d.name
}

func (d Domain) Kill() error {
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

func getStatistics(key string) (map[string]State, error) {
	var stats map[string]State
	out, err := exec.Command("corectl", "statistics", key).Output()
	if err != nil {
		return stats, err
	}
	if err := yaml.Unmarshal(out, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

func getCachedDomain(key string, c *cache.Cache) (Domain, error) {
	cachedDomain := Domain{
		releaseFactor: 1,
	}
	splits := strings.Split(key, "/")
	if len(splits) != 2 {
		return cachedDomain, fmt.Errorf("Statistics key %v doesn't match the expected format", key)
	}
	d, ok := c.Get(splits[1])
	if ok {
		cachedDomain = d.(Domain)
	}
	cachedDomain.name = splits[1]
	return cachedDomain, nil
}

func addDomainMemory(c *cache.Cache) error {
	stats, err := getStatistics("kvm.memory.max")
	if err != nil {
		return err
	}
	for key, stat := range stats {
		cachedDomain, err := getCachedDomain(key, c)
		if err != nil {
			log.Error(err)
			continue
		}
		cachedDomain.memUsage = stat.LastValue
		c.Set(cachedDomain.name, cachedDomain, time.Minute)
	}
	return nil
}

func addDomainCPU(c *cache.Cache) error {
	stats, err := getStatistics("kvm.vcpu.time")
	if err != nil {
		return err
	}
	for key, stat := range stats {
		cachedDomain, err := getCachedDomain(key, c)
		if err != nil {
			log.Error(err)
			continue
		}

		if _, ok := stat.Current["300"]; ok {
			cachedDomain.cpuTime = stat.Current["300"].Avg
		}
		c.Set(cachedDomain.name, cachedDomain, time.Minute)
	}
	return nil
}

func addCpuAggregation(c *cache.Cache) {
	fmt.Println("********")
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Error("Error connecting to qemu")
		return
	}
	defer conn.Close()

	items := c.Items()
	for _, item := range items {
		timestamp := time.Now().Unix()
		d, ok := item.Object.(Domain)
		if !ok || !d.release {
			continue
		}
		dom, err := conn.LookupDomainByName(d.name)
		if err != nil {
			log.Error("Error looking up domain by name")
			continue
		}

		defer dom.Free()
		info, err := dom.GetInfo()
		if err != nil {
			log.Error("Error getting domain info")
			continue
		}

		if (d.cpuAgg == cpuAggregation{}) {
			fmt.Println("adding start")
			d.cpuAgg.start.timestamp = timestamp
			d.cpuAgg.start.totalTime = info.CpuTime
			c.Set(d.name, d, time.Minute)
			continue
		}

		if d.cpuAgg.end.timestamp == 0 && (timestamp-d.cpuAgg.start.timestamp) >= aggSpan {
			fmt.Println("adding end")

			d.cpuAgg.end.timestamp = timestamp
			d.cpuAgg.end.totalTime = info.CpuTime
			c.Set(d.name, d, time.Minute)

		}
	}
}

func UpdateCache(c *cache.Cache) error {
	err := addDomainCPU(c)
	if err != nil {
		return err
	}
	err = addDomainMemory(c)
	if err != nil {
		return err
	}
	return nil
}
