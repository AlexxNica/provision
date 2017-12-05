package backend

import (
	"github.com/digitalrebar/provision/backend/index"
	"github.com/digitalrebar/provision/models"
	"github.com/digitalrebar/store"
)

// Profile represents a set of key/values to use in
// template expansion.
//
// There is one special profile named 'global' that acts
// as a global set of parameters for the system.
//
// These can be assigned to a machine's profile list.
// swagger:model
type Profile struct {
	*models.Profile
	validate
	p *DataTracker
}

func (obj *Profile) SetReadOnly(b bool) {
	obj.ReadOnly = b
}

func (obj *Profile) SaveClean() store.KeySaver {
	mod := *obj.Profile
	mod.ClearValidation()
	return toBackend(obj.p, nil, &mod)
}

func (p *Profile) Indexes() map[string]index.Maker {
	fix := AsProfile
	res := index.MakeBaseIndexes(p)
	res["Name"] = index.Make(
		true,
		"string",
		func(i, j models.Model) bool { return fix(i).Name < fix(j).Name },
		func(ref models.Model) (gte, gt index.Test) {
			refName := fix(ref).Name
			return func(s models.Model) bool {
					return fix(s).Name >= refName
				},
				func(s models.Model) bool {
					return fix(s).Name > refName
				}
		},
		func(s string) (models.Model, error) {
			profile := fix(p.New())
			profile.Name = s
			return profile, nil
		})
	return res
}

func (p *Profile) Backend() store.Store {
	return p.p.getBackend(p)
}

func (p *Profile) GetParams(d Stores, aggregate bool) map[string]interface{} {
	m := p.Params
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func (p *Profile) SetParams(d Stores, values map[string]interface{}) error {
	p.Params = values
	e := &models.Error{Code: 422, Type: ValidationError, Model: p.Prefix(), Key: p.Key()}
	_, e2 := p.p.Save(d, p)
	e.AddError(e2)
	return e.HasError()
}

func (p *Profile) GetParam(d Stores, key string, aggregate bool) (interface{}, bool) {
	mm := p.GetParams(d, aggregate)
	if v, found := mm[key]; found {
		return v, true
	}
	if p := d("params").Find(key); p != nil && aggregate {
		param := p.(*Param)
		return param.DefaultValue()
	}
	return nil, false
}

func (p *Profile) SetParam(d Stores, key string, val interface{}) error {
	p.Params[key] = val
	e := &models.Error{Code: 422, Type: ValidationError, Model: p.Prefix(), Key: p.Key()}
	_, e2 := p.p.Save(d, p)
	e.AddError(e2)
	return e.HasError()
}

func (p *Profile) New() store.KeySaver {
	res := &Profile{Profile: &models.Profile{}}
	if p.Profile != nil && p.ChangeForced() {
		res.ForceChange()
	}
	res.Params = map[string]interface{}{}
	res.p = p.p
	return res
}

func (p *Profile) setDT(dp *DataTracker) {
	p.p = dp
}

func (p *Profile) BeforeDelete() error {
	e := &models.Error{Code: 422, Type: ValidationError, Model: p.Prefix(), Key: p.Key()}
	if p.Name == p.p.GlobalProfileName {
		e.Errorf("Profile %s is the global profile, you cannot delete it", p.Name)
	}
	machines := p.stores("machines")
	for _, i := range machines.Items() {
		m := AsMachine(i)
		if m.HasProfile(p.Name) {
			e.Errorf("Machine %s is using profile %s", m.UUID(), p.Name)
		}
	}
	stages := p.stores("stages")
	for _, i := range stages.Items() {
		s := AsStage(i)
		if s.HasProfile(p.Name) {
			e.Errorf("Stage %s is using profile %s", s.Name, p.Name)
		}
	}
	return e.HasError()
}

func AsProfile(o models.Model) *Profile {
	return o.(*Profile)
}

func AsProfiles(o []models.Model) []*Profile {
	res := make([]*Profile, len(o))
	for i := range o {
		res[i] = AsProfile(o[i])
	}
	return res
}
func (p *Profile) Validate() {
	p.Profile.Validate()
	p.AddError(index.CheckUnique(p, p.stores("profiles").Items()))
	p.SetValid()
	params := p.stores("params")
	for k, v := range p.Params {
		if pIdx := params.Find(k); pIdx != nil {
			param := AsParam(pIdx)
			if err := param.ValidateValue(v); err != nil {
				p.Errorf("Key '%s': invalid val '%s': %v", k, v, err)
			}
		}
	}
	p.SetAvailable()
}

func (p *Profile) BeforeSave() error {
	p.Validate()
	if !p.Useable() {
		return p.MakeError(422, ValidationError, p)
	}
	return nil
}

func (p *Profile) OnLoad() error {
	if p.Params == nil {
		p.Params = map[string]interface{}{}
	}
	p.stores = func(ref string) *Store {
		return p.p.objs[ref]
	}
	defer func() { p.stores = nil }()
	return p.BeforeSave()
}

var profileLockMap = map[string][]string{
	"get":    []string{"profiles", "params"},
	"create": []string{"profiles", "tasks", "params"},
	"update": []string{"profiles", "tasks", "params"},
	"patch":  []string{"profiles", "tasks", "params"},
	"delete": []string{"stages", "profiles", "machines"},
}

func (p *Profile) Locks(action string) []string {
	return profileLockMap[action]
}
