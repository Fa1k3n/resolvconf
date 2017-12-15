package resolvconf

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
)

// Conf represents a configuration object
type Conf struct {
	items  []ConfItem
	logger *log.Logger
}

// GetNameservers returns a list of all added nameservers
func (conf *Conf) GetNameservers() []Nameserver {
	var ret []Nameserver
	for _, item := range conf.items {
		if _, ok := item.(*Nameserver); ok {
			ret = append(ret, *item.(*Nameserver))
		}
	}
	return ret
}

// GetSortItems returns list of all added sortitems
func (conf *Conf) GetSortItems() []SortItem {
	var ret []SortItem
	for _, item := range conf.items {
		if _, ok := item.(*SortItem); ok {
			ret = append(ret, *item.(*SortItem))
		}
	}
	return ret
}

// GetDomain returns current domain
func (conf *Conf) GetDomain() Domain {
	for _, item := range conf.items {
		if d, ok := item.(*Domain); ok {
			return *d
		}
	}
	return Domain{}
}

// GetSearchDomains returns a list of all added SearchDomains
func (conf *Conf) GetSearchDomains() []SearchDomain {
	var ret []SearchDomain
	for _, item := range conf.items {
		if _, ok := item.(*SearchDomain); ok {
			ret = append(ret, *item.(*SearchDomain))
		}
	}
	return ret
}

// GetOptions returns a list of all added options
func (conf *Conf) GetOptions() []Option {
	var ret []Option
	for _, item := range conf.items {
		if _, ok := item.(*Option); ok {
			ret = append(ret, *item.(*Option))
		}
	}
	return ret
}

// ConfItem is the generic interface all items in a resolv.conf file
// must implement.
type ConfItem interface {
	fmt.Stringer
	applyLimits(conf *Conf) (bool, error)
	Equal(b ConfItem) bool
}

// Nameserver is the nameserver type
type Nameserver struct {
	IP net.IP // IP address
}

func (ns Nameserver) applyLimits(conf *Conf) (bool, error) {

	if len(conf.GetNameservers())+1 > 3 {
		return false, fmt.Errorf("Too many Nameserver configs, max is 3")
	}
	// Search if conf Nameserver is already added
	if conf.Find(ns) != nil {
		return false, fmt.Errorf("Nameserver %s already exists in conf", ns.IP)
	}

	return true, nil
}

// Equal compares to nameservers with eachother, returns true if equal
func (ns Nameserver) Equal(b ConfItem) bool {
	if item, ok := b.(*Nameserver); ok {
		return ns.IP.Equal(item.IP)
	}
	return false
}

func (ns Nameserver) String() string {
	return ns.IP.String()
}

// Domain is the single domain in a resolv.conf file
type Domain struct {
	Name string
}

func (dom Domain) applyLimits(conf *Conf) (bool, error) {
	i := conf.indexOf(conf.GetDomain())
	if i != -1 {
		// Found it, update and return not ok to add
		conf.items[i] = &Domain{dom.Name}
		return false, nil
	}

	// Ok to add
	return true, nil
}

func (dom Domain) String() string {
	return dom.Name
}

// Equal compares two domains with each other, returns true if equal
func (dom Domain) Equal(b ConfItem) bool {
	if item, ok := b.(*Domain); ok {
		return dom.Name == item.Name
	}
	return false
}

// SearchDomain is one of the items in the search list
type SearchDomain struct {
	Name string
}

func (sd SearchDomain) applyLimits(conf *Conf) (bool, error) {
	// Search if conf search domain is already added
	if conf.Find(sd) != nil {
		return false, fmt.Errorf("Search domain %s already exists in conf", sd.Name)
	}
	return true, nil
}

func (sd SearchDomain) String() string {
	return sd.Name
}

// Equal compares two search domains with each other, returns true if equal
func (sd SearchDomain) Equal(b ConfItem) bool {
	if item, ok := b.(*SearchDomain); ok {
		return sd.Name == item.Name
	}
	return false
}

// SortItem is one of the items in the sort list, it must have an address and
// may have an netmask
type SortItem struct {
	Address net.IP
	Netmask net.IP
}

