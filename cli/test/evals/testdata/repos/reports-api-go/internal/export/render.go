package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// render encodes rows in format and returns the bytes and content type.
func render(cols []string, rows []Row, format string) ([]byte, string, error) {
	switch format {
	case "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if err := w.Write(cols); err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			if err := w.Write(r.Values); err != nil {
				return nil, "", err
			}
		}
		w.Flush()
		return buf.Bytes(), "text/csv", w.Error()
	case "json":
		out := make([]map[string]string, 0, len(rows))
		for _, r := range rows {
			m := make(map[string]string, len(cols))
			for i, c := range cols {
				if i < len(r.Values) {
					m[c] = r.Values[i]
				}
			}
			out = append(out, m)
		}
		b, err := json.Marshal(out)
		return b, "application/json", err
	case "xlsx":
		b, err := renderXLSX(cols, rows)
		return b, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	default:
		return nil, "", fmt.Errorf("unknown format %q", format)
	}
}

type xlsxCell struct {
	Type string `xml:"t,attr"`
	V    string `xml:"is>t"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxSheet struct {
	XMLName xml.Name  `xml:"worksheet"`
	NS      string    `xml:"xmlns,attr"`
	Rows    []xlsxRow `xml:"sheetData>row"`
}

// renderXLSX writes a single-sheet workbook with inline strings.
func renderXLSX(cols []string, rows []Row) ([]byte, error) {
	sheet := xlsxSheet{NS: "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
	add := func(values []string) {
		var r xlsxRow
		for _, v := range values {
			r.Cells = append(r.Cells, xlsxCell{Type: "inlineStr", V: v})
		}
		sheet.Rows = append(sheet.Rows, r)
	}
	add(cols)
	for _, r := range rows {
		add(r.Values)
	}
	body, err := xml.Marshal(sheet)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"[Content_Types].xml":        contentTypesXML,
		"_rels/.rels":                relsXML,
		"xl/workbook.xml":            workbookXML,
		"xl/_rels/workbook.xml.rels": workbookRelsXML,
		"xl/worksheets/sheet1.xml":   xml.Header + string(body),
	} {
		f, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const (
	contentTypesXML = `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`
	relsXML         = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	workbookXML     = `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Report" sheetId="1" r:id="rId1"/></sheets></workbook>`
	workbookRelsXML = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`
)
