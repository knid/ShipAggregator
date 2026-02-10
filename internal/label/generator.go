// Package label provides convenience label generation for shipping providers
// that do not return labels via their API.
package label

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
)

// ConvenienceLabelParams holds the data needed to generate a convenience label.
type ConvenienceLabelParams struct {
	ProviderName     string
	TrackingNumber   string // cargoKey - will be encoded as barcode
	SenderName       string
	SenderAddress    string
	SenderCity       string
	SenderDistrict   string
	SenderPhone      string
	ReceiverName     string
	ReceiverAddress  string
	ReceiverCity     string
	ReceiverDistrict string
	ReceiverPhone    string
	WeightGrams      int
	DesiValue        float64
	CreatedAt        time.Time
}

// pageWidth is the A6 label width in millimeters.
const pageWidth = 100.0

// pageHeight is the A6 label height in millimeters.
const pageHeight = 150.0

// margin is the page margin in millimeters.
const margin = 5.0

// contentWidth is the usable content width within margins.
const contentWidth = pageWidth - 2*margin

// GenerateConvenienceLabel generates a convenience label PDF for providers
// that do not return labels via their API. The label is A6 size (100x150mm)
// suitable for standard thermal label printers.
func GenerateConvenienceLabel(params ConvenienceLabelParams) ([]byte, error) {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "",
		Size:           fpdf.SizeType{Wd: pageWidth, Ht: pageHeight},
	})

	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	y := margin

	// 1. Provider name header
	y = drawProviderHeader(pdf, params.ProviderName, y)

	// 2. Horizontal separator
	y = drawSeparator(pdf, y)

	// 3. Barcode
	y, err := drawBarcode(pdf, params.TrackingNumber, y)
	if err != nil {
		return nil, fmt.Errorf("failed to draw barcode: %w", err)
	}

	// 4. Tracking number text below barcode
	y = drawTrackingText(pdf, params.TrackingNumber, y)

	// 5. Separator
	y = drawSeparator(pdf, y)

	// 6. Sender and Receiver address blocks in two columns
	y = drawAddressBlocks(pdf, params, y)

	// 7. Separator
	y = drawSeparator(pdf, y)

	// 8. Package info line
	y = drawPackageInfo(pdf, params, y)

	// 9. Footer disclaimer
	drawFooter(pdf, y)

	// Output to bytes
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	if pdf.Err() {
		return nil, fmt.Errorf("PDF generation error: %w", pdf.Error())
	}

	return buf.Bytes(), nil
}

// drawProviderHeader draws the provider name centered and bold at the top.
func drawProviderHeader(pdf *fpdf.Fpdf, providerName string, y float64) float64 {
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(margin, y)
	pdf.CellFormat(contentWidth, 8, strings.ToUpper(providerName), "", 0, "C", false, 0, "")
	return y + 9
}

// drawSeparator draws a horizontal line separator.
func drawSeparator(pdf *fpdf.Fpdf, y float64) float64 {
	y += 1.5
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, y, pageWidth-margin, y)
	return y + 2.5
}

// drawBarcode generates a Code 128 barcode image and embeds it in the PDF.
func drawBarcode(pdf *fpdf.Fpdf, trackingNumber string, y float64) (float64, error) {
	if trackingNumber == "" {
		return y, nil
	}

	// Generate Code 128 barcode
	bc, err := code128.Encode(trackingNumber)
	if err != nil {
		return y, fmt.Errorf("failed to encode barcode: %w", err)
	}

	// Scale barcode to desired pixel dimensions for good quality
	// 60mm wide at ~4 pixels per mm = 240 pixels wide, height ~60 pixels
	barcodeWidth := 240
	barcodeHeight := 60
	scaledBC, err := barcode.Scale(bc, barcodeWidth, barcodeHeight)
	if err != nil {
		return y, fmt.Errorf("failed to scale barcode: %w", err)
	}

	// Encode barcode to PNG in memory
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, scaledBC); err != nil {
		return y, fmt.Errorf("failed to encode barcode PNG: %w", err)
	}

	// Register and place the image in the PDF
	imgName := fmt.Sprintf("barcode_%s", trackingNumber)
	reader := bytes.NewReader(imgBuf.Bytes())
	opts := fpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   true,
	}
	pdf.RegisterImageOptionsReader(imgName, opts, reader)

	// Center the barcode: 60mm wide, ~15mm tall
	barcodeWidthMM := 60.0
	barcodeHeightMM := 15.0
	x := margin + (contentWidth-barcodeWidthMM)/2
	pdf.ImageOptions(imgName, x, y+1, barcodeWidthMM, barcodeHeightMM, false, opts, 0, "")

	return y + barcodeHeightMM + 2, nil
}

// drawTrackingText draws the tracking number text below the barcode in monospace style.
func drawTrackingText(pdf *fpdf.Fpdf, trackingNumber string, y float64) float64 {
	pdf.SetFont("Courier", "B", 10)
	pdf.SetXY(margin, y)
	pdf.CellFormat(contentWidth, 5, trackingNumber, "", 0, "C", false, 0, "")
	return y + 6
}

