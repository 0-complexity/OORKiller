package domain

import (
	"time"

	"bytes"
	"encoding/json"
	"os/exec"
	"strings"

	"fmt"
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
	memUsage float64
	cpuTime  float64
	name     string
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
type Samples map[int64]*Sample
type History map[int64][]Sample

type State struct {
	Operation Operation `json:"op"`
	LastValue float64   `json:"last_value"`
	LastTime  int64     `json:"last_time"`
	Tags      []Tag     `json:"tags,omitempty"`
	Current   Samples   `json:"current"`
	History   History   `json:"history"`
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
	cmd := exec.Command("corectl", "statistics", key)
	var out bytes.Buffer
	var stats map[string]State
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return stats, err
	}
	if err := json.Unmarshal(out.Bytes(), &stats); err != nil {
		return stats, err
	}
	return stats, nil
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
func getCachedDomain(key string, c *cache.Cache) (Domain, error) {
	var cachedDomain Domain
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

		cachedDomain.cpuTime = stat.Current[300].Avg
		c.Set(cachedDomain.name, cachedDomain, time.Minute)
	}
	return nil
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
