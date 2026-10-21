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
	"wings-windows/internal/config"
)

const intentSnapshotLimit = 4096

type intentPayload struct {
	Version  int    `json:"version"`
	Server   string `json:"server"`
	Sequence uint64 `json:"sequence"`
	Running  bool   `json:"running"`
}
type intentSnapshot struct {
	Payload  intentPayload `json:"payload"`
	Checksum string        `json:"sha256"`
}
type intentDisk struct {
	guard    io.Closer
	lock     *os.File
	files    [2]*os.File
	server   string
	sequence uint64
}

func intentBytes(payload intentPayload) ([]byte, error) {
	if payload.Sequence == 0 {
		return nil, errors.New("invalid intent snapshot bounds")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return json.Marshal(intentSnapshot{payload, hex.EncodeToString(sum[:])})
}
func readIntent(file *os.File, server string) (*intentPayload, error) {
	raw, err := io.ReadAll(io.LimitReader(file, intentSnapshotLimit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > intentSnapshotLimit {
		return nil, errors.New("oversize intent snapshot")
	}
	var snapshot intentSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing intent data")
	}
	if snapshot.Payload.Version != 1 || snapshot.Payload.Server != server {
		return nil, errors.New("intent snapshot version/UUID mismatch")
	}
	canonical, err := intentBytes(snapshot.Payload)
	if err != nil {
		return nil, err
	}
	var expected intentSnapshot
	if err = json.Unmarshal(canonical, &expected); err != nil {
		return nil, err
	}
	if expected.Checksum != snapshot.Checksum {
		return nil, errors.New("intent snapshot checksum mismatch")
	}
	return &snapshot.Payload, nil
}
func openIntentDisk(options config.ConsoleLogs, server string) (result intentBackend, running bool, err error) {
	if !config.UUID.MatchString(server) || !filepath.IsAbs(options.Directory) || filepath.Clean(options.Directory) != options.Directory || len(filepath.VolumeName(options.Directory)) != 2 {
		return nil, false, errors.New("invalid intent storage path/UUID")
	}
	server = strings.ToLower(server)
	path := filepath.Join(options.Directory, server)
	d := &intentDisk{server: server}
	defer func() {
		if err != nil {
			d.close()
		}
	}()
	d.guard, err = guardedDirectory(path)
	if err != nil {
		return nil, false, err
	}
	d.lock, err = openFile(filepath.Join(path, ".intent.lock"), true, true)
	if err != nil {
		return nil, false, err
	}
	info, err := d.lock.Stat()
	if err != nil {
		return nil, false, err
	}
	if info.Size() != 0 {
		return nil, false, errors.New("invalid intent lock file")
	}
	var slots [2]*intentPayload
	for i := range 2 {
		d.files[i], err = openFile(filepath.Join(path, fmt.Sprintf("intent.%d.json", i)), true, true)
		if err != nil {
			return nil, false, err
		}
		slots[i], err = readIntent(d.files[i], server)
		if err != nil {
			return nil, false, fmt.Errorf("invalid intent slot %d: %w", i, err)
		}
	}
	if slots[0] != nil && slots[1] != nil {
		a, b := slots[0].Sequence, slots[1].Sequence
		if a == b || (a > b && a-b != 1) || (b > a && b-a != 1) {
			return nil, false, errors.New("inconsistent intent slot sequence")
		}
	}
	for i, slot := range slots {
		if slot == nil {
			continue
		}
		if int(slot.Sequence%2) != i {
			return nil, false, errors.New("misplaced intent slot")
		}
		if slots[1-i] == nil && slot.Sequence != 1 {
			return nil, false, errors.New("missing intent snapshot")
		}
		if slot.Sequence > d.sequence {
			d.sequence = slot.Sequence
			running = slot.Running
		}
	}
	// Both slots start with confirmed stopped snapshots before running intent
	// can ever be recorded. A missing later slot therefore cannot masquerade
	// as a legitimate first running snapshot and roll back a manual stop.
	if d.sequence == 0 {
		if err = d.save(false); err != nil {
			return nil, false, err
		}
	}
	if d.sequence == 1 {
		if running {
			return nil, false, errors.New("invalid initial running intent")
		}
		if err = d.save(false); err != nil {
			return nil, false, err
		}
	}
	return d, running, nil
}
func (d *intentDisk) save(running bool) error {
	if d.sequence == math.MaxUint64 {
		return errors.New("intent sequence exhausted")
	}
	next := d.sequence + 1
	raw, err := intentBytes(intentPayload{1, d.server, next, running})
	if err != nil {
		return err
	}
	if len(raw) > intentSnapshotLimit {
		return errors.New("oversize intent snapshot")
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
func (d *intentDisk) close() error {
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
