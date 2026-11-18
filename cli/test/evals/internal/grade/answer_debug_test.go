package grade

import "testing"

func TestCulpritMatching(t *testing.T) {
	for _, c := range []struct {
		got, want string
		ok        bool
	}{
		{"(*OrderHandler).List", "List", true},
		{"OrderHandler.List", "List", true},
		{"func formatAddress", "formatAddress", true},
		{"formatAddress(a *Address)", "formatAddress", true},
		{"FeedAdapter.bindImage(FeedItem)", "bindImage", true},
		{"`renderInvoice`", "renderInvoice", true},
		{"chatURL.getter", "chatURL", true},
		{"renderTotals", "renderInvoice", false},
		{"", "List", false},
	} {
		if got := sameSymbol(c.got, c.want); got != c.ok {
			t.Errorf("sameSymbol(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
	for _, c := range []struct {
		got, want string
		ok        bool
	}{
		{"internal/invoices/render.go", "internal/invoices/render.go", true},
		{"./internal/invoices/render.go:37", "internal/invoices/render.go", true},
		{"internal\\invoices\\render.go:37:5", "internal/invoices/render.go", true},
		{"/work/internal/invoices/render.go", "internal/invoices/render.go", true},
		{"`render.go`", "internal/invoices/render.go", false},
		{"internal/invoices/model.go", "internal/invoices/render.go", false},
	} {
		if got := samePath(c.got, c.want); got != c.ok {
			t.Errorf("samePath(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
}
