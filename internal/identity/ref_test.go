package identity

import "testing"

func TestIdentityReferenceBoundaries(t *testing.T) {
	good := Reference{SID: "S-1-5-21-1-2-3-1001", CredentialTarget: "WingsWindows/11111111-1111-4111-8111-111111111111"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	if !good.MatchesServer("11111111-1111-4111-8111-111111111111") || good.MatchesServer("22222222-2222-4222-8222-222222222222") {
		t.Fatal("server binding")
	}
	for _, sid := range []string{"S-1-5-18", "S-1-5-21-1-2-3-500", "S-1-5-21-01-2-3-1001", "S-1-5-21-1-2-4294967296-1001", "S-1-5-21-1-2-3-1001-extra"} {
		r := good
		r.SID = sid
		if r.Validate() == nil {
			t.Fatalf("accepted %q", sid)
		}
	}
	for _, target := range []string{"other-app/secret", "WingsWindows/bad", "WingsWindows/AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"} {
		r := good
		r.CredentialTarget = target
		if r.Validate() == nil {
			t.Fatalf("accepted %q", target)
		}
	}
}
