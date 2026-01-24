// Package main provides a command to seed reference data from JSON files.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Province represents a province/city from the JSON file.
type Province struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	PostalCode string `json:"postalCode"`
	Population int    `json:"population"`
}

// District represents a district from the JSON file.
type District struct {
	ID         int    `json:"id"`
	ProvinceID int    `json:"provinceId"`
	Province   string `json:"province"`
	Name       string `json:"name"`
	PostalCode string `json:"postalCode"`
	Population int    `json:"population"`
}

// Neighborhood represents a neighborhood from the JSON file.
type Neighborhood struct {
	ID         int    `json:"id"`
	ProvinceID int    `json:"provinceId"`
	DistrictID int    `json:"districtId"`
	Province   string `json:"province"`
	District   string `json:"district"`
	Name       string `json:"name"`
	Population int    `json:"population"`
}

func main() {
	// Parse command line flags
	dbURL := flag.String("db", "", "Database connection URL (or set DATABASE_URL env var)")
	provincesFile := flag.String("provinces", "data/provinces.json", "Path to provinces JSON file")
	districtsFile := flag.String("districts", "data/districts.json", "Path to districts JSON file")
	neighborhoodsFile := flag.String("neighborhoods", "data/neighborhoods.json", "Path to neighborhoods JSON file")
	truncate := flag.Bool("truncate", false, "Truncate existing data before import")
	flag.Parse()

	// Get database URL
	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		log.Fatal("Database URL is required. Use -db flag or set DATABASE_URL environment variable")
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Truncate if requested
	if *truncate {
		log.Println("Truncating existing reference data...")
		if err := truncateReferenceData(ctx, pool); err != nil {
			log.Fatalf("Failed to truncate data: %v", err)
		}
	}

	// Import provinces (cities)
	log.Printf("Importing provinces from %s...", *provincesFile)
	provinces, err := importProvinces(ctx, pool, *provincesFile)
	if err != nil {
		log.Fatalf("Failed to import provinces: %v", err)
	}
	log.Printf("Imported %d provinces", len(provinces))

	// Import districts
	log.Printf("Importing districts from %s...", *districtsFile)
	districts, err := importDistricts(ctx, pool, *districtsFile, provinces)
	if err != nil {
		log.Fatalf("Failed to import districts: %v", err)
	}
	log.Printf("Imported %d districts", len(districts))

	// Import neighborhoods
	log.Printf("Importing neighborhoods from %s...", *neighborhoodsFile)
	count, err := importNeighborhoods(ctx, pool, *neighborhoodsFile, districts)
	if err != nil {
		log.Fatalf("Failed to import neighborhoods: %v", err)
	}
	log.Printf("Imported %d neighborhoods", count)

	// Generate postal codes from districts
	log.Println("Generating postal codes...")
	postalCount, err := generatePostalCodes(ctx, pool)
	if err != nil {
		log.Fatalf("Failed to generate postal codes: %v", err)
	}
	log.Printf("Generated %d postal codes", postalCount)

	log.Println("Reference data import completed successfully!")
}