// drawAddressBlocks draws sender and receiver address blocks side by side.
func drawAddressBlocks(pdf *fpdf.Fpdf, params ConvenienceLabelParams, y float64) float64 {
	colWidth := contentWidth / 2
	leftX := margin
	rightX := margin + colWidth

	startY := y

	// Column headers
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(colWidth, 4, "GONDEREN / SENDER", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, y)
	pdf.CellFormat(colWidth, 4, "ALICI / RECEIVER", "", 0, "L", false, 0, "")
	y += 5

	// Draw vertical divider
	pdf.SetDrawColor(180, 180, 180)
	pdf.SetLineWidth(0.2)

	// Sender name
	leftY := y
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetXY(leftX, leftY)
	leftY += writeMultiCellColumn(pdf, leftX, leftY, colWidth-1, params.SenderName, "Helvetica", "B", 8)

	// Sender address
	leftY += writeMultiCellColumn(pdf, leftX, leftY, colWidth-1, params.SenderAddress, "Helvetica", "", 7)

	// Sender city/district
	cityDistrict := params.SenderDistrict + " / " + params.SenderCity
	leftY += writeMultiCellColumn(pdf, leftX, leftY, colWidth-1, cityDistrict, "Helvetica", "", 7)

	// Sender phone
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetXY(leftX, leftY)
	pdf.CellFormat(colWidth-1, 3.5, "Tel: "+params.SenderPhone, "", 0, "L", false, 0, "")
	leftY += 4

	// Receiver name
	rightY := y
	rightY += writeMultiCellColumn(pdf, rightX, rightY, colWidth-1, params.ReceiverName, "Helvetica", "B", 8)

	// Receiver address
	rightY += writeMultiCellColumn(pdf, rightX, rightY, colWidth-1, params.ReceiverAddress, "Helvetica", "", 7)

	// Receiver city/district
	cityDistrict = params.ReceiverDistrict + " / " + params.ReceiverCity
	rightY += writeMultiCellColumn(pdf, rightX, rightY, colWidth-1, cityDistrict, "Helvetica", "", 7)

	// Receiver phone
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetXY(rightX, rightY)
	pdf.CellFormat(colWidth-1, 3.5, "Tel: "+params.ReceiverPhone, "", 0, "L", false, 0, "")
	rightY += 4

	// Use the taller column's bottom as the next y
	maxY := leftY
	if rightY > maxY {
		maxY = rightY
	}

	// Draw vertical separator line between columns
	pdf.Line(rightX-0.5, startY, rightX-0.5, maxY)

	return maxY + 1
}

// writeMultiCellColumn writes a multi-cell text block at a specific position
// and returns the height consumed.
func writeMultiCellColumn(pdf *fpdf.Fpdf, x, y, width float64, text, fontFamily, fontStyle string, fontSize float64) float64 {
	pdf.SetFont(fontFamily, fontStyle, fontSize)
	pdf.SetXY(x, y)

	// Calculate line height based on font size
	lineHeight := fontSize * 0.45
	if lineHeight < 3.5 {
		lineHeight = 3.5
	}

	// Use MultiCell for automatic text wrapping
	beforeY := pdf.GetY()
	pdf.MultiCell(width, lineHeight, text, "", "L", false)
	afterY := pdf.GetY()

	return afterY - beforeY
}

// drawPackageInfo draws the package information line (weight, dimensions, date).
func drawPackageInfo(pdf *fpdf.Fpdf, params ConvenienceLabelParams, y float64) float64 {
	pdf.SetFont("Helvetica", "", 7)

	// Weight
	weightStr := formatWeight(params.WeightGrams)

	// Desi
	desiStr := fmt.Sprintf("%.1f desi", params.DesiValue)

	// Date
	dateStr := params.CreatedAt.Format("02.01.2006 15:04")

	infoLine := fmt.Sprintf("Agirlik: %s  |  %s  |  Tarih: %s", weightStr, desiStr, dateStr)

	pdf.SetXY(margin, y)
	pdf.CellFormat(contentWidth, 4, infoLine, "", 0, "C", false, 0, "")

	return y + 5
}

// drawFooter draws the disclaimer footer text.
func drawFooter(pdf *fpdf.Fpdf, y float64) {
	pdf.SetFont("Helvetica", "I", 6)
	pdf.SetTextColor(100, 100, 100)
	pdf.SetXY(margin, y+1)
	pdf.MultiCell(contentWidth, 3, "Bu etiket kolaylik amaclidir. Resmi etiket subede basilacaktir.\nThis label is for convenience only. Official label will be printed at the branch.", "", "C", false)
	pdf.SetTextColor(0, 0, 0)
}

// formatWeight formats weight in grams to a human-readable string.
func formatWeight(grams int) string {
	if grams >= 1000 {
		kg := float64(grams) / 1000.0
		return fmt.Sprintf("%.1f kg", kg)
	}
	return fmt.Sprintf("%d g", grams)
}
