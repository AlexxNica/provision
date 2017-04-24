package backend

import (
	"testing"

	"github.com/digitalrebar/digitalrebar/go/common/store"
)

func TestProfilesCrud(t *testing.T) {
	bs := store.NewSimpleMemoryStore()
	dt := mkDT(bs)
	tests := []crudTest{
		{"Create empty profile", dt.create, &Profile{p: dt}, false},
		{"Create new profile with name", dt.create, &Profile{p: dt, Name: "Test Profile"}, true},
		{"Create Duplicate Profile", dt.create, &Profile{p: dt, Name: "Test Profile"}, false},
		{"Delete Profile", dt.remove, &Profile{p: dt, Name: "Test Profile"}, true},
		{"Delete Nonexistent Profile", dt.remove, &Profile{p: dt, Name: "Test Profile"}, false},
	}
	for _, test := range tests {
		test.Test(t)
	}
}
