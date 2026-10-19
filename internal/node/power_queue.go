package node

import (
	"errors"
	"fmt"
	"time"
)

func normalizeWaitSeconds(value int) int {
	if value < 0 || value > 300 {
		return 30
	}
	return value
}

// EnqueuePower accepts an HTTP action independently of the request lifetime.
// WaitSeconds bounds lock acquisition, not how long a graceful stop may take.
func (s *Server) EnqueuePower(action string, waitSeconds int) error {
	switch action {
	case "kill":
		return s.Power(action)
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("unknown power action")
	}
	waitSeconds = normalizeWaitSeconds(waitSeconds)
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return errors.New("node is shutting down")
	}
	if (action == "start" || action == "restart") && s.settings.Suspended {
		s.mu.Unlock()
		return ErrSuspended
	}
	if s.pendingPower >= 16 {
		s.mu.Unlock()
		return ErrPowerQueueFull
	}
	if s.powerEpoch == nil {
		s.powerEpoch = make(chan struct{})
	}
	epoch := s.powerEpoch
	s.pendingPower++
	s.mu.Unlock()
	expires := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	go func() {
		defer func() { s.mu.Lock(); s.pendingPower--; s.mu.Unlock() }()
		deadline := time.NewTimer(time.Until(expires))
		defer deadline.Stop()
		poll := time.NewTicker(10 * time.Millisecond)
		defer poll.Stop()
		for {
			if waitSeconds > 0 && !time.Now().Before(expires) {
				s.log(fmt.Sprintf("[Windows node] power %s timed out waiting for another action (%d seconds)", action, waitSeconds))
				return
			}
			err := s.powerWithEpoch(action, epoch)
			if err == nil || errors.Is(err, ErrPowerCancelled) {
				return
			}
			if !errors.Is(err, ErrPowerBusy) {
				s.log("[Windows node] " + err.Error())
				return
			}
			if waitSeconds == 0 {
				s.log("[Windows node] " + ErrPowerBusy.Error())
				return
			}
			select {
			case <-epoch:
				return
			case <-deadline.C:
				s.log(fmt.Sprintf("[Windows node] power %s timed out waiting for another action (%d seconds)", action, waitSeconds))
				return
			case <-poll.C:
			}
		}
	}()
	return nil
}
