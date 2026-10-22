package consolelog

import (
	"errors"
	"wings-windows/internal/config"
)

type InstallRecord struct {
	Phase      string `json:"phase"`
	AttemptID  string `json:"attempt_id"`
	Successful bool   `json:"successful"`
	Reinstall  bool   `json:"reinstall"`
}

func (r InstallRecord) Validate() error {
	if r.Phase == "idle" && r.AttemptID == "" && !r.Successful && !r.Reinstall {
		return nil
	}
	if !config.UUID.MatchString(r.AttemptID) {
		return errors.New("invalid install attempt ID")
	}
	switch r.Phase {
	case "running":
		if r.Successful {
			return errors.New("running installer cannot be successful")
		}
	case "result", "acknowledged":
	default:
		return errors.New("invalid install record phase")
	}
	return nil
}
func installTransition(old, next InstallRecord) error {
	if err := next.Validate(); err != nil {
		return err
	}
	if old.Phase == "" || old.Phase == "idle" {
		if next.Phase == "idle" || next.Phase == "running" {
			return nil
		}
	}
	if old.Phase == "acknowledged" && next.Phase == "running" && next.AttemptID != old.AttemptID {
		return nil
	}
	if old.AttemptID == next.AttemptID && old.Reinstall == next.Reinstall {
		if old.Phase == "running" && next.Phase == "result" {
			return nil
		}
		if old.Phase == "result" && next.Phase == "acknowledged" && old.Successful == next.Successful {
			return nil
		}
	}
	return errors.New("invalid install record transition")
}
