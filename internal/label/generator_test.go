package label

import (
	"testing"
	"time"
)

func TestGenerateConvenienceLabel(t *testing.T) {
	tests := []struct {
		name    string
		params  ConvenienceLabelParams
		wantErr bool
	}{
		{
			name: "standard label with typical Turkish addresses",
			params: ConvenienceLabelParams{
				ProviderName:     "YURTICI KARGO",
				TrackingNumber:   "3340000012345678",
				SenderName:       "Ahmet Yilmaz",
				SenderAddress:    "Ataturk Cad. No:42 D:5 Besiktas Mah.",
				SenderCity:       "ISTANBUL",
				SenderDistrict:   "BESIKTAS",
				SenderPhone:      "+905321234567",
				ReceiverName:     "Mehmet Demir",
				ReceiverAddress:  "Cumhuriyet Blv. No:128/A Alsancak Mah.",
				ReceiverCity:     "IZMIR",
				ReceiverDistrict: "KONAK",
				ReceiverPhone:    "+905559876543",
				WeightGrams:      2500,
				DesiValue:        3.5,
				CreatedAt:        time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name: "label with long address lines",
			params: ConvenienceLabelParams{
				ProviderName:     "YURTICI KARGO",
				TrackingNumber:   "3340000099887766",
				SenderName:       "Fatma Ozturk Elektronik Ticaret Ltd. Sti.",
				SenderAddress:    "Organize Sanayi Bolgesi 4. Cadde No:15 Ivedik OSB Yenimahalle",
				SenderCity:       "ANKARA",
				SenderDistrict:   "YENIMAHALLE",
				SenderPhone:      "+903121234567",
				ReceiverName:     "Ali Kaya",
				ReceiverAddress:  "Sakarya Mah. 456 Sok. No:78 Kat:3 D:12 Muratpasa",
				ReceiverCity:     "ANTALYA",
				ReceiverDistrict: "MURATPASA",
				ReceiverPhone:    "+902421234567",
				WeightGrams:      500,
				DesiValue:        1.2,
				CreatedAt:        time.Date(2025, 3, 20, 9, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name: "label with lightweight package under 1kg",
			params: ConvenienceLabelParams{
				ProviderName:     "YURTICI KARGO",
				TrackingNumber:   "3340000055667788",
				SenderName:       "Zeynep Aydin",
				SenderAddress:    "Kisla Mah. Fevzi Cakmak Cad. No:5",
				SenderCity:       "BURSA",
				SenderDistrict:   "OSMANGAZI",
				SenderPhone:      "+902241234567",
				ReceiverName:     "Can Polat",
				ReceiverAddress:  "Sehitkamil Mah. Gazi Cad. No:33",
				ReceiverCity:     "GAZIANTEP",
				ReceiverDistrict: "SEHITKAMIL",
				ReceiverPhone:    "+903421234567",
				WeightGrams:      350,
				DesiValue:        0.8,
				CreatedAt:        time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty tracking number",
			params: ConvenienceLabelParams{
				ProviderName:     "YURTICI KARGO",
				TrackingNumber:   "",
				SenderName:       "Test Sender",
				SenderAddress:    "Test Address",
				SenderCity:       "ISTANBUL",
				SenderDistrict:   "KADIKOY",
				SenderPhone:      "+905001234567",
				ReceiverName:     "Test Receiver",
				ReceiverAddress:  "Test Address",
				ReceiverCity:     "ANKARA",
				ReceiverDistrict: "CANKAYA",
				ReceiverPhone:    "+905009876543",
				WeightGrams:      1000,
				DesiValue:        1.0,
				CreatedAt:        time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateConvenienceLabel(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateConvenienceLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify non-empty output
			if len(result) == 0 {
				t.Error("GenerateConvenienceLabel() returned empty result")
				return
			}

			// Verify valid PDF header (%PDF)
			if len(result) < 4 {
				t.Error("GenerateConvenienceLabel() result too short to be a valid PDF")
				return
			}
			header := string(result[:4])
			if header != "%PDF" {
				t.Errorf("GenerateConvenienceLabel() result does not start with PDF header, got %q", header)
			}

			// Verify reasonable file size (a label should be between 1KB and 500KB)
			if len(result) < 1024 {
				t.Errorf("GenerateConvenienceLabel() result suspiciously small: %d bytes", len(result))
			}
			if len(result) > 512*1024 {
				t.Errorf("GenerateConvenienceLabel() result suspiciously large: %d bytes", len(result))
			}
		})
	}
}

func TestFormatWeight(t *testing.T) {
	tests := []struct {
		grams    int
		expected string
	}{
		{500, "500 g"},
		{999, "999 g"},
		{1000, "1.0 kg"},
		{1500, "1.5 kg"},
		{2500, "2.5 kg"},
		{10000, "10.0 kg"},
		{25750, "25.8 kg"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatWeight(tt.grams)
			if result != tt.expected {
				t.Errorf("formatWeight(%d) = %q, want %q", tt.grams, result, tt.expected)
			}
		})
	}
}
