package config

import "testing"

func TestEmptyProfilesRequirePendingCreationOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		c := valid(t)
		c.Profiles = map[string]Profile{}
		c.AllowPendingCreation = enabled
		err := c.Validate()
		if (err == nil) != enabled {
			t.Fatal(enabled, err)
		}
	}
}

func TestBootstrapExampleHasNoExecutionApproval(t *testing.T) {
	c, err := Load("../../configs/config.bootstrap.example.json")
	if err != nil {
		t.Fatal(err)
	}
	if !c.AllowPendingCreation || len(c.Profiles) != 0 {
		t.Fatal("bootstrap example grants executable approval")
	}
}
