package domain

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	libvirt "github.com/libvirt/libvirt-go"
	"github.com/patrickmn/go-cache"

	"gopkg.in/yaml.v2"
)

const aggSpan = 5

type operation string
type sample struct {
	Avg   float64 `json:"avg"`
	Total float64 `json:"total"`
	Max   float64 `json:"max"`
	Count uint    `json:"count"`
	Start int64   `json:"start"`
}
type samples map[string]*sample
type history map[string][]sample

type state struct {
	Operation operation `json:"op"`
	LastValue float64   `json:"last_value"`
	LastTime  int64     `json:"last_time"`
	Current   samples   `json:"current"`
	History   history   `json:"history"`
}

func getStatistics(key string) (map[string]state, error) {
	var stats map[string]state
	out, err := exec.Command("corectl", "statistics", key).Output()
	if err != nil {
		return stats, err
	}
	if err := yaml.Unmarshal(out, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

func getCachedDomain(key string, c *cache.Cache) (*Domain, error) {
	cachedDomain := &Domain{
		releaseFactor: 1,
	}
	splits := strings.Split(key, "/")
	if len(splits) != 2 {
		message := fmt.Sprintf("Statistics key %v doesn't match the expected format", key)
		log.Error(message)
		return cachedDomain, fmt.Errorf(message)
	}

	if d, ok := c.Get(splits[1]); ok {
		cachedDomain = d.(*Domain)
	}
	cachedDomain.name = splits[1]
	return cachedDomain, nil
}

func addDomainMemory(c *cache.Cache) error {
	stats, err := getStatistics("kvm.memory.max")
	if err != nil {
		log.Errorf("Error getting domains memory statistics: %v", err)
		return err
	}
	for key, stat := range stats {
		cachedDomain, err := getCachedDomain(key, c)
		if err != nil {
			continue
		}
		cachedDomain.memUsage = stat.LastValue
		c.Set(cachedDomain.name, cachedDomain, time.Minute)
	}
	return nil
}

func addDomainCPU(c *cache.Cache) error {
	stats, err := getStatistics("kvm.cpu.time")
	if err != nil {
		log.Errorf("Error getting domains cpu statistics: %v", err)
		return err
	}
	for key, stat := range stats {
		cachedDomain, err := getCachedDomain(key, c)
		if err != nil {
			log.Error(err)
			continue
		}

		if _, ok := stat.Current["300"]; ok {
			cachedDomain.cpuTime = stat.Current["300"].Total / float64(time.Now().Unix()-stat.Current["300"].Start)
		}
		c.Set(cachedDomain.name, cachedDomain, time.Minute)
	}
	return nil
}

func addCpuAggregation(c *cache.Cache) error {
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Errorf("Error connecting to qemu: %v", err)
		return err
	}
	defer conn.Close()

	items := c.Items()
	for _, item := range items {
		d, ok := item.Object.(*Domain)
		// This is not a domain or it is a domain but it is not in release state
		// no need to calculate cpu agg.
		if !ok || !d.release {
			d.cpuAgg = cpuAggregation{}
			continue
		}

		// Aggregation has already been measured
		if d.release  && d.cpuAgg.end.timestamp != 0 {
			continue
		}

		dom, err := conn.LookupDomainByName(d.name)
		if err != nil {
			log.Errorf("Error looking up domain by name: %v", err)
			continue
		}
		defer dom.Free()

		info, err := dom.GetInfo()
		if err != nil {
			log.Errorf("Error getting domain info: %v", err)
			continue
		}

		timestamp := time.Now().Unix()
		if (d.cpuAgg == cpuAggregation{}) {
			d.cpuAgg.start.timestamp = timestamp
			d.cpuAgg.start.totalTime = float64(info.CpuTime) / 1000000000.
			c.Set(d.name, d, time.Minute)
			continue
		}

		if d.cpuAgg.end.timestamp == 0 && (timestamp-d.cpuAgg.start.timestamp) >= aggSpan {
			d.cpuAgg.end.timestamp = timestamp
			d.cpuAgg.end.totalTime = float64(info.CpuTime) /1000000000.
			c.Set(d.name, d, time.Minute)
		}
	}
	return nil
}

func UpdateCache(c *cache.Cache) {
	addDomainCPU(c)
	addDomainMemory(c)
	addCpuAggregation(c)
}
