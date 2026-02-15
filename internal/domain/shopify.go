package domain

// ShopifyStoreStatus represents the status of a connected Shopify store.
type ShopifyStoreStatus string

const (
	ShopifyStoreStatusActive      ShopifyStoreStatus = "active"
	ShopifyStoreStatusPaused      ShopifyStoreStatus = "paused"
	ShopifyStoreStatusUninstalled ShopifyStoreStatus = "uninstalled"
)

// IsValid checks if the store status is valid.
func (s ShopifyStoreStatus) IsValid() bool {
	switch s {
	case ShopifyStoreStatusActive, ShopifyStoreStatusPaused, ShopifyStoreStatusUninstalled:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (s ShopifyStoreStatus) String() string {
	return string(s)
}

// ShopifySyncMode represents the sync mode for a Shopify store.
type ShopifySyncMode string

const (
	ShopifySyncModeManual ShopifySyncMode = "manual"
	ShopifySyncModeAuto   ShopifySyncMode = "auto"
)

// IsValid checks if the sync mode is valid.
func (s ShopifySyncMode) IsValid() bool {
	switch s {
	case ShopifySyncModeManual, ShopifySyncModeAuto:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (s ShopifySyncMode) String() string {
	return string(s)
}

// ShopifyOrderMappingStatus represents the status of a Shopify order mapping.
type ShopifyOrderMappingStatus string

const (
	ShopifyOrderStatusPending          ShopifyOrderMappingStatus = "pending"
	ShopifyOrderStatusProcessing       ShopifyOrderMappingStatus = "processing"
	ShopifyOrderStatusDraftCreated     ShopifyOrderMappingStatus = "draft_created"
	ShopifyOrderStatusLabelPurchased   ShopifyOrderMappingStatus = "label_purchased"
	ShopifyOrderStatusFulfillmentSynced ShopifyOrderMappingStatus = "fulfillment_synced"
	ShopifyOrderStatusCancelled        ShopifyOrderMappingStatus = "cancelled"
	ShopifyOrderStatusFailed           ShopifyOrderMappingStatus = "failed"
	ShopifyOrderStatusSkipped          ShopifyOrderMappingStatus = "skipped"
)

// IsValid checks if the order mapping status is valid.
func (s ShopifyOrderMappingStatus) IsValid() bool {
	switch s {
	case ShopifyOrderStatusPending, ShopifyOrderStatusProcessing, ShopifyOrderStatusDraftCreated,
		ShopifyOrderStatusLabelPurchased, ShopifyOrderStatusFulfillmentSynced,
		ShopifyOrderStatusCancelled, ShopifyOrderStatusFailed, ShopifyOrderStatusSkipped:
		return true
	default:
		return false
	}
}

// String returns the string representation.
func (s ShopifyOrderMappingStatus) String() string {
	return string(s)
}

// IsFinal returns true if the status is a terminal state.
func (s ShopifyOrderMappingStatus) IsFinal() bool {
	switch s {
	case ShopifyOrderStatusFulfillmentSynced, ShopifyOrderStatusCancelled, ShopifyOrderStatusSkipped:
		return true
	default:
		return false
	}
}
