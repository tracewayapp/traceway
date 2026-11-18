package shipments

import (
	"fmt"
	"strings"

	"example.com/shipping-api/internal/httpx"
)

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
	return nil
}
