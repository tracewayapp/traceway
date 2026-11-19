// Package pdf renders account statements.
package pdf

import (
	"bytes"
	"compress/flate"
	"context"
	"fmt"

	"example.com/statements-api/internal/statements"
	"example.com/statements-api/internal/telemetry"
)

// Renderer lays out a statement and compresses its page streams. The font is
// embedded so the PDF prints identically everywhere.
type Renderer struct {
	Font []byte
}

// NewRenderer returns a renderer with the embedded statement font.
func NewRenderer(font []byte) *Renderer { return &Renderer{Font: font} }

// Render returns the PDF bytes for the account's invoices: a summary page,
// then the invoices, 25 to a page.
func (r *Renderer) Render(ctx context.Context, acct *statements.Account, invoices []statements.Invoice) ([]byte, error) {
	invoicePages := 1 + len(invoices)/25
	ctx, end := telemetry.Start(ctx, "render statement pdf", "statement.pages", 1+invoicePages, "statement.invoices", len(invoices))
	defer end()
	var doc bytes.Buffer
	doc.WriteString("%PDF-1.7\n")
	doc.Write(r.Font)
	var total int64
	for _, inv := range invoices {
		total += inv.TotalCents
	}
	summary := fmt.Sprintf("BT /F1 14 Tf 56 790 Td (%s - account statement) Tj ET\nBT /F1 11 Tf 56 760 Td (%d invoices, %d.%02d %s) Tj ET\n",
		acct.LegalName, len(invoices), total/100, total%100, acct.Currency)
	if err := writePage(&doc, 4, []byte(summary)); err != nil {
		return nil, err
	}
	for p := 0; p < invoicePages; p++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var page bytes.Buffer
		fmt.Fprintf(&page, "BT /F1 11 Tf 56 790 Td (%s - statement page %d) Tj ET\n", acct.LegalName, p+2)
		for i := p * 25; i < len(invoices) && i < (p+1)*25; i++ {
			inv := invoices[i]
			fmt.Fprintf(&page, "BT 56 %d Td (%s  %s  %d.%02d %s) Tj ET\n", 760-(i%25)*28, inv.Number,
				inv.IssuedAt.Format("2006-01-02"), inv.TotalCents/100, inv.TotalCents%100, acct.Currency)
		}
		if err := writePage(&doc, p+5, page.Bytes()); err != nil {
			return nil, err
		}
	}
	doc.WriteString("%%EOF\n")
	return doc.Bytes(), nil
}

// writePage compresses one page's content stream into doc as object obj.
func writePage(doc *bytes.Buffer, obj int, content []byte) error {
	var z bytes.Buffer
	fw, err := flate.NewWriter(&z, flate.BestCompression)
	if err != nil {
		return err
	}
	if _, err := fw.Write(content); err != nil {
		return err
	}
	if err := fw.Close(); err != nil {
		return err
	}
	fmt.Fprintf(doc, "%d 0 obj << /Length %d /Filter /FlateDecode >> stream\n", obj, z.Len())
	doc.Write(z.Bytes())
	doc.WriteString("\nendstream endobj\n")
	return nil
}
