package shipments

import (
	"errors"
	"testing"

	"example.com/shipping-api/internal/httpx"
)

func TestHiddenInternationalPostalCodesAccepted(t *testing.T) {
	for _, a := range []Address{
		{Line1: "10 Downing St", City: "London", PostalCode: "SW1A 2AA", Country: "GB"},
		{Line1: "Damrak 1", City: "Amsterdam", PostalCode: "1012 LG", Country: "NL"},
		{Line1: "290 Bremner Blvd", City: "Toronto", PostalCode: "M5V 3L9", Country: "CA"},
		{Line1: "1-1 Chiyoda", City: "Tokyo", PostalCode: "100-8111", Country: "JP"},
		{Line1: "Unter den Linden 1", City: "Berlin", PostalCode: "10117", Country: "DE"},
		{Line1: "500 Howard St", City: "San Francisco", PostalCode: "94105-1420", Country: "US"},
	} {
		if err := validateAddress(a); err != nil {
			t.Errorf("%s postal code %q rejected: %v", a.Country, a.PostalCode, err)
		}
	}
}

func TestHiddenUSZipStillChecked(t *testing.T) {
	for _, zip := range []string{"9410", "ABCDE", "941051"} {
		err := validateAddress(Address{Line1: "1 Main St", City: "Austin", PostalCode: zip, Country: "US"})
		if err == nil {
			t.Errorf("US ZIP %q accepted", zip)
			continue
		}
		if !errors.Is(err, httpx.ErrInvalid) {
			t.Errorf("US ZIP %q: the error must stay a 422 (httpx.ErrInvalid): %v", zip, err)
		}
	}
}
