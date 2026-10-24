package api

import (
	"encoding/json"
	"testing"
	"time"
	"wings-windows/internal/diskusage"
)

func TestDiskUsageAPIAndConsoleStats(t *testing.T) {
	a, _ := setup(t)
	putDownload(t, a, "nested/data", []byte{0, 255, 1, 2})
	putDownload(t, a, ".hidden", []byte{7})
	var response struct {
		Utilization struct {
			DiskBytes int64 `json:"disk_bytes"`
			Native    struct {
				Available bool             `json:"disk_available"`
				Disk      diskusage.Status `json:"disk"`
			} `json:"native_metrics"`
		} `json:"utilization"`
	}
	until := time.Now().Add(3 * time.Second)
	for {
		w := request(t, a, "GET", "/api/servers/"+serverID, "", secret)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
			t.Fatal(e)
		}
		if response.Utilization.Native.Available {
			break
		}
		if time.Now().After(until) {
			t.Fatal("disk measurement not available", w.Body)
		}
		time.Sleep(time.Millisecond)
	}
	u := response.Utilization
	if u.DiskBytes != 5 || u.Native.Disk.Bytes != 5 || u.Native.Disk.Files != 2 || u.Native.Disk.Directories != 1 || u.Native.Disk.Stale || u.Native.Disk.QuotaEnforced || u.Native.Disk.MeasuredAt == "" {
		t.Fatal(u)
	}
	var stats map[string]any
	if e := json.Unmarshal([]byte(a.Manager.Get(serverID).StatsJSON()), &stats); e != nil {
		t.Fatal(e)
	}
	if stats["disk_bytes"] != float64(5) {
		t.Fatal(stats)
	}
}
