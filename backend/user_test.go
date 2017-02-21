package backend

import (
	"testing"

	"github.com/digitalrebar/digitalrebar/go/common/store"
)

func TestUserStuff(t *testing.T) {
	bs := store.NewSimpleMemoryStore()
	dt := mkDT(bs)
	u := dt.NewUser()
	u.Name = "test user"
	saved, err := dt.create(u)
	if !saved {
		t.Errorf("Unable to create test user: %v", err)
	} else {
		t.Logf("Created test user")
	}
	// should fail because we have no password
	if u.CheckPassword("password") {
		t.Errorf("Checking password should have failed!")
	} else {
		t.Logf("Checking password failed, as expected.")
	}
	if err := u.ChangePassword("password"); err != nil {
		t.Errorf("Changing password failed: %v", err)
	} else {
		t.Logf("Changing password passed.")
	}
	// reload the user, then check the password again.
	newU := dt.NewUser()
	buf, found := dt.FetchOne(newU, "test user")
	newU = AsUser(buf)
	if !found || newU.Name != "test user" {
		t.Errorf("Unable to fetch user from datatracker")
	} else {
		t.Logf("Fetched new user from datatracker cache")
	}
	if !newU.CheckPassword("password") {
		t.Errorf("Checking password should have succeeded.")
	} else {
		t.Logf("CHecking password passed, as expected.")
	}
}
