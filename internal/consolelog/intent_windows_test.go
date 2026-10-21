package consolelog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIntentDurableTransitionsAndExclusiveWriter(t *testing.T) {
	options := budgetOptions(t)
	console, _, err := Open(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close(context.Background())
	budget, _, err := OpenRecovery(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close(context.Background())
	intent, running, err := OpenIntent(options, budgetTestID)
	if err != nil || running {
		t.Fatal(running, err)
	}
	if other, _, err := OpenIntent(options, budgetTestID); err == nil {
		other.Close(context.Background())
		t.Fatal("second intent writer accepted")
	}
	for _, want := range []bool{true, false, true, false} {
		if err := intent.Save(want); err != nil {
			t.Fatal(err)
		}
		if err := intent.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		intent, running, err = OpenIntent(options, budgetTestID)
		if err != nil || running != want {
			t.Fatal(running, want, err)
		}
	}
	intent.Close(context.Background())
	if intent.Save(true) == nil {
		t.Fatal("closed writer accepted")
	}
}
func TestIntentCorruptionNeverFallsBackToOlderRunningState(t *testing.T) {
	for _, mode := range []string{"corrupt", "missing", "checksum", "wrong-server", "sequence"} {
		t.Run(mode, func(t *testing.T) {
			options := budgetOptions(t)
			console, _, err := Open(options, budgetTestID)
			if err != nil {
				t.Fatal(err)
			}
			defer console.Close(context.Background())
			intent, _, err := OpenIntent(options, budgetTestID)
			if err != nil {
				t.Fatal(err)
			}
			intent.Save(true)
			intent.Save(false)
			intent.Close(context.Background())
			path := filepath.Join(options.Directory, budgetTestID, "intent.0.json")
			switch mode {
			case "missing":
				os.Remove(path)
			case "corrupt":
				os.WriteFile(path, []byte("TORN_STOP"), 0600)
			default:
				payload := intentPayload{Version: 1, Server: budgetTestID, Sequence: 2, Running: false}
				if mode == "wrong-server" {
					payload.Server = "22222222-2222-4222-8222-222222222222"
				}
				if mode == "sequence" {
					payload.Sequence = 6
				}
				raw, _ := intentBytes(payload)
				if mode == "checksum" {
					raw[len(raw)-4] ^= 1
				}
				os.WriteFile(path, raw, 0600)
			}
			if reopened, _, err := OpenIntent(options, budgetTestID); err == nil {
				reopened.Close(context.Background())
				t.Fatal("bad stop intent rolled back")
			}
		})
	}
}
func TestIntentRejectsHardlinksAndConflictingMutation(t *testing.T) {
	options := budgetOptions(t)
	console, _, err := Open(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	defer console.Close(context.Background())
	intent, _, err := OpenIntent(options, budgetTestID)
	if err != nil {
		t.Fatal(err)
	}
	intent.Save(true)
	path := filepath.Join(options.Directory, budgetTestID, "intent.1.json")
	if file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0600); err == nil {
		file.Close()
		t.Fatal("writer changed guarded state")
	}
	intent.Close(context.Background())
	if err := os.Link(path, path+".link"); err != nil {
		t.Fatal(err)
	}
	if reopened, _, err := OpenIntent(options, budgetTestID); err == nil {
		reopened.Close(context.Background())
		t.Fatal("hardlinked state accepted")
	}
}
