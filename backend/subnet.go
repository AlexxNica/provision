package backend

import (
	"bytes"
	"net"

	"github.com/digitalrebar/digitalrebar/go/common/store"
)

// Subnet represents a DHCP Subnet
// swagger:model
type Subnet struct {
	// Name is the name of the subnet.
	// Subnet names must be unique
	// required: true
	Name string
	// Subnet is the network address in CIDR form that all leases
	// acquired in its range will use for options, lease times, and NextServer settings
	// by default
	// required: true
	// pattern: ^([0-9]+\.){3}[0-9]+/[0-9]+$
	Subnet string
	// NextServer is the address of the next server
	// swagger:strfmt ipv4
	// required: true
	NextServer net.IP
	// ActiveStart is the first non-reserved IP address we will hand
	// non-reserved leases from.
	// swagger:strfmt ipv4
	// required: true
	ActiveStart net.IP
	// ActiveEnd is the last non-reserved IP address we will hand
	// non-reserved leases from.
	// swagger:strfmt ipv4
	// required: true
	ActiveEnd net.IP
	// ActiveLeaseTime is the default lease duration in seconds
	// we will hand out to leases that do not have a reservation.
	// required: true
	ActiveLeaseTime int32
	// ReservedLeasTime is the default lease time we will hand out
	// to leases created from a reservation in our subnet.
	// required: true
	ReservedLeaseTime int32
	// OnlyReservations indicates that we will only allow leases for which
	// there is a preexisting reservation.
	// required: true
	OnlyReservations bool
	// Options is the list of DHCP options that every lease in
	// this subnet will get.
	// required: true
	Options        []DhcpOption
	p              *DataTracker
	subnet         *net.IPNet
	nextLeasableIP net.IP
}

func (s *Subnet) Prefix() string {
	return "subnets"
}

func (s *Subnet) Key() string {
	return s.Name
}

func (s *Subnet) Backend() store.SimpleStore {
	return s.p.getBackend(s)
}

func (s *Subnet) New() store.KeySaver {
	return &Subnet{p: s.p}
}

func (p *DataTracker) NewSubnet() *Subnet {
	return &Subnet{p: p}
}

func (s *Subnet) List() []*Subnet {
	return AsSubnets(s.p.FetchAll(s))
}

func (s *Subnet) InSubnetRange(ip net.IP) bool {
	return s.subnet.Contains(ip)
}

func (s *Subnet) InActiveRange(ip net.IP) bool {
	return !s.OnlyReservations &&
		bytes.Compare(ip, s.ActiveStart) >= 0 &&
		bytes.Compare(ip, s.ActiveEnd) <= 0
}

func AsSubnet(o store.KeySaver) *Subnet {
	return o.(*Subnet)
}

func AsSubnets(o []store.KeySaver) []*Subnet {
	res := make([]*Subnet, len(o))
	for i := range o {
		res[i] = AsSubnet(o[i])
	}
	return res
}

func (s *Subnet) BeforeSave() error {
	e := &Error{Code: 422, Type: ValidationError, o: s}
	_, subnet, err := net.ParseCIDR(s.Subnet)
	if err != nil {
		e.Errorf("Invalid subnet %s: %v", s.Subnet, err)
	} else {
		s.subnet = subnet
		validateIP4(e, subnet.IP)
	}
	if !s.OnlyReservations {
		validateIP4(e, s.ActiveStart)
		validateIP4(e, s.ActiveEnd)
		if !subnet.Contains(s.ActiveStart) {
			e.Errorf("ActiveStart %s not in subnet range %s", s.ActiveStart, subnet)
		}
		if !subnet.Contains(s.ActiveEnd) {
			e.Errorf("ActiveEnd %s not in subnet range %s", s.ActiveEnd, subnet)
		}
		if s.ActiveLeaseTime < 60 {
			e.Errorf("ActiveLeaseTime must be greater than or equal to 60 seconds, not %d", s.ActiveLeaseTime)
		}
	}
	if s.ReservedLeaseTime < 7200 {
		e.Errorf("ReservedLeaseTime must be creater than or equal to 7200 seconds, not %d", s.ReservedLeaseTime)
	}
	return e.OrNil()
}
