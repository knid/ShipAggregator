package provider

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/knid/kargonet/internal/domain"
)

// AdapterFactory creates and manages provider adapters.
// It provides a registry for adapter constructors and creates
// adapter instances on demand.
type AdapterFactory struct {
	mu           sync.RWMutex
	constructors map[domain.ProviderCode]AdapterConstructor
	configs      map[domain.ProviderCode]AdapterConfig
	logger       *slog.Logger
}

// AdapterConstructor is a function that creates a new adapter instance.
type AdapterConstructor func(config AdapterConfig, logger *slog.Logger) (Adapter, error)

// NewAdapterFactory creates a new AdapterFactory.
func NewAdapterFactory(logger *slog.Logger) *AdapterFactory {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdapterFactory{
		constructors: make(map[domain.ProviderCode]AdapterConstructor),
		configs:      make(map[domain.ProviderCode]AdapterConfig),
		logger:       logger,
	}
}

// Register registers an adapter constructor for a provider.
// This should be called during application initialization.
func (f *AdapterFactory) Register(providerCode domain.ProviderCode, constructor AdapterConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.constructors[providerCode] = constructor
	f.logger.Info("registered provider adapter",
		slog.String("provider", string(providerCode)),
	)
}

// SetConfig sets the configuration for a provider.
func (f *AdapterFactory) SetConfig(providerCode domain.ProviderCode, config AdapterConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs[providerCode] = config
}

// GetConfig returns the configuration for a provider.
func (f *AdapterFactory) GetConfig(providerCode domain.ProviderCode) (AdapterConfig, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	config, ok := f.configs[providerCode]
	return config, ok
}

// Create creates a new adapter instance for the given provider.
// Returns an error if the provider is not registered.
func (f *AdapterFactory) Create(providerCode domain.ProviderCode) (Adapter, error) {
	f.mu.RLock()
	constructor, ok := f.constructors[providerCode]
	config := f.configs[providerCode]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerCode)
	}

	// Use default config if not set
	if config.BaseURL == "" {
		config = DefaultAdapterConfig()
	}

	adapter, err := constructor(config, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter for %s: %w", providerCode, err)
	}

	return adapter, nil
}

// CreateWithConfig creates a new adapter instance with a custom configuration.
func (f *AdapterFactory) CreateWithConfig(providerCode domain.ProviderCode, config AdapterConfig) (Adapter, error) {
	f.mu.RLock()
	constructor, ok := f.constructors[providerCode]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerCode)
	}

	adapter, err := constructor(config, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter for %s: %w", providerCode, err)
	}

	return adapter, nil
}

// IsRegistered checks if a provider is registered.
func (f *AdapterFactory) IsRegistered(providerCode domain.ProviderCode) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.constructors[providerCode]
	return ok
}

// RegisteredProviders returns a list of all registered provider codes.
func (f *AdapterFactory) RegisteredProviders() []domain.ProviderCode {
	f.mu.RLock()
	defer f.mu.RUnlock()

	providers := make([]domain.ProviderCode, 0, len(f.constructors))
	for code := range f.constructors {
		providers = append(providers, code)
	}
	return providers
}

// AdapterPool manages a pool of adapter instances.
// It caches adapter instances for reuse and provides thread-safe access.
type AdapterPool struct {
	factory  *AdapterFactory
	mu       sync.RWMutex
	adapters map[domain.ProviderCode]Adapter
	logger   *slog.Logger
}

// NewAdapterPool creates a new AdapterPool.
func NewAdapterPool(factory *AdapterFactory, logger *slog.Logger) *AdapterPool {
	if logger == nil {
		logger = slog.Default()
	}
	return &AdapterPool{
		factory:  factory,
		adapters: make(map[domain.ProviderCode]Adapter),
		logger:   logger,
	}
}

// Get returns an adapter for the given provider.
// If the adapter is not cached, it creates a new one.
func (p *AdapterPool) Get(providerCode domain.ProviderCode) (Adapter, error) {
	// Try to get from cache first
	p.mu.RLock()
	adapter, ok := p.adapters[providerCode]
	p.mu.RUnlock()

	if ok {
		return adapter, nil
	}

	// Create new adapter
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if adapter, ok = p.adapters[providerCode]; ok {
		return adapter, nil
	}

	adapter, err := p.factory.Create(providerCode)
	if err != nil {
		return nil, err
	}

	p.adapters[providerCode] = adapter
	p.logger.Info("created and cached provider adapter",
		slog.String("provider", string(providerCode)),
	)

	return adapter, nil
}

