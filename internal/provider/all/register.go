// Package all imports all provider adapter packages to register them with the global factory.
//
// This package should be imported with a blank identifier in the main application
// to ensure all provider adapters are registered:
//
//	import _ "github.com/knid/kargonet/internal/provider/all"
//
// After importing this package, all implemented provider adapters will be available
// through the provider.GlobalFactory() or provider.CreateAdapter() functions.
//
// Currently registered providers:
//   - aras: Aras Kargo (SOAP)
//   - yurtici: Yurtiçi Kargo (SOAP)
//   - surat: Sürat Kargo (SOAP)
//   - ptt: PTT Kargo (SOAP)
//   - hepsijet: Hepsijet (REST)
//
// Not yet implemented:
//   - mng: MNG Kargo (SOAP)
//   - kolaygelsin: Kolaygelsin (REST)
//   - sendeo: Sendeo (REST)
//   - pakettaxi: Pakettaxi (REST)
package all

import (
	// Import all provider adapter packages to register them with the global factory.
	// Each adapter package has an init() function that calls provider.RegisterAdapter().

	// SOAP-based providers
	_ "github.com/knid/kargonet/internal/provider/aras"     // Aras Kargo
	_ "github.com/knid/kargonet/internal/provider/hepsijet" // Hepsijet (REST)
	_ "github.com/knid/kargonet/internal/provider/ptt"      // PTT Kargo
	_ "github.com/knid/kargonet/internal/provider/surat"    // Sürat Kargo
	_ "github.com/knid/kargonet/internal/provider/yurtici"  // Yurtiçi Kargo
	// The following providers are not yet implemented:
	// _ "github.com/knid/kargonet/internal/provider/mng"        // MNG Kargo (SOAP)
	// _ "github.com/knid/kargonet/internal/provider/kolaygelsin" // Kolaygelsin (REST)
	// _ "github.com/knid/kargonet/internal/provider/sendeo"     // Sendeo (REST)
	// _ "github.com/knid/kargonet/internal/provider/pakettaxi"  // Pakettaxi (REST)
)
