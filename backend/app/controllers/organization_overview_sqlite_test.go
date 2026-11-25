//go:build !telemetry_ch && !transactional_pg && !telemetry_duckdb

package controllers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
)

// The filesystem points carry the attributes the hostmetrics receiver sends
// (device, mode, mountpoint, type) and no state, as the Traceway OTel Agent
// reported them in the reproduction of issue #440.
func TestServerRowsReadDiskFromHostmetricsFilesystemPoints(t *testing.T) {
	setupSetupControllerDB(t)
	project := &models.Project{Id: uuid.New(), Name: "infra"}
	now := time.Now().UTC()
	at := now.Add(-time.Minute)

	point := func(server, name string, value float64, tags map[string]string) models.MetricPoint {
		all := map[string]string{"server_name": server, "host.name": server}
		for k, v := range tags {
			all[k] = v
		}
		return models.MetricPoint{ProjectId: project.Id, Name: name, Value: value, Tags: all, RecordedAt: at}
	}
	mount := func(server, device, mode, mountpoint, fsType string, value float64) models.MetricPoint {
		return point(server, models.MetricNameSystemFilesystemUtilization, value, map[string]string{
			"device": device, "mode": mode, "mountpoint": mountpoint, "type": fsType,
		})
	}

	points := []models.MetricPoint{}
	for _, server := range []string{"agent-host", "snap-only-host", "no-mode-host", "no-filesystem-host"} {
		points = append(points, point(server, models.MetricNameSystemCPUUtilization, 0.9, map[string]string{"cpu": "cpu0", "state": "idle"}))
	}
	points = append(points,
		mount("agent-host", "/dev/sda1", "rw", "/", "ext4", 0.55),
		mount("agent-host", "/dev/sda2", "rw", "/boot", "ext4", 0.06),
		mount("agent-host", "/dev/sda3", "rw", "/boot/efi", "vfat", 0.16),
		mount("agent-host", "/dev/loop0", "ro", "/snap/core22/2045", "squashfs", 1),
		mount("snap-only-host", "/dev/loop0", "ro", "/snap/core22/2045", "squashfs", 1),
		mount("snap-only-host", "/dev/loop1", "ro", "/snap/lxd/31333", "squashfs", 0.97),
		point("no-mode-host", models.MetricNameSystemFilesystemUtilization, 0.42, map[string]string{"mountpoint": "/"}),
	)
	if err := telemetry.MetricPointRepository.InsertAsync(t.Context(), points); err != nil {
		t.Fatal(err)
	}

	rows, err := loadProjectServerRows(t.Context(), project, 0, now.Add(-30*time.Minute), now)
	if err != nil {
		t.Fatalf("loadProjectServerRows: %v", err)
	}
	disk := map[string]*float64{}
	for _, row := range rows {
		disk[row.ServerName] = row.DiskPct
	}

	want := map[string]float64{
		"agent-host":     55,
		"snap-only-host": 100,
		"no-mode-host":   42,
	}
	for server, pct := range want {
		got := disk[server]
		if got == nil {
			t.Errorf("%s: Disk is empty, want %v%%", server, pct)
			continue
		}
		if diff := *got - pct; diff > 0.001 || diff < -0.001 {
			t.Errorf("%s: Disk = %v%%, want %v%%", server, *got, pct)
		}
	}
	if got, ok := disk["no-filesystem-host"]; !ok || got != nil {
		t.Errorf("a host without filesystem metrics should have a row and an empty Disk, got present=%v value=%v", ok, got)
	}
}