func truncateReferenceData(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE ref_postal_codes CASCADE;
		TRUNCATE TABLE ref_neighborhoods CASCADE;
		TRUNCATE TABLE ref_districts CASCADE;
		TRUNCATE TABLE ref_cities CASCADE;
	`)
	return err
}

func importProvinces(ctx context.Context, pool *pgxpool.Pool, filename string) (map[int]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var provinces []Province
	if err := json.Unmarshal(data, &provinces); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Map province ID to code (plate code format: 01, 02, ..., 81)
	provinceMap := make(map[int]string)

	batch := &pgx.Batch{}
	for _, p := range provinces {
		// Use plate code format (01-81)
		code := fmt.Sprintf("%02d", p.ID)
		provinceMap[p.ID] = code

		normalized := normalizeText(p.Name)
		batch.Queue(`
			INSERT INTO ref_cities (code, name, name_normalized, is_active, updated_at)
			VALUES ($1, $2, $3, TRUE, $4)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				name_normalized = EXCLUDED.name_normalized,
				updated_at = EXCLUDED.updated_at
		`, code, p.Name, normalized, time.Now())
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	for range provinces {
		if _, err := br.Exec(); err != nil {
			return nil, fmt.Errorf("failed to insert province: %w", err)
		}
	}

	return provinceMap, nil
}

func importDistricts(ctx context.Context, pool *pgxpool.Pool, filename string, provinceMap map[int]string) (map[int]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var districts []District
	if err := json.Unmarshal(data, &districts); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Map district ID to code
	districtMap := make(map[int]string)

	// Process in batches of 1000
	batchSize := 1000
	for i := 0; i < len(districts); i += batchSize {
		end := i + batchSize
		if end > len(districts) {
			end = len(districts)
		}

		batch := &pgx.Batch{}
		for _, d := range districts[i:end] {
			cityCode, ok := provinceMap[d.ProvinceID]
			if !ok {
				log.Printf("Warning: Unknown province ID %d for district %s", d.ProvinceID, d.Name)
				continue
			}

			// District code format: city_code + district_id
			code := fmt.Sprintf("%s-%d", cityCode, d.ID)
			districtMap[d.ID] = code

			normalized := normalizeText(d.Name)
			batch.Queue(`
				INSERT INTO ref_districts (code, city_code, name, name_normalized, is_active, updated_at)
				VALUES ($1, $2, $3, $4, TRUE, $5)
				ON CONFLICT (code) DO UPDATE SET
					city_code = EXCLUDED.city_code,
					name = EXCLUDED.name,
					name_normalized = EXCLUDED.name_normalized,
					updated_at = EXCLUDED.updated_at
			`, code, cityCode, d.Name, normalized, time.Now())
		}

		br := pool.SendBatch(ctx, batch)
		for j := i; j < end; j++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return nil, fmt.Errorf("failed to insert district: %w", err)
			}
		}
		br.Close()
	}

	return districtMap, nil
}

func importNeighborhoods(ctx context.Context, pool *pgxpool.Pool, filename string, districtMap map[int]string) (int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	var neighborhoods []Neighborhood
	if err := json.Unmarshal(data, &neighborhoods); err != nil {
		return 0, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Process in batches of 1000
	batchSize := 1000
	count := 0
	for i := 0; i < len(neighborhoods); i += batchSize {
		end := i + batchSize
		if end > len(neighborhoods) {
			end = len(neighborhoods)
		}

		batch := &pgx.Batch{}
		batchCount := 0
		for _, n := range neighborhoods[i:end] {
			districtCode, ok := districtMap[n.DistrictID]
			if !ok {
				// Try to find by province and district name
				log.Printf("Warning: Unknown district ID %d for neighborhood %s", n.DistrictID, n.Name)
				continue
			}

			// Neighborhood code format: district_code + neighborhood_id
			code := fmt.Sprintf("%s-%d", districtCode, n.ID)
			normalized := normalizeText(n.Name)

			batch.Queue(`
				INSERT INTO ref_neighborhoods (code, district_code, name, name_normalized, postal_code, is_active, updated_at)
				VALUES ($1, $2, $3, $4, NULL, TRUE, $5)
				ON CONFLICT (code) DO UPDATE SET
					district_code = EXCLUDED.district_code,
					name = EXCLUDED.name,
					name_normalized = EXCLUDED.name_normalized,
					updated_at = EXCLUDED.updated_at
			`, code, districtCode, n.Name, normalized, time.Now())
			batchCount++
		}

		if batchCount > 0 {
			br := pool.SendBatch(ctx, batch)
			for j := 0; j < batchCount; j++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return count, fmt.Errorf("failed to insert neighborhood: %w", err)
				}
				count++
			}
			br.Close()
		}

		// Progress indicator
		if (i+batchSize)%10000 == 0 {
			log.Printf("  Processed %d neighborhoods...", i+batchSize)
		}
	}

	return count, nil
}

func generatePostalCodes(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	// Get unique postal codes from districts
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT d.code, 
			CASE 
				WHEN LENGTH(c.code) = 1 THEN '0' || c.code || '000'
				ELSE c.code || '000'
			END as postal_code
		FROM ref_districts d
		JOIN ref_cities c ON d.city_code = c.code
		WHERE NOT EXISTS (
			SELECT 1 FROM ref_postal_codes p WHERE p.district_code = d.code
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to query districts: %w", err)
	}
	defer rows.Close()

	type postalEntry struct {
		districtCode string
		postalCode   string
	}

	var entries []postalEntry
	for rows.Next() {
		var e postalEntry
		if err := rows.Scan(&e.districtCode, &e.postalCode); err != nil {
			return 0, fmt.Errorf("failed to scan row: %w", err)
		}
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`
			INSERT INTO ref_postal_codes (code, district_code, is_active, updated_at)
			VALUES ($1, $2, TRUE, $3)
			ON CONFLICT (code) DO NOTHING
		`, e.postalCode, e.districtCode, time.Now())
	}

	br := pool.SendBatch(ctx, batch)
	defer br.Close()

	count := 0
	for range entries {
		if _, err := br.Exec(); err != nil {
			// Ignore duplicate key errors
			continue
		}
		count++
	}

	return count, nil
}

// normalizeText converts Turkish characters to ASCII and lowercases the text.
func normalizeText(s string) string {
	// Turkish character mapping
	replacer := strings.NewReplacer(
		"İ", "i",
		"I", "i",
		"ı", "i",
		"Ğ", "g",
		"ğ", "g",
		"Ü", "u",
		"ü", "u",
		"Ş", "s",
		"ş", "s",
		"Ö", "o",
		"ö", "o",
		"Ç", "c",
		"ç", "c",
	)

	// First apply Turkish-specific replacements
	s = replacer.Replace(s)

	// Then normalize using unicode normalization
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)

	return strings.ToLower(result)
}
