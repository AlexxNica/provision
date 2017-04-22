package backend

import (
	"io/ioutil"
	"net"
	"path"
	"testing"

	"github.com/digitalrebar/digitalrebar/go/common/store"
	"github.com/pborman/uuid"
)

const (
	tmplIncluded = `Machine: 
Name = {{.Machine.Name}}
HexAddress = {{.Machine.HexAddress}}
ShortName = {{.Machine.ShortName}}
FooParam = {{.Param "foo"}}`

	tmplDefault = `{{template "included" .}}

BootEnv:
Name = {{.Env.Name}}

{{if .ParamExists "fred"}}{{.Param "fred"}}{{end}}

RenderData:
ProvisionerAddress = {{.ProvisionerAddress}}
ProvisionerURL = {{.ProvisionerURL}}
ApiURL = {{.ApiURL}}
CommandURL = {{.CommandURL}}
BootParams = {{.BootParams}}`
	tmplDefaultRenderedWithoutFred = `Machine: 
Name = Test Name
HexAddress = C0A87C0B
ShortName = Test Name
FooParam = bar

BootEnv:
Name = default



RenderData:
ProvisionerAddress = 127.0.0.1
ProvisionerURL = http://127.0.0.1:8091
ApiURL = https://127.0.0.1:8092
CommandURL = CURL
BootParams = default`
	tmplDefaultRenderedWithFred = `Machine: 
Name = Test Name
HexAddress = C0A87C0B
ShortName = Test Name
FooParam = bar

BootEnv:
Name = default

fred = fred

RenderData:
ProvisionerAddress = 127.0.0.1
ProvisionerURL = http://127.0.0.1:8091
ApiURL = https://127.0.0.1:8092
CommandURL = CURL
BootParams = default`
	tmplNothing = `Nothing`
)

