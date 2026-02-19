package label

import (
	"bytes"
	"fmt"
	"image/png"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// arasFontFamily is the font family name used for Aras labels.
// Uses Go fonts which support Turkish characters (ı, İ, ş, ğ, etc.).
const arasFontFamily = "GoFont"

// ArasLabelParams holds the data needed to generate an Aras-format shipping label.
type ArasLabelParams struct {
	SenderName         string
	ReceiverName       string
	ServiceType        string // "Standart Kargo" or "Tahsilatlı Kargo / Nakit" etc.
	CODAmountTL        string // e.g. "500" (only used if COD)
	IntegrationCode    string // shown + barcoded
	PieceBarcodeNumber string // shown + barcoded (integrationCode + piece suffix)
	PieceIndex         int    // 1-based
	PieceTotal         int    // total pieces
	Date               time.Time
}

// truncateText truncates a string to maxLen characters and appends "..." if truncated.
func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}

// GenerateArasLabel generates an Aras Kargo format label as a PDF.
// The layout matches the official Aras label examples with sender/receiver
// sections, service type, integration code barcode, and piece barcode.
func GenerateArasLabel(params ArasLabelParams) ([]byte, error) {
	if params.IntegrationCode == "" {
		return nil, fmt.Errorf("integration code is required")
	}
	if params.PieceBarcodeNumber == "" {
		return nil, fmt.Errorf("piece barcode number is required")
	}
	if params.PieceIndex <= 0 {
		params.PieceIndex = 1
	}
	if params.PieceTotal <= 0 {
		params.PieceTotal = 1
	}
	if params.Date.IsZero() {
		params.Date = time.Now()
	}

	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: pageWidth, Ht: pageHeight},
	})

	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)

	// Register UTF-8 fonts for Turkish character support
	pdf.AddUTF8FontFromBytes(arasFontFamily, "", goregular.TTF)
	pdf.AddUTF8FontFromBytes(arasFontFamily, "B", gobold.TTF)

	pdf.AddPage()

	y := margin

	// 1. Sender row — bordered box with name left, date right
	y = drawArasSenderRow(pdf, params.SenderName, params.Date, y)

	// 2. Receiver row — bordered box with receiver info
	y = drawArasReceiverRow(pdf, params.ReceiverName, y)

	// 3. Separator line between receiver and service type
	y = drawSeparator(pdf, y)

	// 4. Service type row
	y = drawArasServiceType(pdf, params.ServiceType, params.CODAmountTL, y)

	// 5. Integration code with barcode
	y = drawArasIntegrationCode(pdf, params.IntegrationCode, y)

	// 6. Piece barcode section with package counter
	drawArasPieceBarcode(pdf, params.PieceBarcodeNumber, params.PieceIndex, params.PieceTotal, y)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate Aras label PDF: %w", err)
	}
	if pdf.Err() {
		return nil, fmt.Errorf("Aras label PDF error: %w", pdf.Error())
	}

	return buf.Bytes(), nil
}

// drawArasSenderRow draws the sender section: bordered box with name and date on a single line.
func drawArasSenderRow(pdf *fpdf.Fpdf, senderName string, date time.Time, y float64) float64 {
	boxH := 8.0
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Rect(margin, y, contentWidth, boxH, "D")

	name := truncateText(senderName, 45)
	label := "Gönderici  :  " + name

	pdf.SetFont(arasFontFamily, "B", 9)
	pdf.SetXY(margin+2, y+2)
	pdf.CellFormat(contentWidth*0.7, 4, label, "", 0, "L", false, 0, "")

	dateStr := date.Format("02.01.2006")
	pdf.SetFont(arasFontFamily, "", 9)
	pdf.SetXY(margin+contentWidth*0.7, y+2)
	pdf.CellFormat(contentWidth*0.3-2, 4, dateStr, "", 0, "R", false, 0, "")

	return y + boxH
}