// GetWithConfig returns an adapter with a custom configuration.
// This does not cache the adapter since it uses a custom config.
func (p *AdapterPool) GetWithConfig(providerCode domain.ProviderCode, config AdapterConfig) (Adapter, error) {
	return p.factory.CreateWithConfig(providerCode, config)
}

// Clear removes all cached adapters.
func (p *AdapterPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.adapters = make(map[domain.ProviderCode]Adapter)
	p.logger.Info("cleared adapter pool")
}

// Remove removes a specific adapter from the cache.
func (p *AdapterPool) Remove(providerCode domain.ProviderCode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.adapters, providerCode)
}

// CachedProviders returns a list of cached provider codes.
func (p *AdapterPool) CachedProviders() []domain.ProviderCode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	providers := make([]domain.ProviderCode, 0, len(p.adapters))
	for code := range p.adapters {
		providers = append(providers, code)
	}
	return providers
}

// Global factory instance for convenience.
var (
	globalFactory     *AdapterFactory
	globalFactoryOnce sync.Once
)

// GlobalFactory returns the global adapter factory instance.
// It initializes the factory on first call.
func GlobalFactory() *AdapterFactory {
	globalFactoryOnce.Do(func() {
		globalFactory = NewAdapterFactory(slog.Default())
	})
	return globalFactory
}

// RegisterAdapter registers an adapter constructor with the global factory.
// This is a convenience function for use during package initialization.
func RegisterAdapter(providerCode domain.ProviderCode, constructor AdapterConstructor) {
	GlobalFactory().Register(providerCode, constructor)
}

// SetAdapterConfig sets the configuration for a provider in the global factory.
func SetAdapterConfig(providerCode domain.ProviderCode, config AdapterConfig) {
	GlobalFactory().SetConfig(providerCode, config)
}

// CreateAdapter creates an adapter using the global factory.
func CreateAdapter(providerCode domain.ProviderCode) (Adapter, error) {
	return GlobalFactory().Create(providerCode)
}

// ProviderRegistry holds provider metadata and configurations.
// It is used to store provider information loaded from the database.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[domain.ProviderCode]*domain.Provider
	services  map[domain.ProviderCode][]domain.ProviderService
}

// NewProviderRegistry creates a new ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[domain.ProviderCode]*domain.Provider),
		services:  make(map[domain.ProviderCode][]domain.ProviderService),
	}
}

// SetProvider sets the provider metadata.
func (r *ProviderRegistry) SetProvider(provider *domain.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Code] = provider
}

// GetProvider returns the provider metadata.
func (r *ProviderRegistry) GetProvider(code domain.ProviderCode) (*domain.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[code]
	return provider, ok
}

// SetServices sets the services for a provider.
func (r *ProviderRegistry) SetServices(code domain.ProviderCode, services []domain.ProviderService) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[code] = services
}

// GetServices returns the services for a provider.
func (r *ProviderRegistry) GetServices(code domain.ProviderCode) ([]domain.ProviderService, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services, ok := r.services[code]
	return services, ok
}

// GetService returns a specific service for a provider.
func (r *ProviderRegistry) GetService(code domain.ProviderCode, serviceCode string) (*domain.ProviderService, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services, ok := r.services[code]
	if !ok {
		return nil, false
	}
	for i := range services {
		if services[i].ServiceCode == serviceCode {
			return &services[i], true
		}
	}
	return nil, false
}

// AllProviders returns all registered providers.
func (r *ProviderRegistry) AllProviders() []*domain.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*domain.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// ActiveProviders returns all active providers.
func (r *ProviderRegistry) ActiveProviders() []*domain.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*domain.Provider, 0)
	for _, p := range r.providers {
		if p.IsActive {
			providers = append(providers, p)
		}
	}
	return providers
}

// SupportsCapability returns providers that support a specific capability.
func (r *ProviderRegistry) SupportsCapability(capability string) []*domain.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*domain.Provider, 0)
	for _, p := range r.providers {
		if p.IsActive && p.SupportsFeature(capability) {
			providers = append(providers, p)
		}
	}
	return providers
}