func TestRenderData(t *testing.T) {
	bs := store.NewSimpleMemoryStore()
	dt := mkDT(bs)

	defaultBootEnv := &BootEnv{p: dt, Name: "default", Templates: []TemplateInfo{{Name: "ipxe", Path: "machines/{{.Machine.UUID}}/file", ID: "default"}}, BootParams: "{{.Env.Name}}"}
	nothingBootEnv := &BootEnv{p: dt, Name: "nothing", Templates: []TemplateInfo{{Name: "ipxe", Path: "machines/{{.Machine.UUID}}/file", ID: "nothing"}}, BootParams: "{{.Env.Name}}"}
	badBootEnv := &BootEnv{p: dt, Name: "bad", Templates: []TemplateInfo{{Name: "ipxe", Path: "machines/{{.Machine.UUID}}/file", ID: "nothing"}}, BootParams: "{{.Param \"cow\"}}"}

	objs := []crudTest{
		{"Update global profile to have test with a value", dt.update, &Profile{Name: "global", Params: map[string]interface{}{"test": "foreal"}}, true},
		{"create test profile to have test with a value", dt.create, &Profile{Name: "test", Params: map[string]interface{}{"test": "fred"}}, true},

		{"Create included template", dt.create, &Template{p: dt, ID: "included", Contents: tmplIncluded}, true},
		{"Create default template", dt.create, &Template{p: dt, ID: "default", Contents: tmplDefault}, true},
		{"Create nothing template", dt.create, &Template{p: dt, ID: "nothing", Contents: tmplNothing}, true},
		{"Create default bootenv", dt.create, defaultBootEnv, true},
		{"Create nothing bootenv", dt.create, nothingBootEnv, true},
		{"Create bad bootenv", dt.create, badBootEnv, true},
	}
	for _, obj := range objs {
		obj.Test(t)
	}
	machine := dt.NewMachine()
	machine.Uuid = uuid.NewRandom()
	machine.Name = "Test Name"
	machine.Address = net.ParseIP("192.168.124.11")
	machine.BootEnv = "default"
	created, err := dt.create(machine)
	if !created {
		t.Errorf("Failed to create new test machine: %v", err)
		return
	} else {
		t.Logf("Created new test machine")
	}
	pp := machine.GetParams()
	pp["foo"] = "bar"
	machine.SetParams(pp)

	genLoc := path.Join("/", "machines", machine.UUID(), "file")
	out, err := dt.FS.Open(genLoc, nil)
	if err != nil || out == nil {
		t.Errorf("Failed to get tmeplate for %s: %v\n%#v", genLoc, err, out)
		return
	}
	buf, err := ioutil.ReadAll(out)
	if err != nil {
		t.Errorf("Failed to read %s: %v", genLoc, err)
	} else if string(buf) != tmplDefaultRenderedWithoutFred {
		t.Errorf("Failed to render expected template!\nExpected:\n%s\n\nGot:\n%s", tmplDefaultRenderedWithoutFred, string(buf))
	} else {
		t.Logf("BootEnv default without fred rendered properly for test machine")
	}

	pp = machine.GetParams()
	pp["fred"] = "fred = fred"
	machine.SetParams(pp)
	out, err = dt.FS.Open(genLoc, nil)
	if err != nil {
		t.Errorf("Failed to get tmeplate for %s: %v", genLoc, err)
	}
	buf, err = ioutil.ReadAll(out)
	if err != nil {
		t.Errorf("Failed to read %s: %v", genLoc, err)
	} else if string(buf) != tmplDefaultRenderedWithFred {
		t.Errorf("Failed to render expected template!\nExpected:\n%s\n\nGot:\n%s", tmplDefaultRenderedWithFred, string(buf))
	} else {
		t.Logf("BootEnv default with fred rendered properly for test machine")
	}

	machine.BootEnv = "nothing"
	saved, err := dt.save(machine)
	if !saved {
		t.Errorf("Failed to save test machine with new bootenv: %v", err)
	}
	out, err = dt.FS.Open(genLoc, nil)
	if err != nil {
		t.Errorf("Failed to get tmeplate for %s: %v", genLoc, err)
	}
	buf, err = ioutil.ReadAll(out)
	if err != nil {
		t.Errorf("Failed to read %s: %v", genLoc, err)
	} else if string(buf) != tmplNothing {
		t.Errorf("Failed to render expected template!\nExpected:\n%s\n\nGot:\n%s", tmplNothing, string(buf))
	} else {
		t.Logf("BootEnv nothing rendered properly for test machine")
	}

	// Test the render functions directly.
	rd := dt.NewRenderData(nil, nil)

	// Test ParseUrl - independent of Machine and Env
	s, e := rd.ParseUrl("scheme", "http://192.168.0.%31:8080/")
	if e == nil {
		t.Errorf("ParseUrl with bad URL should have generated an error\n")
	} else if e.Error() != "parse http://192.168.0.%31:8080/: invalid URL escape \"%31\"" {
		t.Errorf("ParseUrl with bad URL should have generated an error: %s, but got %s\n", "parse http://192.168.0.%31:8080/: invalid URL escape \"%31\"", e.Error())
	}
	s, e = rd.ParseUrl("bogus", "https://fred/path/apt")
	if e == nil {
		t.Errorf("ParseUrl with bad segment should have generated an error\n")
	} else if e.Error() != "No idea how to get URL part bogus from https://fred/path/apt" {
		t.Errorf("ParseUrl with bad segment should have generated an error: %s, but got %s\n", "No idea how to get URL part bogus from https://fred/path/apt", e.Error())
	}
	s, e = rd.ParseUrl("scheme", "https://fred/path/apt")
	if e != nil {
		t.Errorf("ParseUrl with scheme segment should NOT have generated an error: %v\n", e)
	}
	if s != "https" {
		t.Errorf("ParseUrl with scheme segment found incorrect scheme: %s, %s\n", "https", s)
	}
	s, e = rd.ParseUrl("host", "https://fred/path/apt")
	if e != nil {
		t.Errorf("ParseUrl with host segment should NOT have generated an error: %v\n", e)
	}
	if s != "fred" {
		t.Errorf("ParseUrl with host segment found incorrect host: %s, %s\n", "fred", s)
	}
	s, e = rd.ParseUrl("path", "https://fred/path/apt")
	if e != nil {
		t.Errorf("ParseUrl with path segment should NOT have generated an error: %v\n", e)
	}
	if s != "/path/apt" {
		t.Errorf("ParseUrl with path segment found incorrect path: %s, %s\n", "/path/apt", s)
	}

	// Test other functions - without a machine or env
	_, e = rd.Param("bogus")
	if e == nil {
		t.Errorf("Param should return an error when machine is not and not global defined in RenderData\n")
	} else if e.Error() != "No such machine parameter bogus" {
		t.Errorf("Param should return an error: No such machine parameter bogus, but returned: %s\n", e.Error())
	}
	ok := rd.ParamExists("bogus")
	if ok {
		t.Errorf("ParamExists should return false when machine is not defined and not global in RenderData\n")
	}
	// Test global parameter
	d, e := rd.Param("test")
	if e != nil {
		t.Errorf("Param test should NOT return an error: %v\n", e)
	}
	s, ok = d.(string)
	if !ok {
		t.Errorf("Parameter test should have been a string\n")
	} else {
		if s != "foreal" {
			t.Errorf("Parameter test should have been foreal: %s\n", s)
		}
	}
	ok = rd.ParamExists("test")
	if !ok {
		t.Errorf("ParamExists test should return true when machine has foo defined in RenderData\n")
	}

	s, e = rd.BootParams()
	if e == nil {
		t.Errorf("BootParams with no ENV should have generated an error\n")
	} else if e.Error() != "Missing bootenv" {
		t.Errorf("BootParams with no ENV should have generated an error: %s, but got %s\n", "Missing bootenv", e.Error())
	}
	s = rd.GenerateToken()
	claim, e := dt.GetToken(s)
	if e != nil {
		t.Errorf("GetToken should return a good claim. %v\n", e)
	}
	if !claim.Match("machines", "post", "anything") {
		t.Errorf("Unknown token should match: machines/post/*\n")
	}
	if !claim.Match("machines", "get", "anything") {
		t.Errorf("Unknown token should match: machines/post/*\n")
	}
	if claim.ExpiresAt-claim.IssuedAt != 600 {
		t.Errorf("Unknown token timeout should be 600, but was %v\n", claim.ExpiresAt-claim.IssuedAt)
	}
	e = dt.SetPrefs(map[string]string{"unknownTokenTimeout": "50"})
	if e != nil {
		t.Errorf("SetPrefs should not return an error: %v\n", e)
	}
	s = rd.GenerateToken()
	claim, e = dt.GetToken(s)
	if e != nil {
		t.Errorf("GetToken should return a good claim. %v\n", e)
	}
	if !claim.Match("machines", "post", "anything") {
		t.Errorf("Unknown token should match: machines/post/*\n")
	}
	if !claim.Match("machines", "get", "anything") {
		t.Errorf("Unknown token should match: machines/post/*\n")
	}
	if claim.Match("machines", "patch", "anything") {
		t.Errorf("Unknown token should NOT match: machines/patch/*\n")
	}
	if claim.ExpiresAt-claim.IssuedAt != 50 {
		t.Errorf("Unknown token timeout should be 50, but was %v\n", claim.ExpiresAt-claim.IssuedAt)
	}

	// Tests with machine and bootenv (has bad BootParams)
	rd = dt.NewRenderData(machine, badBootEnv)
	_, e = rd.Param("bogus")
	if e == nil {
		t.Errorf("Param should return an error when machine is not defined in RenderData\n")
	} else if e.Error() != "No such machine parameter bogus" {
		t.Errorf("Param should return an error: No such machine parameter bogus, but returned: %s\n", e.Error())
	}
	ok = rd.ParamExists("bogus")
	if ok {
		t.Errorf("ParamExists should return false when machine is not defined in RenderData\n")
	}

	// Test machine parameter
	d, e = rd.Param("foo")
	if e != nil {
		t.Errorf("Param foo should NOT return an error: %v\n", e)
	}
	s, ok = d.(string)
	if !ok {
		t.Errorf("Parameter foo should have been a string\n")
	} else {
		if s != "bar" {
			t.Errorf("Parameter foo should have been bar: %s\n", s)
		}
	}
	ok = rd.ParamExists("foo")
	if !ok {
		t.Errorf("ParamExists foo should return true when machine has foo defined in RenderData\n")
	}

	// Test global parameter
	d, e = rd.Param("test")
	if e != nil {
		t.Errorf("Param test should NOT return an error: %v\n", e)
	}
	s, ok = d.(string)
	if !ok {
		t.Errorf("Parameter test should have been a string\n")
	} else {
		if s != "foreal" {
			t.Errorf("Parameter test should have been foreal: %s\n", s)
		}
	}
	ok = rd.ParamExists("test")
	if !ok {
		t.Errorf("ParamExists test should return true when machine has foo defined in RenderData\n")
	}

	// Test a machine profile parameter
	machine.Profiles = []string{"test"}
	saved, err = dt.save(machine)
	if !saved {
		t.Errorf("Failed to save test machine with new profile list: %v", err)
	}
	d, e = rd.Param("test")
	if e != nil {
		t.Errorf("Param test should NOT return an error: %v\n", e)
	}
	s, ok = d.(string)
	if !ok {
		t.Errorf("Parameter test should have been a string\n")
	} else {
		if s != "fred" {
			t.Errorf("Parameter test should have been fred: %s\n", s)
		}
	}
	ok = rd.ParamExists("test")
	if !ok {
		t.Errorf("ParamExists test should return true when machine profile has test defined in RenderData\n")
	}

	s, e = rd.BootParams()
	errString := "template: machine:1:2: executing \"machine\" at <.Param>: error calling Param: No such machine parameter cow"
	if e == nil {
		t.Errorf("BootParams with no ENV should have generated an error\n")
	} else if e.Error() != errString {
		t.Errorf("BootParams with no ENV should have generated an error: %s, but got %s\n", errString, e.Error())
	}
	s = rd.GenerateToken()
	claim, e = dt.GetToken(s)
	if e != nil {
		t.Errorf("GetToken should return a good claim. %v\n", e)
	}
	if claim.Match("machines", "post", "anything") {
		t.Errorf("Known token should NOT match: machines/post/*\n")
	}
	if claim.Match("machines", "get", "anything") {
		t.Errorf("Known token should NOT match: machines/get/*\n")
	}
	if claim.Match("machines", "patch", "anything") {
		t.Errorf("Known token should NOT match: machines/patch/*\n")
	}
	if !claim.Match("machines", "get", machine.Key()) {
		t.Errorf("Known token should match: machines/get/%s\n", machine.Key())
	}
	if !claim.Match("machines", "patch", machine.Key()) {
		t.Errorf("Known token should match: machines/patch/%s\n", machine.Key())
	}
	if claim.ExpiresAt-claim.IssuedAt != 3600 {
		t.Errorf("Known token timeout should be 3600, but was %v\n", claim.ExpiresAt-claim.IssuedAt)
	}
	e = dt.SetPrefs(map[string]string{"knownTokenTimeout": "50"})
	if e != nil {
		t.Errorf("SetPrefs should not return an error: %v\n", e)
	}
	s = rd.GenerateToken()
	claim, e = dt.GetToken(s)
	if e != nil {
		t.Errorf("GetToken should return a good claim. %v\n", e)
	}
	if claim.Match("machines", "post", "anything") {
		t.Errorf("Known token should NOT match: machines/post/*\n")
	}
	if claim.Match("machines", "get", "anything") {
		t.Errorf("Known token should NOT match: machines/get/*\n")
	}
	if claim.Match("machines", "patch", "anything") {
		t.Errorf("Known token should NOT match: machines/patch/*\n")
	}
	if !claim.Match("machines", "get", machine.Key()) {
		t.Errorf("Known token should match: machines/get/%s\n", machine.Key())
	}
	if !claim.Match("machines", "patch", machine.Key()) {
		t.Errorf("Known token should match: machines/patch/%s\n", machine.Key())
	}
	if claim.ExpiresAt-claim.IssuedAt != 50 {
		t.Errorf("Known token timeout should be 50, but was %v\n", claim.ExpiresAt-claim.IssuedAt)
	}

}
