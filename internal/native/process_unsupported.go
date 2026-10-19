//go:build !windows

package native

import "errors"

type platformProcess struct{}

func startPlatform(Spec) (*Process, error)        { return nil, errors.New("native runtime requires Windows") }
func (platformProcess) kill() error               { return errors.New("Windows required") }
func (platformProcess) stats() (Stats, error)     { return Stats{}, errors.New("Windows required") }
func (platformProcess) updateLimits(Limits) error { return errors.New("Windows required") }
