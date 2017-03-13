package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/digitalrebar/digitalrebar/go/common/store"
	"github.com/digitalrebar/digitalrebar/go/rebar-api/api"
)

type dtobjs struct {
	sync.Mutex
	d []store.KeySaver
}

func (dt *dtobjs) sort() {
	sort.Slice(dt.d, func(i, j int) bool { return dt.d[i].Key() < dt.d[j].Key() })
}

func (dt *dtobjs) find(key string) (int, bool) {
	idx := sort.Search(len(dt.d), func(i int) bool { return dt.d[i].Key() >= key })
	return idx, idx < len(dt.d) && dt.d[idx].Key() == key
}

func (dt *dtobjs) subset(lower, upper func(string) bool) []store.KeySaver {
	i := sort.Search(len(dt.d), func(i int) bool { return lower(dt.d[i].Key()) })
	j := sort.Search(len(dt.d), func(i int) bool { return upper(dt.d[i].Key()) })
	if i == len(dt.d) {
		return []store.KeySaver{}
	}
	res := make([]store.KeySaver, j-i)
	copy(res, dt.d[i:])
	return res
}

func (dt *dtobjs) add(obj store.KeySaver) {
	// This could be smarter and avoid sorting, but I really don't care
	// right now.
	dt.d = append(dt.d, obj)
	dt.sort()
}

type dtSetter interface {
	store.KeySaver
	setDT(*DataTracker)
}

func (dt *dtobjs) remove(idx int) {
	// This could also try harder to avoid copies, but I am not worrying for now.
	neu := make([]store.KeySaver, 0, len(dt.d)-1)
	for i := range dt.d {
		if i != idx {
			neu = append(neu, dt.d[i])
		}
	}
	dt.d = neu
}

// DataTracker represents everything there is to know about acting as a dataTracker.
type DataTracker struct {
	useProvisioner bool
	useDHCP        bool
	FileRoot       string
	CommandURL     string
	DefaultBootEnv string
	UnknownBootEnv string
	OurAddress     string

	FileURL string
	ApiURL  string

	Logger *log.Logger

	RebarClient *api.Client
	// Note the lack of mutexes for these maps.
	// We shouls be able to get away with not locking them
	// by only ever writing to them at DataTracker create time,
	// and only ever reading from them afterwards.
	backends map[string]store.SimpleStore
	objs     map[string]*dtobjs
}

func (p *DataTracker) lockFor(prefix string) *dtobjs {
	res := p.objs[prefix]
	res.Lock()
	return res
}

func (p *DataTracker) makeBackends(backend store.SimpleStore, objs []store.KeySaver) {
	for _, o := range objs {
		prefix := o.Prefix()
		bk, err := backend.Sub(prefix)
		if err != nil {
			p.Logger.Fatalf("dataTracker: Error creating substore %s: %v", prefix, err)
		}
		p.backends[prefix] = bk
	}
}

func (p *DataTracker) loadData(refObjs []store.KeySaver) error {
	p.objs = map[string]*dtobjs{}
	for _, refObj := range refObjs {
		prefix := refObj.Prefix()
		objs, err := store.List(refObj)
		if err != nil {
			p.Logger.Fatalf("dataTracker: Error loading data for %s: %v", prefix, err)
		}
		p.objs[prefix] = &dtobjs{d: objs}
		p.objs[prefix].sort()
	}
	return nil
}

// Create a new DataTracker that will use passed store to save all operational data
func NewDataTracker(backend store.SimpleStore,
	useProvisioner, useDHCP bool,
	fileRoot, commandURL, dbe, ube, furl, aurl, addr string,
	logger *log.Logger) *DataTracker {

	res := &DataTracker{
		useDHCP:        useDHCP,
		useProvisioner: useProvisioner,
		FileRoot:       fileRoot,
		CommandURL:     commandURL,
		DefaultBootEnv: dbe,
		UnknownBootEnv: ube,
		FileURL:        furl,
		ApiURL:         aurl,
		OurAddress:     addr,
		Logger:         logger,

		backends: map[string]store.SimpleStore{},
	}
	objs := []store.KeySaver{&Machine{p: res}, &User{p: res}}

	if useProvisioner {
		objs = append(objs, &Template{p: res}, &BootEnv{p: res})
	}
	if useDHCP {
		objs = append(objs, &Subnet{p: res}, &Reservation{p: res}, &Lease{p: res})
	}
	res.makeBackends(backend, objs)
	res.loadData(objs)
	return res
}

func (p *DataTracker) getBackend(t store.KeySaver) store.SimpleStore {
	res, ok := p.backends[t.Prefix()]
	if !ok {
		p.Logger.Fatalf("%s: No registered storage backend!", t.Prefix())
	}
	return res
}

func (p *DataTracker) setDT(s store.KeySaver) {
	if tgt, ok := s.(dtSetter); ok {
		tgt.setDT(p)
	}
}

func (p *DataTracker) Clone(ref store.KeySaver) store.KeySaver {
	var res store.KeySaver
	switch ref.(type) {
	case *Machine:
		res = p.NewMachine()
	case *User:
		res = p.NewUser()
	case *Template:
		res = p.NewTemplate()
	case *BootEnv:
		res = p.NewBootEnv()
	case *Lease:
		res = p.NewLease()
	case *Reservation:
		res = p.NewReservation()
	case *Subnet:
		res = p.NewSubnet()
	default:
		panic("Unknown type of KeySaver passed to Clone")
	}
	buf, err := json.Marshal(ref)
	if err != nil {
		panic(err.Error())
	}
	if err := json.Unmarshal(buf, &res); err != nil {
		panic(err.Error())
	}
	return res
}

