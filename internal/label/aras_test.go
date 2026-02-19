package label

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGenerateArasLabel_Standard(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test Gönderici",
		ReceiverName:       "Test Alıcı",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}
	// Verify PDF header
	if string(pdfBytes[:4]) != "%PDF" {
		t.Fatal("output is not a valid PDF")
	}

	// Write to /tmp for visual inspection
	if err := os.WriteFile("/tmp/aras_label_standard.pdf", pdfBytes, 0644); err != nil {
		t.Logf("could not write test PDF: %v", err)
	} else {
		t.Log("Standard label written to /tmp/aras_label_standard.pdf")
	}
}

func TestGenerateArasLabel_COD(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test Gönderici",
		ReceiverName:       "Test Alıcı",
		ServiceType:        "Tahsilatlı Kargo / Nakit",
		CODAmountTL:        "500",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}

	if err := os.WriteFile("/tmp/aras_label_cod.pdf", pdfBytes, 0644); err != nil {
		t.Logf("could not write test PDF: %v", err)
	} else {
		t.Log("COD label written to /tmp/aras_label_cod.pdf")
	}
}

func TestGenerateArasLabel_MultiPiece(t *testing.T) {
	for i := 1; i <= 2; i++ {
		params := ArasLabelParams{
			SenderName:         "Test Gönderici",
			ReceiverName:       "Test Alıcı",
			ServiceType:        "Standart Kargo",
			IntegrationCode:    "2091601829",
			PieceBarcodeNumber: fmt.Sprintf("20916018290%d", i),
			PieceIndex:         i,
			PieceTotal:         2,
			Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		pdfBytes, err := GenerateArasLabel(params)
		if err != nil {
			t.Fatalf("GenerateArasLabel piece %d failed: %v", i, err)
		}
		if len(pdfBytes) == 0 {
			t.Fatalf("GenerateArasLabel piece %d returned empty bytes", i)
		}

		path := fmt.Sprintf("/tmp/aras_label_multi_%d.pdf", i)
		if err := os.WriteFile(path, pdfBytes, 0644); err != nil {
			t.Logf("could not write test PDF: %v", err)
		} else {
			t.Logf("Multi-piece label %d written to %s", i, path)
		}
	}
}

func TestGenerateArasLabel_EmptyIntegrationCode(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test",
		ReceiverName:       "Test",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := GenerateArasLabel(params)
	if err == nil {
		t.Fatal("expected error for empty integration code")
	}
	if !strings.Contains(err.Error(), "integration code is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGenerateArasLabel_EmptyPieceBarcode(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test",
		ReceiverName:       "Test",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := GenerateArasLabel(params)
	if err == nil {
		t.Fatal("expected error for empty piece barcode number")
	}
	if !strings.Contains(err.Error(), "piece barcode number is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGenerateArasLabel_ZeroPieceDefaults(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test",
		ReceiverName:       "Test",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         0,
		PieceTotal:         0,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}
}

func TestGenerateArasLabel_ZeroDate(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Test",
		ReceiverName:       "Test",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		// Date left as zero value
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}
}

func TestGenerateArasLabel_LongNames(t *testing.T) {
	longName := strings.Repeat("Abcçdefgğ ", 10) // ~100 chars with Turkish chars
	params := ArasLabelParams{
		SenderName:         longName,
		ReceiverName:       longName,
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}

	if err := os.WriteFile("/tmp/aras_label_long_names.pdf", pdfBytes, 0644); err != nil {
		t.Logf("could not write test PDF: %v", err)
	} else {
		t.Log("Long names label written to /tmp/aras_label_long_names.pdf")
	}
}

func TestGenerateArasLabel_TurkishChars(t *testing.T) {
	params := ArasLabelParams{
		SenderName:         "Şükrü Çeşmecioğlu",
		ReceiverName:       "İbrahim Güneş Öztürk",
		ServiceType:        "Standart Kargo",
		IntegrationCode:    "2091601829",
		PieceBarcodeNumber: "209160182901",
		PieceIndex:         1,
		PieceTotal:         1,
		Date:               time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	pdfBytes, err := GenerateArasLabel(params)
	if err != nil {
		t.Fatalf("GenerateArasLabel failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("GenerateArasLabel returned empty bytes")
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long string", 10, "this is a ..."},
		{"", 5, ""},
		{"Güneş Öztürk", 5, "Güneş..."},
	}
	for _, tt := range tests {
		got := truncateText(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateText(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