// drawArasReceiverRow draws the receiver section: bordered box.
func drawArasReceiverRow(pdf *fpdf.Fpdf, receiverName string, y float64) float64 {
	boxH := 14.0
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Rect(margin, y, contentWidth, boxH, "D")

	pdf.SetFont(arasFontFamily, "B", 9)
	pdf.SetXY(margin+2, y+1)
	pdf.CellFormat(contentWidth, 4, "Alıcı Bilgileri", "", 0, "L", false, 0, "")

	name := truncateText(receiverName, 45)
	pdf.SetFont(arasFontFamily, "", 9)
	pdf.SetXY(margin+2, y+5.5)
	nameText := ":"
	if name != "" {
		nameText = ": " + name
	}
	pdf.CellFormat(contentWidth, 4, "İsim  "+nameText, "", 0, "L", false, 0, "")

	return y + boxH
}

// drawArasServiceType draws the service type line (standard or COD).
func drawArasServiceType(pdf *fpdf.Fpdf, serviceType, codAmountTL string, y float64) float64 {
	y += 2 // small gap

	pdf.SetFont(arasFontFamily, "B", 10)
	pdf.SetXY(margin+2, y)

	if codAmountTL != "" {
		// COD label: "Tahsilatlı Kargo / Nakit  Tahsilat Tutarı : 500 TL"
		text := serviceType + "  Tahsilat Tutarı : " + codAmountTL + " TL"
		pdf.CellFormat(contentWidth, 6, text, "", 0, "L", false, 0, "")
	} else {
		pdf.CellFormat(contentWidth, 6, serviceType, "", 0, "L", false, 0, "")
	}

	return y + 8
}

// drawArasIntegrationCode draws the integration code text and barcode.
func drawArasIntegrationCode(pdf *fpdf.Fpdf, integrationCode string, y float64) float64 {
	pdf.SetFont(arasFontFamily, "B", 9)
	pdf.SetXY(margin+2, y)
	pdf.CellFormat(contentWidth, 5, "Entegrasyon No : "+integrationCode, "", 0, "L", false, 0, "")
	y += 5

	endY, err := drawArasBarcode(pdf, integrationCode, "bc_int_"+integrationCode, y)
	if err != nil {
		return y + 2
	}

	return endY + 1
}

// drawArasPieceBarcode draws the piece barcode section with package counter.
func drawArasPieceBarcode(pdf *fpdf.Fpdf, pieceBarcodeNumber string, pieceIndex, pieceTotal int, y float64) {
	boxStartY := y
	boxH := 30.0
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	pdf.Rect(margin, boxStartY, contentWidth, boxH, "D")

	pdf.SetFont(arasFontFamily, "B", 9)
	pdf.SetXY(margin+2, y+1)
	pdf.CellFormat(contentWidth, 5, "Paket Barkod No : "+pieceBarcodeNumber, "", 0, "L", false, 0, "")
	y += 6

	_, _ = drawArasBarcode(pdf, pieceBarcodeNumber, "bc_piece_"+pieceBarcodeNumber, y)

	pdf.SetFont(arasFontFamily, "B", 10)
	counterText := fmt.Sprintf("Paket : %d / %d", pieceIndex, pieceTotal)
	pdf.SetXY(margin+2, boxStartY+boxH-6)
	pdf.CellFormat(contentWidth-4, 5, counterText, "", 0, "R", false, 0, "")
}

// drawArasBarcode generates a Code 128 barcode and embeds it in the PDF.
func drawArasBarcode(pdf *fpdf.Fpdf, text, imgName string, y float64) (float64, error) {
	if text == "" {
		return y, nil
	}

	bc, err := code128.Encode(text)
	if err != nil {
		return y, fmt.Errorf("failed to encode barcode: %w", err)
	}

	scaledBC, err := barcode.Scale(bc, 240, 50)
	if err != nil {
		return y, fmt.Errorf("failed to scale barcode: %w", err)
	}

	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, scaledBC); err != nil {
		return y, fmt.Errorf("failed to encode barcode PNG: %w", err)
	}

	reader := bytes.NewReader(imgBuf.Bytes())
	opts := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	pdf.RegisterImageOptionsReader(imgName, opts, reader)

	barcodeWidthMM := 55.0
	barcodeHeightMM := 13.0
	x := margin + 2
	pdf.ImageOptions(imgName, x, y, barcodeWidthMM, barcodeHeightMM, false, opts, 0, "")

	return y + barcodeHeightMM, nil
}
