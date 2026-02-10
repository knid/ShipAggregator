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
package all

import (
	// Import all provider adapter packages to register them with the global factory.
	// Each adapter package has an init() function that calls provider.RegisterAdapter().

	_ "github.com/knid/kargonet/internal/provider/aras"    // Aras Kargo (SOAP)
	_ "github.com/knid/kargonet/internal/provider/yurtici" // Yurtiçi Kargo (SOAP)
)
