package backend

import (
	"testing"

	"github.com/digitalrebar/provision/models"
)

func TestUserCrud(t *testing.T) {
	dt := mkDT(nil)
	d, unlocker := dt.LockEnts("users")
	defer unlocker()
	tests := []crudTest{
		{"Create empty user", dt.Create, &models.User{}, false},
		{"Create with bad user /", dt.Create, &models.User{Name: "greg/asdg"}, false},
		{"Create with bad user \\", dt.Create, &models.User{Name: "greg\\agsd"}, false},
		{"Create new user with name", dt.Create, &models.User{Name: "Test User"}, true},
		{"Create Duplicate User", dt.Create, &models.User{Name: "Test User"}, false},
		{"Delete User", dt.Remove, &models.User{Name: "Test User"}, true},
		{"Delete Nonexistent User", dt.Remove, &models.User{Name: "Test User"}, false},
	}
	for _, test := range tests {
		test.Test(t, d)
	}
	// List test.
	bes := d("users").Items()
	if bes != nil {
		if len(bes) != 1 {
			t.Errorf("List function should have returned: 1, but got %d\n", len(bes))
		}
	} else {
		t.Errorf("List function returned nil!!")
	}
}

func TestUserPassword(t *testing.T) {
	dt := mkDT(nil)
	d, unlocker := dt.LockEnts("users")
	defer unlocker()
	u := &User{}
	Fill(u)
	u.Name = "test user"
	saved, err := dt.Create(d, u)
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
	if err := u.ChangePassword(d, "password"); err != nil {
		t.Errorf("Changing password failed: %v", err)
	} else {
		t.Logf("Changing password passed.")
	}
	// reload the user, then check the password again.
	buf := d("users").Find("test user")
	if buf == nil {
		t.Errorf("Unable to fetch user from datatracker")
	} else {
		t.Logf("Fetched new user from datatracker cache")
	}
	newU := AsUser(buf)
	if !newU.CheckPassword("password") {
		t.Errorf("Checking password should have succeeded.")
	} else {
		t.Logf("CHecking password passed, as expected.")
	}
	// Make sure sanitizing the user works as expected
	sanitizedU := newU.Sanitize().(*models.User)
	if len(sanitizedU.PasswordHash) != 0 {
		t.Errorf("Sanitize did not strip out the password hash")
	} else {
		t.Logf("Sanitize stripped out the password hash")
	}
}
