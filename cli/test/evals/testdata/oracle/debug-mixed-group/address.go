package shipments

import (
	"fmt"
	"regexp"
	"strings"

	"example.com/shipping-api/internal/httpx"
)

// usZIPRe is the US ZIP or ZIP+4 format. Other countries use their own
// formats (SW1A 1AA, 1012 AB, M5V 2T6, ...), so only US codes are checked
// against it.
var usZIPRe = regexp.MustCompile(`^\d{5}(-\d{4})?$`)

// validateAddress rejects destinations the carriers cannot deliver to.
func validateAddress(a Address) error {
	if strings.TrimSpace(a.Line1) == "" || strings.TrimSpace(a.City) == "" {
		return fmt.Errorf("%w: address line and city are required", httpx.ErrInvalid)
	}
	if len(a.Country) != 2 {
		return fmt.Errorf("%w: country %q is not an ISO 3166 code", httpx.ErrInvalid, a.Country)
	}
	if strings.TrimSpace(a.PostalCode) == "" {
		return fmt.Errorf("%w: postal code is required", httpx.ErrInvalid)
	}
	if a.Country == "US" && !usZIPRe.MatchString(a.PostalCode) {
		return fmt.Errorf("%w: postal code %q is not valid for %s", httpx.ErrInvalid, a.PostalCode, a.Country)
	}
	return nil
}
