package shipments

import "testing"

func TestValidateAddress(t *testing.T) {
	ok := Address{Name: "Ada", Line1: "500 Howard St", City: "San Francisco", PostalCode: "94105", Country: "US"}
	if err := validateAddress(ok); err != nil {
		t.Fatalf("valid address rejected: %v", err)
	}
	if err := validateAddress(Address{City: "Austin", PostalCode: "73301", Country: "US"}); err == nil {
		t.Fatal("an address without a street line must be rejected")
	}
}
