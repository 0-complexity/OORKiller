package domain

import (
	"github.com/0-complexity/ORK/utils"
	libvirt "github.com/libvirt/libvirt-go"
	"github.com/op/go-logging"
)

const connectionURI string = "qemu:///system"

var log = logging.MustGetLogger("ORK")

// Domains is a list of libvirt.Domains
type Domains struct {
	Domains []libvirt.Domain
	Sort    func([]libvirt.Domain, int, int) bool
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

	return d.Sort(d.Domains, i, j)
}

// DomainByMem sorts domains by memory consumption in a descending order
func DomainsByMem(d []libvirt.Domain, i, j int) bool {
	diInfo, err := d[i].GetInfo()
	if err != nil {
		panic(err)
	}
	djInfo, err := d[j].GetInfo()
	if err != nil {
		panic(err)
	}
	return diInfo.MaxMem > djInfo.MaxMem
}

// DestroyDomains destroys domains to free up memory.
// systemCheck is the function used to determine if the system state is ok or not.
// sorter is the sorting function used to sort domains.
func DestroyDomains(systemCheck func() (bool, error), sorter func([]libvirt.Domain, int, int) bool) error {
	conn, err := libvirt.NewConnect(connectionURI)

	if err != nil {
		log.Debug("Error connecting to qemu")
		return err
	}
	defer conn.Close()

	doms, err := conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		log.Debug("Error listing domains")
		return err
	}

	domains := Domains{doms, sorter}
	defer domains.Free()

	err = utils.Sort(&domains)
	if err != nil {
		log.Debug("Error sorting domains")
		return err
	}

	for _, d := range domains.Domains {
		if memOk, memErr := systemCheck(); memErr != nil {
			return memErr
		} else if memOk {
			return nil
		}

		name, err := d.GetName()
		if err != nil {
			log.Warning("Error getting domain name")
			name = "unknown"
		}

		err = d.DestroyFlags(1)
		if err != nil {
			log.Warning("Error destroying machine", name)
			continue
		}
		log.Info("Successfully destroyed", name)

	}
	return nil
}
