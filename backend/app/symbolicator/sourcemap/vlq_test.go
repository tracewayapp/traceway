package sourcemap

import "testing"

func TestVLQRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 15, 16, -16, 31, 32, 1000, -1000, 1 << 20, -(1 << 20), 1<<30 - 1}
	var encoded []byte
	for _, v := range values {
		encoded = AppendVLQ(encoded, v)
	}
	decoded, err := decodeVLQ(string(encoded), nil)
	if err != nil {
		t.Fatalf("decodeVLQ: %v", err)
	}
	if len(decoded) != len(values) {
		t.Fatalf("decoded %d values, want %d", len(decoded), len(values))
	}
	for i, v := range values {
		if decoded[i] != v {
			t.Errorf("value %d: got %d, want %d", i, decoded[i], v)
		}
	}
}
