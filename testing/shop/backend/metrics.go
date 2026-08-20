package main

import (
	"context"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	gopsmem "github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	ordersPlaced     metric.Int64Counter
	revenueCounter   metric.Float64Counter
	checkoutValue    metric.Float64Histogram
	paymentsDeclined metric.Int64Counter
	couponsApplied   metric.Int64Counter
	couponFailures   metric.Int64Counter
)

func recordCouponFailure(ctx context.Context) {
	if couponFailures != nil {
		couponFailures.Add(ctx, 1)
	}
}

func recordCouponApplied(ctx context.Context, code, result string) {
	if couponsApplied != nil {
		couponsApplied.Add(ctx, 1, metric.WithAttributes(
			attribute.String("code", code),
			attribute.String("result", result),
		))
	}
}

func initMetrics() error {
	meter := otel.Meter(serviceName)
	var err error

	if ordersPlaced, err = meter.Int64Counter("shop.orders.placed", metric.WithUnit("1")); err != nil {
		return err
	}
	if revenueCounter, err = meter.Float64Counter("shop.revenue", metric.WithUnit("USD")); err != nil {
		return err
	}
	if checkoutValue, err = meter.Float64Histogram("shop.checkout.value", metric.WithUnit("USD")); err != nil {
		return err
	}
	if paymentsDeclined, err = meter.Int64Counter("shop.payments.declined", metric.WithUnit("1")); err != nil {
		return err
	}
	if couponsApplied, err = meter.Int64Counter("shop.coupons.applied", metric.WithUnit("1")); err != nil {
		return err
	}
	if couponFailures, err = meter.Int64Counter("shop.coupon.failures", metric.WithUnit("1")); err != nil {
		return err
	}

	cartItems, err := meter.Int64ObservableGauge("shop.cart.items", metric.WithUnit("1"))
	if err != nil {
		return err
	}

	// Built-in dashboard pages: Server Metrics reads cpu.used_pcnt / mem.used /
	// mem.total, Application Metrics reads the go.* family.
	cpuUsedPcnt, err := meter.Float64ObservableGauge("cpu.used_pcnt", metric.WithUnit("%"))
	if err != nil {
		return err
	}
	memUsed, err := meter.Float64ObservableGauge("mem.used", metric.WithUnit("MB"))
	if err != nil {
		return err
	}
	memTotal, err := meter.Float64ObservableGauge("mem.total", metric.WithUnit("MB"))
	if err != nil {
		return err
	}
	goRoutines, err := meter.Int64ObservableGauge("go.go_routines", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	heapObjects, err := meter.Int64ObservableGauge("go.heap_objects", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	numGC, err := meter.Int64ObservableGauge("go.num_gc", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	gcPause, err := meter.Float64ObservableGauge("go.gc_pause", metric.WithUnit("ns"))
	if err != nil {
		return err
	}

	// hostmetrics-style names charted by the "OTelemetry Server Agent" template.
	sysCPUUtil, err := meter.Float64ObservableGauge("system.cpu.utilization", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	load1, err := meter.Float64ObservableGauge("system.cpu.load_average.1m", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	load5, err := meter.Float64ObservableGauge("system.cpu.load_average.5m", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	load15, err := meter.Float64ObservableGauge("system.cpu.load_average.15m", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	sysMemUsage, err := meter.Float64ObservableGauge("system.memory.usage", metric.WithUnit("By"))
	if err != nil {
		return err
	}
	sysMemUtil, err := meter.Float64ObservableGauge("system.memory.utilization", metric.WithUnit("1"))
	if err != nil {
		return err
	}
	fsUsage, err := meter.Float64ObservableGauge("system.filesystem.usage", metric.WithUnit("By"))
	if err != nil {
		return err
	}
	fsUtil, err := meter.Float64ObservableGauge("system.filesystem.utilization", metric.WithUnit("1"))
	if err != nil {
		return err
	}

	var lastNumGC uint32
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		var totalCart int64
		row := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(qty), 0) FROM cart_items`)
		_ = row.Scan(&totalCart)
		o.ObserveInt64(cartItems, totalCart)

		if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
			o.ObserveFloat64(cpuUsedPcnt, pcts[0])
			o.ObserveFloat64(sysCPUUtil, pcts[0]/100)
		}
		if vm, err := gopsmem.VirtualMemory(); err == nil {
			o.ObserveFloat64(memUsed, float64(vm.Used)/1024/1024)
			o.ObserveFloat64(memTotal, float64(vm.Total)/1024/1024)
			o.ObserveFloat64(sysMemUsage, float64(vm.Used))
			o.ObserveFloat64(sysMemUtil, vm.UsedPercent/100)
		}
		if avg, err := load.Avg(); err == nil {
			o.ObserveFloat64(load1, avg.Load1)
			o.ObserveFloat64(load5, avg.Load5)
			o.ObserveFloat64(load15, avg.Load15)
		}
		if du, err := disk.Usage("/"); err == nil {
			o.ObserveFloat64(fsUsage, float64(du.Used))
			o.ObserveFloat64(fsUtil, du.UsedPercent/100)
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		o.ObserveInt64(goRoutines, int64(runtime.NumGoroutine()))
		o.ObserveInt64(heapObjects, int64(ms.HeapObjects))
		o.ObserveInt64(numGC, int64(ms.NumGC-lastNumGC))
		lastNumGC = ms.NumGC
		if ms.NumGC > 0 {
			o.ObserveFloat64(gcPause, float64(ms.PauseNs[(ms.NumGC+255)%256]))
		}
		return nil
	}, cartItems, cpuUsedPcnt, memUsed, memTotal, goRoutines, heapObjects, numGC, gcPause,
		sysCPUUtil, load1, load5, load15, sysMemUsage, sysMemUtil, fsUsage, fsUtil)
	return err
}
