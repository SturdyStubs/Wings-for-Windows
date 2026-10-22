package consolelog

import "testing"

func TestInstallReceiptRejectsFalseOutcomesAndAttemptChanges(t *testing.T) {
	running := InstallRecord{Phase: "running", AttemptID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}
	result := running
	result.Phase = "result"
	result.Successful = true
	for _, mode := range []string{"missing-attempt", "unknown-phase", "running-success", "idle-result", "changed-attempt", "changed-reinstall", "changed-success", "reuse-attempt"} {
		t.Run(mode, func(t *testing.T) {
			old, next := running, result
			switch mode {
			case "missing-attempt":
				next.AttemptID = ""
			case "unknown-phase":
				next.Phase = "invented"
			case "running-success":
				next.Phase = "running"
			case "idle-result":
				old = InstallRecord{Phase: "idle"}
			case "changed-attempt":
				next.AttemptID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
			case "changed-reinstall":
				next.Reinstall = true
			case "changed-success":
				old = result
				next.Phase = "acknowledged"
				next.Successful = false
			case "reuse-attempt":
				old = result
				old.Phase = "acknowledged"
				next = running
			}
			if installTransition(old, next) == nil {
				t.Fatal("invalid receipt transition accepted", mode)
			}
		})
	}
}
