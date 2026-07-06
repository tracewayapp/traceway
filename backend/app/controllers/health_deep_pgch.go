//go:build telemetry_ch

package controllers

import (
	"context"
	"time"

	"github.com/tracewayapp/traceway/backend/app/chdb"
)

func fetchCHHealth(parent context.Context) HealthDeepResponse {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	resp := HealthDeepResponse{}

	if err := chdb.Conn.Exec(ctx, "SELECT 1"); err != nil {
		resp.CHReachable = false
		return resp
	}
	resp.CHReachable = true

	if row := chdb.Conn.QueryRow(ctx, "SELECT toInt64(uptime())"); row != nil {
		var uptime int64
		if err := row.Scan(&uptime); err == nil {
			resp.CHUptimeSec = uptime
		}
	}

	if rows, err := chdb.Conn.Query(ctx, "SELECT table, sum(rows > 0) AS parts FROM system.parts WHERE active GROUP BY table ORDER BY parts DESC"); err == nil {
		for rows.Next() {
			var pt TableParts
			if err := rows.Scan(&pt.Table, &pt.Parts); err != nil {
				continue
			}
			resp.PartsByTable = append(resp.PartsByTable, pt)
			resp.PartsCount += pt.Parts
		}
		rows.Close()
	}

	if row := chdb.Conn.QueryRow(ctx, "SELECT count(), coalesce(max(elapsed), 0) FROM system.merges"); row != nil {
		var n int64
		var longest float64
		if err := row.Scan(&n, &longest); err == nil {
			resp.ActiveMerges = n
			resp.LongestMergeSec = longest
		}
	}

	if rows, err := chdb.Conn.Query(ctx, "SELECT name, value, toString(last_error_time) FROM system.errors WHERE value > 0 ORDER BY last_error_time DESC LIMIT 20"); err == nil {
		for rows.Next() {
			var e CHError
			if err := rows.Scan(&e.Name, &e.Value, &e.LastErrorTime); err != nil {
				continue
			}
			resp.ErrorsRecent = append(resp.ErrorsRecent, e)
		}
		rows.Close()
	}

	if row := chdb.Conn.QueryRow(ctx, "SELECT toInt64(value) FROM system.asynchronous_metrics WHERE metric = 'MemoryResident'"); row != nil {
		var v int64
		if err := row.Scan(&v); err == nil {
			resp.MemoryUsageBytes = v
		}
	}
	if row := chdb.Conn.QueryRow(ctx, "SELECT toInt64(value) FROM system.asynchronous_metrics WHERE metric = 'OSMemoryTotal'"); row != nil {
		var v int64
		if err := row.Scan(&v); err == nil {
			resp.MemoryTotalBytes = v
		}
	}

	return resp
}
