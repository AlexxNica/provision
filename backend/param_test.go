package backend

import (
	"testing"

	"github.com/digitalrebar/digitalrebar/go/common/store"
)

func TestParamsCrud(t *testing.T) {
	bs := store.NewSimpleMemoryStore()
	dt := mkDT(bs)
	tests := []crudTest{
		{"Create empty profile", dt.create, &Param{p: dt}, false},
		{"Create new profile with name", dt.create, &Param{p: dt, Name: "Test Param"}, false},
		{"Create new profile with name and schema", dt.create, &Param{p: dt,
			Name:   "Test Param",
			Schema: map[string]interface{}{},
		}, true},
		{"Create new profile with name and schema", dt.create, &Param{p: dt,
			Name: "Test Param 2",
			Schema: map[string]interface{}{
				"type": "boolean",
			},
		}, true},
		{"Create Duplicate Param", dt.create, &Param{p: dt, Name: "Test Param"}, false},
		{"Delete Param", dt.remove, &Param{p: dt, Name: "Test Param"}, true},
		{"Delete Nonexistent Param", dt.remove, &Param{p: dt, Name: "Test Param"}, false},
	}
	for _, test := range tests {
		test.Test(t)
	}
	// List test.
	b := dt.NewParam()
	bes := b.List()
	if bes != nil {
		if len(bes) != 1 {
			t.Errorf("List function should have returned: 1, but got %d\n", len(bes))
		}
	} else {
		t.Errorf("List function returned nil!!")
	}
}