func (si SortItem) applyLimits(conf *Conf) (bool, error) {
	if i := conf.Find(si); i != nil {
		return false, fmt.Errorf("Sortlist pair %s already exists in conf", si)
	}
	if len(conf.GetSortItems()) == 10 {
		return false, fmt.Errorf("Too long sortlist, 10 is maximum")
	}
	return true, nil
}

// Equal compares two SortItems, return true if equal
func (si SortItem) Equal(b ConfItem) bool {
	if item, ok := b.(*SortItem); ok {
		return si.Address.String() == item.Address.String()
	}

	return false
}

// SetNetmask sets the netmask for an SortItem
func (si *SortItem) SetNetmask(nm net.IP) *SortItem {
	si.Netmask = nm
	return si
}

// GetNetmask returns netmask from an SortItems
func (si SortItem) GetNetmask() net.IP {
	return si.Netmask
}

func (si SortItem) String() string {
	if len(si.Netmask) > 0 {
		return fmt.Sprintf("%s/%s", si.Address, si.Netmask)
	}
	return fmt.Sprintf("%s", si.Address)
}

// Option represents an option item which must have a Type
// and some options must have a value
type Option struct {
	Type  string
	Value int
}

func (opt Option) applyLimits(conf *Conf) (bool, error) {
	if opt.Type == "ndots" && opt.Value < 0 {
		return false, fmt.Errorf("Bad value %d", opt.Value)
	}
	if _, e := parseOption(opt.String()); e != nil {
		return false, fmt.Errorf("Unknown option %s", opt)
	}
	if o := conf.Find(opt); o != nil {
		return false, fmt.Errorf("Option %s is already present", opt)
	}
	return true, nil
}

// Equal compares two Option, return true if equal
func (opt Option) Equal(b ConfItem) bool {
	if o, ok := b.(*Option); ok {
		return opt.Type == o.Type
	}
	return false
}

// Set sets the value of an option
func (opt *Option) Set(value int) *Option {
	if value < 0 {
		return opt
	}
	opt.Value = value
	return opt
}

// Get returns the option value
func (opt Option) Get() int {
	return opt.Value
}

func (opt Option) String() string {
	switch opt.Type {
	case "debug", "rotate", "no-check-names", "inet6",
		"ip6-bytestring", "ip6-dotint", "no-ip6-dotint",
		"edns0", "single-request", "single-request-reopen",
		"no-tld-query", "use-vc":
		return fmt.Sprintf("%s", opt.Type)
	case "ndots", "timeout", "attempts":
		return fmt.Sprintf("%s:%d", opt.Type, opt.Value)
	}
	return ""
}

// New creates a new configuration
func New() *Conf {
	c := new(Conf)
	c.logger = log.New(ioutil.Discard, "[resolvconf] ", 0)
	return c
}

// NewNameserver creates a new Nameserver item
func NewNameserver(IP net.IP) *Nameserver {
	return &Nameserver{IP}
}

// NewDomain creates a new domain that will be used
// as value for the 'domain' option in the generated file
func NewDomain(dom string) *Domain {
	return &Domain{dom}
}

// NewSearchDomain creates a new search domain that will be added
// to the 'search' list in the generated file
func NewSearchDomain(dom string) *SearchDomain {
	return &SearchDomain{dom}
}

// NewSortItem creates a new sortlist that will be added to the
// 'sort' item in the resolv.conf file.
// If mask is given the output will be IP/mask e.g.
// 8.8.8.8/255.255.255.0 otherwise output will be
// IP only, e.g. 8.8.8.8
func NewSortItem(addr net.IP) *SortItem {
	slp := new(SortItem)
	slp.Address = addr
	return slp
}

// NewOption creates a new option, val must be a positive number if used.
// Witout val the option will be interpreted as a bolean e.g.
// debug , with a val the option will be interpreted as an
// setvalue, e.g. ndots:5
func NewOption(t string) *Option {
	// Check va
	opt := new(Option)
	opt.Type = t
	opt.Value = -1

	if _, err := parseOption(opt.String()); err != nil {
		return nil
	}

	return opt
}