// unlockedFetchAll gets all the objects with a given prefix without
// taking any locks.  It should only be called in hooks that need to
// check for object uniqueness based on something besides a Key()
func (p *DataTracker) unlockedFetchAll(prefix string) []store.KeySaver {
	return p.objs[prefix].d
}

// fetchSome returns all of the objects in one of our caches in
// between the point where lower starts to match its Search and upper
// starts to match its Search.  The lower and upper parameters must be
// functions that accept a Key() and return a yes or no decision about
// whether that particular entry is in range.
func (p *DataTracker) fetchSome(prefix string, lower, upper func(string) bool) []store.KeySaver {
	mux := p.lockFor(prefix)
	defer mux.Unlock()
	return mux.subset(lower, upper)
}

// fetchAll returns all the instances we know about, It differs from FetchAll in that
// it does not make a copy of the thing.
func (p *DataTracker) fetchAll(ref store.KeySaver) []store.KeySaver {
	prefix := ref.Prefix()
	mux := p.lockFor(prefix)
	defer mux.Unlock()
	res := make([]store.KeySaver, len(mux.d))
	copy(res, mux.d)
	return res
}

// FetchAll returns all of the cached objects of a given type.  It
// should be used instead of store.List.
func (p *DataTracker) FetchAll(ref store.KeySaver) []store.KeySaver {
	prefix := ref.Prefix()
	res := p.lockFor(prefix)
	ret := make([]store.KeySaver, len(res.d))
	for i := range res.d {
		ret[i] = p.Clone(res.d[i])
	}
	res.Unlock()
	return ret
}

func (p *DataTracker) lockedGet(prefix, key string) (*dtobjs, int, bool) {
	mux := p.lockFor(prefix)
	idx, found := mux.find(key)
	return mux, idx, found
}

func (p *DataTracker) fetchOne(ref store.KeySaver, key string) (store.KeySaver, bool) {
	prefix := ref.Prefix()
	mux, idx, found := p.lockedGet(prefix, key)
	defer mux.Unlock()
	if found {
		return mux.d[idx], found
	}
	return nil, found
}

// FetchOne returns a specific instance from the cached objects of
// that type.  It should be used instead of store.Load.
func (p *DataTracker) FetchOne(ref store.KeySaver, key string) (store.KeySaver, bool) {
	res, found := p.fetchOne(ref, key)
	if !found {
		return nil, found
	}
	return p.Clone(res), found
}

func (p *DataTracker) create(ref store.KeySaver) (bool, error) {
	prefix := ref.Prefix()
	key := ref.Key()
	if key == "" {
		return false, fmt.Errorf("dataTracker create %s: Empty key not allowed", prefix)
	}
	mux, _, found := p.lockedGet(prefix, key)
	defer mux.Unlock()
	if found {
		return false, fmt.Errorf("dataTracker create %s: %s already exists", prefix, key)
	}
	p.setDT(ref)
	saved, err := store.Create(ref)
	if saved {
		mux.add(ref)
	}
	return saved, err
}

// Create creates a new thing, caching it locally iff the create
// succeeds.  It should be used instead of store.Create
func (p *DataTracker) Create(ref store.KeySaver) (store.KeySaver, error) {
	created, err := p.create(ref)
	if created {
		return p.Clone(ref), err
	}
	return ref, err
}

func (p *DataTracker) remove(ref store.KeySaver) (bool, error) {
	prefix := ref.Prefix()
	key := ref.Key()
	mux, idx, found := p.lockedGet(prefix, key)
	defer mux.Unlock()
	if !found {
		return false, fmt.Errorf("dataTracker remove %s: %s does not exist", prefix, key)
	}
	removed, err := store.Remove(mux.d[idx])
	if removed {
		mux.remove(idx)
	}
	return removed, err
}

// Remove removes the thing from the backing store.  If the remove
// succeeds, it will also be removed from the local cache.  It should
// be used instead of store.Remove
func (p *DataTracker) Remove(ref store.KeySaver) (store.KeySaver, error) {
	removed, err := p.remove(ref)
	if !removed {
		return p.Clone(ref), err
	}
	return ref, err
}

func (p *DataTracker) update(ref store.KeySaver) (bool, error) {
	prefix := ref.Prefix()
	key := ref.Key()
	mux, idx, found := p.lockedGet(prefix, key)
	defer mux.Unlock()
	if !found {
		return false, fmt.Errorf("dataTracker remove %s: %s does not exist", prefix, key)
	}
	p.setDT(ref)
	ok, err := store.Update(ref)
	if ok {
		mux.d[idx] = ref
	}
	return ok, err
}

// Update updates the passed thing, and updates the local cache iff
// the update succeeds.  It should be used in preference to
// store.Update
func (p *DataTracker) Update(ref store.KeySaver) (store.KeySaver, error) {
	updated, err := p.update(ref)
	if updated {
		return p.Clone(ref), err
	}
	return ref, err
}

func (p *DataTracker) save(ref store.KeySaver) (bool, error) {
	prefix := ref.Prefix()
	key := ref.Key()
	mux, idx, found := p.lockedGet(prefix, key)
	defer mux.Unlock()
	p.setDT(ref)
	ok, err := store.Save(ref)
	if !ok {
		return ok, err
	}
	if found {
		mux.d[idx] = ref
	} else {
		mux.add(ref)
	}
	return ok, err
}

// Save saves the passed thing, updating the local cache iff the save
// succeeds.  It should be used instead of store.Save
func (p *DataTracker) Save(ref store.KeySaver) (store.KeySaver, error) {
	saved, err := p.save(ref)
	if saved {
		return p.Clone(ref), err
	}
	return ref, err
}
