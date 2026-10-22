package consolelog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const receiptAttempt = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func TestInstallStateDurablePhasesAndExclusiveWriter(t *testing.T) {
	options := budgetOptions(t)
	store, record, err := OpenInstallState(options, budgetTestID)
	if err != nil || record.Phase != "idle" {
		t.Fatal(record, err)
	}
	if other, _, err := OpenInstallState(options, budgetTestID); err == nil {
		other.Close(context.Background())
		t.Fatal("second writer accepted")
	}
	for _, next := range []InstallRecord{
		{Phase: "running", AttemptID: receiptAttempt, Reinstall: true},
		{Phase: "result", AttemptID: receiptAttempt, Successful: true, Reinstall: true},
		{Phase: "acknowledged", AttemptID: receiptAttempt, Successful: true, Reinstall: true},
		{Phase: "running", AttemptID: budgetTestID},
		{Phase: "result", AttemptID: budgetTestID},
		{Phase: "acknowledged", AttemptID: budgetTestID},
	} {
		if err := store.Save(next); err != nil {
			t.Fatal(err)
		}
		if err := store.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		store, record, err = OpenInstallState(options, budgetTestID)
		if err != nil || record != next {
			t.Fatal(record, next, err)
		}
	}
	path := filepath.Join(options.Directory, budgetTestID, "install.1.json")
	if f, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0600); err == nil {
		f.Close()
		t.Fatal("conflicting writer accepted")
	}
	store.Close(context.Background())
	if store.Save(record) == nil {
		t.Fatal("closed writer accepted")
	}
	if err := os.Link(path, path+".link"); err != nil {
		t.Fatal(err)
	}
	if s, _, err := OpenInstallState(options, budgetTestID); err == nil {
		s.Close(context.Background())
		t.Fatal("hardlink accepted")
	}
}

func TestInstallStateRejectsCorruptionAndInvalidTransitions(t *testing.T) {
	for _, mode := range []string{"torn", "missing", "empty", "checksum", "wrong-server", "oversize", "wrong-sequence", "wrong-attempt", "invalid-phase", "unknown-field", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			options := budgetOptions(t)
			store, _, err := OpenInstallState(options, budgetTestID)
			if err != nil {
				t.Fatal(err)
			}
			running := InstallRecord{Phase: "running", AttemptID: receiptAttempt}
			if err := store.Save(running); err != nil {
				t.Fatal(err)
			}
			result := running
			result.Phase = "result"
			result.Successful = true
			if err := store.Save(result); err != nil {
				t.Fatal(err)
			}
			store.Close(context.Background())
			path := filepath.Join(options.Directory, budgetTestID, "install.0.json")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "torn":
				b = []byte("TORN")
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "empty":
				b = nil
			case "oversize":
				b = make([]byte, 4097)
			case "checksum":
				b[len(b)-4] ^= 1
			case "unknown-field":
				b = append([]byte(`{"extra":1,`), b[1:]...)
			case "trailing":
				b = append(b, []byte(" {}")...)
			default:
				payload := installPayload{1, budgetTestID, 4, result}
				if mode == "wrong-server" {
					payload.Server = receiptAttempt
				}
				if mode == "wrong-sequence" {
					payload.Sequence = 8
				}
				if mode == "wrong-attempt" {
					payload.Record.AttemptID = budgetTestID
				}
				if mode == "invalid-phase" {
					payload.Record.Phase = "running"
					payload.Record.Successful = false
				}
				b, err = installBytes(payload)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode != "missing" {
				if err := os.WriteFile(path, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if reopened, _, err := OpenInstallState(options, budgetTestID); err == nil {
				reopened.Close(context.Background())
				t.Fatal("invalid receipt recovered older result")
			}
		})
	}
}
