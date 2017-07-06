package backend

import (
	"testing"

	"github.com/digitalrebar/digitalrebar/go/common/store"
)

func TestTaskCrud(t *testing.T) {
	bs := store.NewSimpleMemoryStore()
	dt := mkDT(bs)
	tmpl := &Template{p: dt, ID: "ok", Contents: "{{ .Env.Name }}"}
	if ok, err := dt.create(tmpl); !ok {
		t.Errorf("Failed to create test OK template: %v", err)
		return
	}
	tests := []crudTest{
		{"Create Task with nonexistent Name", dt.create, &Task{p: dt}, false},
		{"Create Task with no templates", dt.create, &Task{p: dt, Name: "test 1"}, true},
		{"Create Task with invalid TemplateInfo (missing Name)", dt.create, &Task{p: dt, Name: "test 3", Templates: []TemplateInfo{{Path: "{{ .Env.Name }}", ID: "ok"}}}, false},
		{"Create Task with invalid TemplateInfo (missing ID)", dt.create, &Task{p: dt, Name: "test 3", Templates: []TemplateInfo{{Name: "test 3", Path: "{{ .Env.Name }}"}}}, false},
		{"Create Task with invalid TemplateInfo (invalid ID)", dt.create, &Task{p: dt, Name: "test 3", Templates: []TemplateInfo{{Name: "test 3", Path: "{{ .Env.Name }}", ID: "okp"}}}, false},
		{"Create Task with invalid TemplateInfo (invalid Path)", dt.create, &Task{p: dt, Name: "test 3", Templates: []TemplateInfo{{Name: "test 3", Path: "{{ .Env.Name }", ID: "ok"}}}, false},
		{"Create Task with valid TemplateInfo (not available}", dt.create, &Task{p: dt, Name: "test 3", Templates: []TemplateInfo{{Name: "unavailable", Path: "{{ .Env.Name }}", ID: "ok"}}}, true},
		{"Create Task with valid TemplateInfo (available)", dt.create, &Task{p: dt, Name: "available", Templates: []TemplateInfo{{Name: "ipxe", Path: "{{ .Env.Name }}", ID: "ok"}}}, true},
	}

	for _, test := range tests {
		test.Test(t)
	}

	// List test.
	b := dt.NewTask()
	bes := b.List()
	if bes != nil {
		if len(bes) != 3 {
			t.Errorf("List function should have returned: 5, but got %d\n", len(bes))
		}
	} else {
		t.Errorf("List function returned nil!!")
	}
	/*
		// We need a Machine that refers to one of our Tasks to
		// test proper delete restrictions
		machine := &Machine{p: dt, Name: "test 1", Task: "available", Uuid: uuid.NewRandom()}
		if ok, err := dt.create(machine); !ok {
			t.Errorf("Failed to create test machine: %v", err)
			return
		}
		rmTests := []crudTest{
			{"Remove Task that is not in use", dt.remove, &Task{p: dt, Name: "test 1"}, true},
			{"Remove nonexistent Task", dt.remove, &Task{p: dt, Name: "test 1"}, false},
			{"Remove Task that is in use", dt.remove, &Task{p: dt, Name: "available"}, false},
		}
		for _, test := range rmTests {
			test.Test(t)
		}
	*/
}
