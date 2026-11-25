//go:build telemetry_ch

package chdb

import (
	"slices"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
)

// SortedMap appends a Map(String, String) value with its entries in key order.
// ClickHouse compares maps in the order their entries were written, and the driver
// writes a Go map in iteration order, which differs from one call to the next: where
// the map is part of a sorting key or a GROUP BY, the same entries in another order
// are another row. The order is the one mapSort gives (keys compared as bytes).
//
// One value serves a whole batch: Of returns the receiver, valid until the next Of.
type SortedMap struct {
	keys    []string
	entries map[string]string
	at      int
}

func (m *SortedMap) Of(entries map[string]string) *SortedMap {
	m.keys = m.keys[:0]
	for key := range entries {
		m.keys = append(m.keys, key)
	}
	slices.Sort(m.keys)
	m.entries = entries
	return m
}

func (m *SortedMap) Iterator() column.MapIterator {
	m.at = -1
	return m
}

func (m *SortedMap) Next() bool {
	m.at++
	return m.at < len(m.keys)
}

func (m *SortedMap) Key() any { return m.keys[m.at] }

func (m *SortedMap) Value() any { return m.entries[m.keys[m.at]] }

// Put is the scan half of the driver's ordered map interface.
func (m *SortedMap) Put(key any, value any) {
	if m.entries == nil {
		m.entries = map[string]string{}
	}
	name, _ := key.(string)
	text, _ := value.(string)
	if _, known := m.entries[name]; !known {
		m.keys = append(m.keys, name)
	}
	m.entries[name] = text
}
