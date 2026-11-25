//go:build telemetry_ch

package chdb

import (
	"slices"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
)

func entriesOf(m column.IterableOrderedMap) (keys, values []string) {
	for it := m.Iterator(); it.Next(); {
		keys = append(keys, it.Key().(string))
		values = append(values, it.Value().(string))
	}
	return keys, values
}

func TestSortedMapIteratesInByteOrder(t *testing.T) {
	var sorted SortedMap
	entries := map[string]string{"server_name": "a", "Zone": "b", "état": "c", "host.name": "d", "host": "e", "": "f"}
	for attempt := 0; attempt < 20; attempt++ {
		keys, values := entriesOf(sorted.Of(entries))
		if want := []string{"", "Zone", "host", "host.name", "server_name", "état"}; !slices.Equal(keys, want) {
			t.Fatalf("keys %q, want %q", keys, want)
		}
		if want := []string{"f", "b", "e", "d", "a", "c"}; !slices.Equal(values, want) {
			t.Fatalf("values %q, want %q", values, want)
		}
	}
}

func TestSortedMapIsReusedAcrossRows(t *testing.T) {
	var sorted SortedMap
	sorted.Of(map[string]string{"b": "2", "a": "1", "c": "3"})
	if keys, _ := entriesOf(sorted.Of(map[string]string{"z": "26"})); !slices.Equal(keys, []string{"z"}) {
		t.Fatalf("a second row kept keys of the first: %q", keys)
	}
	if keys, _ := entriesOf(sorted.Of(nil)); len(keys) != 0 {
		t.Fatalf("a nil map must be an empty one, got %q", keys)
	}
	// The driver iterates a row once, but a retried append must see every entry again.
	row := sorted.Of(map[string]string{"b": "2", "a": "1"})
	first, _ := entriesOf(row)
	second, _ := entriesOf(row)
	if !slices.Equal(first, second) || len(first) != 2 {
		t.Fatalf("iterations differ: %q then %q", first, second)
	}
}
