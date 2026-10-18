// Package pdf renders account statements.
package pdf

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"

	"example.com/statements-api/internal/statements"
)

// Renderer lays out a statement and compresses its page streams. The font is
// embedded so the PDF prints identically everywhere.
type Renderer struct {
	Font []byte
}

// NewRenderer returns a renderer with the embedded statement font.
func NewRenderer(font []byte) *Renderer { return &Renderer{Font: font} }

// Render returns the PDF bytes for the account's invoices.
func (r *Renderer) Render(ctx context.Context, acct *statements.Account, invoices []statements.Invoice) ([]byte, error) {
	var doc bytes.Buffer
	doc.WriteString("%PDF-1.7\n")
	doc.Write(r.Font)
	pages := 1 + len(invoices)/25
	for p := 0; p < pages; p++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var page bytes.Buffer
		fmt.Fprintf(&page, "BT /F1 11 Tf 56 790 Td (%s - statement page %d) Tj ET\n", acct.LegalName, p+1)
		for i := p * 25; i < len(invoices) && i < (p+1)*25; i++ {
			inv := invoices[i]
			fmt.Fprintf(&page, "BT 56 %d Td (%s  %s  %d.%02d %s) Tj ET\n", 760-(i%25)*28, inv.Number,
				inv.IssuedAt.Format("2006-01-02"), inv.TotalCents/100, inv.TotalCents%100, acct.Currency)
		}
		var z bytes.Buffer
		fw, err := flate.NewWriter(&z, flate.BestCompression)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(page.Bytes()); err != nil {
			return nil, err
		}
		if err := fw.Close(); err != nil {
			return nil, err
		}
		fmt.Fprintf(&doc, "%d 0 obj << /Length %d /Filter /FlateDecode >> stream\n", p+4, z.Len())
		doc.Write(z.Bytes())
		doc.WriteString("\nendstream endobj\n")
	}
	doc.WriteString("%%EOF\n")
	return doc.Bytes(), nil
}
