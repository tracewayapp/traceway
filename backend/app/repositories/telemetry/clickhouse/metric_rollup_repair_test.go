//go:build telemetry_ch

package clickhouse

import "testing"

func TestRollupRepairStagedName_CarriesThePartitionAndTheFirstDay(t *testing.T) {
	for _, partition := range []string{"202609", "2026_09", "all"} {
		name := rollupRepairStagedName("metric_points_1h", partition, "2026-09-03")
		gotPartition, gotSince, err := rollupRepairStagedParts("metric_points_1h", name)
		if err != nil || gotPartition != partition || gotSince != "2026-09-03" {
			t.Errorf("%s read back as partition %q since %q (%v)", name, gotPartition, gotSince, err)
		}
	}
	// A name without a first day cannot bound the delete, so the pass must stop on it.
	for _, name := range []string{"metric_points_1h_tagrepair_202609", "metric_points_1h_tagrepair_202609_20261340", "metric_points_1h_tagrepair__20260903"} {
		if partition, since, err := rollupRepairStagedParts("metric_points_1h", name); err == nil {
			t.Errorf("%s must be refused, read as partition %q since %q", name, partition, since)
		}
	}
}
