package shopify

import (
	"context"
	"strings"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	shopifytypes "github.com/knid/kargonet/internal/shopify"
)

// MappedAddress holds the resolved e-port address fields from a Shopify address.
type MappedAddress struct {
	ContactName      string
	Phone            string
	StreetLine1      string
	StreetLine2      string
	CityCode         string
	CityName         string
	DistrictCode     string
	DistrictName     string
	NeighborhoodCode string
	PostalCode       string
}

// MapShopifyAddress maps a Shopify shipping address to e-port address fields.
// Strategy: postal code lookup first, then city name fuzzy match, then district match.
func MapShopifyAddress(ctx context.Context, queries *db.Queries, addr *shopifytypes.Address, logger *observability.Logger) *MappedAddress {
	if addr == nil {
		return nil
	}

	result := &MappedAddress{
		ContactName: addr.Name,
		Phone:       addr.Phone,
		StreetLine1: addr.Address1,
		StreetLine2: addr.Address2,
		PostalCode:  addr.Zip,
	}

	// Strategy 1: Postal code lookup
	if addr.Zip != "" {
		zip := addr.Zip
		postalResult, err := queries.GetLocationByPostalCode(ctx, &zip)
		if err == nil && postalResult != nil {
			result.CityCode = postalResult.CityCode
			result.CityName = postalResult.CityName
			result.DistrictCode = postalResult.DistrictCode
			result.DistrictName = postalResult.DistrictName
			result.NeighborhoodCode = postalResult.NeighborhoodCode
			return result
		}
	}

	// Strategy 2: Match city by name (province field from Shopify)
	cityName := addr.Province
	if cityName == "" {
		cityName = addr.City
	}
	if cityName != "" {
		normalized := normalizeTurkishCity(cityName)
		cities, err := queries.SearchCitiesByName(ctx, db.SearchCitiesByNameParams{
			Column1:    &normalized,
			LimitCount: 1,
		})
		if err == nil && len(cities) > 0 {
			result.CityCode = cities[0].Code
			result.CityName = cities[0].Name

			// Strategy 3: Match district by name within the matched city
			districtName := addr.City
			if districtName != "" {
				distNormalized := normalizeTurkishCity(districtName)
				districts, err := queries.SearchDistrictsByName(ctx, db.SearchDistrictsByNameParams{
					Column1:    &distNormalized,
					LimitCount: 5,
				})
				if err == nil {
					// Find first district matching our resolved city
					for _, d := range districts {
						if d.CityCode == result.CityCode {
							result.DistrictCode = d.Code
							result.DistrictName = d.Name
							break
						}
					}
				}
			}
		}
	}

	if result.CityCode == "" {
		logger.Warn("could not resolve Shopify address to e-port city",
			"province", addr.Province,
			"city", addr.City,
			"zip", addr.Zip,
		)
	}

	return result
}

// normalizeTurkishCity normalizes Turkish city names for matching.
func normalizeTurkishCity(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToUpper(name)
	replacer := strings.NewReplacer(
		"İ", "I",
		"Ş", "S",
		"Ğ", "G",
		"Ü", "U",
		"Ö", "O",
		"Ç", "C",
	)
	return replacer.Replace(name)
}
