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

const installSnapshotLimit = 4096

type installPayload struct {
	Version  int           `json:"version"`
	Server   string        `json:"server"`
	Sequence uint64        `json:"sequence"`
	Record   InstallRecord `json:"record"`
}
type installSnapshot struct {
	Payload  installPayload `json:"payload"`
	Checksum string         `json:"sha256"`
}
type installDisk struct {
	guard    io.Closer
	lock     *os.File
	files    [2]*os.File
	server   string
	sequence uint64
	last     InstallRecord
}

func installBytes(payload installPayload) ([]byte, error) {
	if payload.Sequence == 0 || payload.Record.Validate() != nil || (payload.Record.Phase == "idle" && payload.Sequence > 2) || (payload.Record.Phase != "idle" && payload.Sequence <= 2) {
		return nil, errors.New("invalid install snapshot bounds")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return json.Marshal(installSnapshot{payload, hex.EncodeToString(sum[:])})
}
func readInstall(file *os.File, server string) (*installPayload, error) {
	raw, err := io.ReadAll(io.LimitReader(file, installSnapshotLimit+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > installSnapshotLimit {
		return nil, errors.New("oversize install snapshot")
	}
	var snapshot installSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing install data")
	}
	if snapshot.Payload.Version != 1 || snapshot.Payload.Server != server {
		return nil, errors.New("install snapshot version/UUID mismatch")
	}
	canonical, err := installBytes(snapshot.Payload)
	if err != nil {
		return nil, err
	}
	var expected installSnapshot
	if err = json.Unmarshal(canonical, &expected); err != nil {
		return nil, err
	}
	if expected.Checksum != snapshot.Checksum {
		return nil, errors.New("install snapshot checksum mismatch")
	}
	return &snapshot.Payload, nil
}
func openInstallDisk(options config.ConsoleLogs, server string) (result installBackend, record InstallRecord, err error) {
	if !config.UUID.MatchString(server) || !filepath.IsAbs(options.Directory) || filepath.Clean(options.Directory) != options.Directory || len(filepath.VolumeName(options.Directory)) != 2 {
		return nil, InstallRecord{}, errors.New("invalid install storage path/UUID")
	}
	server = strings.ToLower(server)
	path := filepath.Join(options.Directory, server)
	d := &installDisk{server: server}
	defer func() {
		if err != nil {
			d.close()
		}
	}()
	d.guard, err = guardedDirectory(path)
	if err != nil {
		return nil, InstallRecord{}, err
	}
	d.lock, err = openFile(filepath.Join(path, ".install.lock"), true, true)
	if err != nil {
		return nil, InstallRecord{}, err
	}
	info, err := d.lock.Stat()
	if err != nil {
		return nil, InstallRecord{}, err
	}
	if info.Size() != 0 {
		return nil, InstallRecord{}, errors.New("invalid install lock file")
	}
	var slots [2]*installPayload
	for i := range 2 {
		d.files[i], err = openFile(filepath.Join(path, fmt.Sprintf("install.%d.json", i)), true, true)
		if err != nil {
			return nil, InstallRecord{}, err
		}
		slots[i], err = readInstall(d.files[i], server)
		if err != nil {
			return nil, InstallRecord{}, fmt.Errorf("invalid install slot %d: %w", i, err)
		}
	}
	if slots[0] != nil && slots[1] != nil {
		a, b := slots[0].Sequence, slots[1].Sequence
		if a == b || (a > b && a-b != 1) || (b > a && b-a != 1) {
			return nil, InstallRecord{}, errors.New("inconsistent install slot sequence")
		}
	}
	for i, slot := range slots {
		if slot == nil {
			continue
		}
		if int(slot.Sequence%2) != i {
			return nil, InstallRecord{}, errors.New("misplaced install slot")
		}
		if slots[1-i] == nil && slot.Sequence != 1 {
			return nil, InstallRecord{}, errors.New("missing install snapshot")
		}
		if slot.Sequence > d.sequence {
			d.sequence = slot.Sequence
			record = slot.Record
		}
	}
	if slots[0] != nil && slots[1] != nil {
		older, newer := slots[0], slots[1]
		if older.Sequence > newer.Sequence {
			older, newer = newer, older
		}
		if err = installTransition(older.Record, newer.Record); err != nil {
			return nil, InstallRecord{}, err
		}
	}
	d.last = record
	// Initialize both slots as idle before any attempt can begin. Missing or
	// inconsistent later slots fail closed instead of reviving an older result.
	if d.sequence == 0 {
		record = InstallRecord{Phase: "idle"}
		if err = d.save(record); err != nil {
			return nil, InstallRecord{}, err
		}
	}
	if d.sequence == 1 {
		if record.Phase != "idle" {
			return nil, InstallRecord{}, errors.New("invalid initial install record")
		}
		if err = d.save(InstallRecord{Phase: "idle"}); err != nil {
			return nil, InstallRecord{}, err
		}
	}
	return d, record, nil
}
func (d *installDisk) save(record InstallRecord) error {
	if err := installTransition(d.last, record); err != nil {
		return err
	}
	if d.sequence == math.MaxUint64 {
		return errors.New("install sequence exhausted")
	}
	next := d.sequence + 1
	raw, err := installBytes(installPayload{1, d.server, next, record})
	if err != nil {
		return err
	}
	if len(raw) > installSnapshotLimit {
		return errors.New("oversize install snapshot")
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
	d.last = record
	return nil
}
func (d *installDisk) close() error {
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
