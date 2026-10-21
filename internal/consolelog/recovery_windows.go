package consolelog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wings-windows/internal/config"
)

const recoverySnapshotLimit = 4096

type budgetPayload struct {
	Version  int         `json:"version"`
	Server   string      `json:"server"`
	Sequence uint64      `json:"sequence"`
	Attempts []time.Time `json:"attempts"`
}
type budgetSnapshot struct {
	Payload  budgetPayload `json:"payload"`
	Checksum string        `json:"sha256"`
}
type budgetDisk struct {
	guard    io.Closer
	lock     *os.File
	files    [2]*os.File
	server   string
	sequence uint64
}

func snapshotBytes(payload budgetPayload) ([]byte, error) {
	if payload.Sequence == 0 || len(payload.Attempts) > 10 {
		return nil, errors.New("invalid recovery snapshot bounds")
	}
	for _, when := range payload.Attempts {
		if when.IsZero() {
			return nil, errors.New("invalid recovery timestamp")
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return json.Marshal(budgetSnapshot{payload, hex.EncodeToString(sum[:])})
}
func readSnapshot(file *os.File, server string) (*budgetPayload, error) {
	raw, err := io.ReadAll(io.LimitReader(file, recoverySnapshotLimit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > recoverySnapshotLimit {
		return nil, errors.New("oversize recovery snapshot")
	}
	var snapshot budgetSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing recovery data")
	}
	if snapshot.Payload.Version != 1 || snapshot.Payload.Server != server {
		return nil, errors.New("recovery snapshot version/UUID mismatch")
	}
	canonical, err := snapshotBytes(snapshot.Payload)
	if err != nil {
		return nil, err
	}
	var expected budgetSnapshot
	if err = json.Unmarshal(canonical, &expected); err != nil {
		return nil, err
	}
	if expected.Checksum != snapshot.Checksum {
		return nil, errors.New("recovery snapshot checksum mismatch")
	}
	return &snapshot.Payload, nil
}
func openRecoveryDisk(options config.ConsoleLogs, server string) (result recoveryDisk, history []time.Time, err error) {
	if !config.UUID.MatchString(server) || !filepath.IsAbs(options.Directory) || filepath.Clean(options.Directory) != options.Directory || len(filepath.VolumeName(options.Directory)) != 2 {
		return nil, nil, errors.New("invalid recovery storage path/UUID")
	}
	server = strings.ToLower(server)
	path := filepath.Join(options.Directory, server)
	d := &budgetDisk{server: server}
	defer func() {
		if err != nil {
			d.close()
		}
	}()
	d.guard, err = guardedDirectory(path)
	if err != nil {
		return nil, nil, err
	}
	d.lock, err = openFile(filepath.Join(path, ".recovery.lock"), true, true)
	if err != nil {
		return nil, nil, err
	}
	info, err := d.lock.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.Size() != 0 {
		return nil, nil, errors.New("invalid recovery lock file")
	}
	var slots [2]*budgetPayload
	for i := range 2 {
		d.files[i], err = openFile(filepath.Join(path, fmt.Sprintf("recovery.%d.json", i)), true, true)
		if err != nil {
			return nil, nil, err
		}
		slots[i], err = readSnapshot(d.files[i], server)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid recovery slot %d: %w", i, err)
		}
	}
	if slots[0] != nil && slots[1] != nil {
		a, b := slots[0].Sequence, slots[1].Sequence
		if a == b || (a > b && a-b != 1) || (b > a && b-a != 1) {
			return nil, nil, errors.New("inconsistent recovery slot sequence")
		}
	}
	for i, slot := range slots {
		if slot == nil {
			continue
		}
		if int(slot.Sequence%2) != i {
			return nil, nil, errors.New("misplaced recovery slot")
		}
		if slots[1-i] == nil && slot.Sequence != 1 {
			return nil, nil, errors.New("missing recovery snapshot")
		}
		if slot.Sequence > d.sequence {
			d.sequence = slot.Sequence
			history = append([]time.Time(nil), slot.Attempts...)
		}
	}
	return d, history, nil
}
func (d *budgetDisk) save(history []time.Time) error {
	if d.sequence == math.MaxUint64 {
		return errors.New("recovery sequence exhausted")
	}
	next := d.sequence + 1
	raw, err := snapshotBytes(budgetPayload{1, d.server, next, history})
	if err != nil {
		return err
	}
	if len(raw) > recoverySnapshotLimit {
		return errors.New("oversize recovery snapshot")
	}
	file := d.files[next%2]
	if err = file.Truncate(0); err != nil {
		return err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = file.Write(raw); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	d.sequence = next
	return nil
}
func (d *budgetDisk) close() error {
	var result error
	for _, file := range d.files {
		if file != nil {
			result = errors.Join(result, file.Close())
		}
	}
	if d.lock != nil {
		result = errors.Join(result, d.lock.Close())
	}
	if d.guard != nil {
		result = errors.Join(result, d.guard.Close())
	}
	return result
}
