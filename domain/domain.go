package domain

import (
	libvirt "github.com/libvirt/libvirt-go"
)

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

func (d Domains) Len() int { return len(d.Domains) }
func (d Domains) Swap(i, j int) {
	d.Domains[i], d.Domains[j] = d.Domains[j], d.Domains[i]
}

func (d Domains) Less(i, j int) bool {

	return d.Sort(d.Domains, i, j)
}

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
