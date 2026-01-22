package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ShipmentStatus represents the status of a shipment.
type ShipmentStatus string

const (
	// ShipmentStatusDraft indicates the shipment is created but label not purchased.
	ShipmentStatusDraft ShipmentStatus = "draft"
	// ShipmentStatusLabelPurchased indicates the label has been purchased.
	ShipmentStatusLabelPurchased ShipmentStatus = "label_purchased"
	// ShipmentStatusPickupScheduled indicates pickup has been scheduled.
	ShipmentStatusPickupScheduled ShipmentStatus = "pickup_scheduled"
	// ShipmentStatusPickedUp indicates the shipment has been picked up.
	ShipmentStatusPickedUp ShipmentStatus = "picked_up"
	// ShipmentStatusInTransit indicates the shipment is in transit.
	ShipmentStatusInTransit ShipmentStatus = "in_transit"
	// ShipmentStatusAtHub indicates the shipment is at a hub/sorting facility.
	ShipmentStatusAtHub ShipmentStatus = "at_hub"
	// ShipmentStatusOutForDelivery indicates the shipment is out for delivery.
	ShipmentStatusOutForDelivery ShipmentStatus = "out_for_delivery"
	// ShipmentStatusDelivered indicates the shipment has been delivered.
	ShipmentStatusDelivered ShipmentStatus = "delivered"
	// ShipmentStatusDeliveryFailed indicates delivery failed.
	ShipmentStatusDeliveryFailed ShipmentStatus = "delivery_failed"
	// ShipmentStatusReturning indicates the shipment is being returned.
	ShipmentStatusReturning ShipmentStatus = "returning"
	// ShipmentStatusReturnedToSender indicates the shipment was returned to sender.
	ShipmentStatusReturnedToSender ShipmentStatus = "returned_to_sender"
	// ShipmentStatusCancelled indicates the shipment was cancelled.
	ShipmentStatusCancelled ShipmentStatus = "cancelled"
	// ShipmentStatusException indicates an exception occurred.
	ShipmentStatusException ShipmentStatus = "exception"
)

// IsValid checks if the shipment status is valid.
func (s ShipmentStatus) IsValid() bool {
	switch s {
	case ShipmentStatusDraft, ShipmentStatusLabelPurchased, ShipmentStatusPickupScheduled,
		ShipmentStatusPickedUp, ShipmentStatusInTransit, ShipmentStatusAtHub,
		ShipmentStatusOutForDelivery, ShipmentStatusDelivered, ShipmentStatusDeliveryFailed,
		ShipmentStatusReturning, ShipmentStatusReturnedToSender, ShipmentStatusCancelled,
		ShipmentStatusException:
		return true
	default:
		return false
	}
}

// String returns the string representation of the shipment status.
func (s ShipmentStatus) String() string {
	return string(s)
}

// IsFinal returns true if the status is a final state.
func (s ShipmentStatus) IsFinal() bool {
	switch s {
	case ShipmentStatusDelivered, ShipmentStatusReturnedToSender, ShipmentStatusCancelled:
		return true
	default:
		return false
	}
}

// IsActive returns true if the shipment is in an active state.
func (s ShipmentStatus) IsActive() bool {
	switch s {
	case ShipmentStatusLabelPurchased, ShipmentStatusPickupScheduled, ShipmentStatusPickedUp,
		ShipmentStatusInTransit, ShipmentStatusAtHub, ShipmentStatusOutForDelivery,
		ShipmentStatusDeliveryFailed, ShipmentStatusReturning, ShipmentStatusException:
		return true
	default:
		return false
	}
}

// CanBeCancelled returns true if the shipment can be cancelled.
func (s ShipmentStatus) CanBeCancelled() bool {
	switch s {
	case ShipmentStatusDraft, ShipmentStatusLabelPurchased, ShipmentStatusPickupScheduled:
		return true
	default:
		return false
	}
}

// CanSchedulePickup returns true if pickup can be scheduled.
func (s ShipmentStatus) CanSchedulePickup() bool {
	return s == ShipmentStatusLabelPurchased
}

// CanCreateReturn returns true if a return can be created.
func (s ShipmentStatus) CanCreateReturn() bool {
	return s == ShipmentStatusDelivered
}

// LabelFormat represents the format of a shipping label.
type LabelFormat string

const (
	// LabelFormatPDF is a PDF label format.
	LabelFormatPDF LabelFormat = "pdf"
	// LabelFormatZPL is a ZPL (Zebra) label format.
	LabelFormatZPL LabelFormat = "zpl"
	// LabelFormatPNG is a PNG image label format.
	LabelFormatPNG LabelFormat = "png"
)

// IsValid checks if the label format is valid.
func (f LabelFormat) IsValid() bool {
	switch f {
	case LabelFormatPDF, LabelFormatZPL, LabelFormatPNG:
		return true
	default:
		return false
	}
}

// String returns the string representation of the label format.
func (f LabelFormat) String() string {
	return string(f)
}

// ContentType returns the MIME content type for the label format.
func (f LabelFormat) ContentType() string {
	switch f {
	case LabelFormatPDF:
		return "application/pdf"
	case LabelFormatZPL:
		return "application/x-zpl"
	case LabelFormatPNG:
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// Shipment represents a core shipment record.
type Shipment struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// IdempotencyKey is the client-provided idempotency key.
	IdempotencyKey string `json:"idempotency_key"`
	// OfferID is the source offer ID (if created from quote).
	OfferID *uuid.UUID `json:"offer_id,omitempty"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// ServiceCode is the service code.
	ServiceCode string `json:"service_code"`
	// ProviderAccountID is the BYOA account ID (if applicable).
	ProviderAccountID *uuid.UUID `json:"provider_account_id,omitempty"`
	// IsMarketplace indicates if this is a marketplace shipment.
	IsMarketplace bool `json:"is_marketplace"`
	// ProviderShipmentID is the provider's internal shipment ID.
	ProviderShipmentID *string `json:"provider_shipment_id,omitempty"`
	// ProviderTrackingNumber is the provider's tracking number.
	ProviderTrackingNumber *string `json:"provider_tracking_number,omitempty"`
	// ProviderBarcode is the provider's barcode for the label.
	ProviderBarcode *string `json:"provider_barcode,omitempty"`
	// OriginAddress is the origin address snapshot.
	OriginAddress AddressSnapshot `json:"origin_address"`
	// DestinationAddress is the destination address snapshot.
	DestinationAddress AddressSnapshot `json:"destination_address"`
	// Parcel contains the parcel details.
	Parcel Parcel `json:"parcel"`
	// CODAmountKurus is the COD amount in kuruş.
	CODAmountKurus *int64 `json:"cod_amount_kurus,omitempty"`
	// DeclaredValueKurus is the declared value for insurance in kuruş.
	DeclaredValueKurus *int64 `json:"declared_value_kurus,omitempty"`
	// Status is the shipment status.
	Status ShipmentStatus `json:"status"`
	// LabelDocumentID is the label document ID.
	LabelDocumentID *uuid.UUID `json:"label_document_id,omitempty"`
	// LabelFormat is the label format.
	LabelFormat *LabelFormat `json:"label_format,omitempty"`
	// PriceChargedKurus is the amount charged to wallet in kuruş.
	PriceChargedKurus *int64 `json:"price_charged_kurus,omitempty"`
	// CostKurus is our cost (marketplace only) in kuruş.
	CostKurus *int64 `json:"cost_kurus,omitempty"`
	// PickupScheduledAt is the scheduled pickup time.
	PickupScheduledAt *time.Time `json:"pickup_scheduled_at,omitempty"`
	// PickedUpAt is the actual pickup time.
	PickedUpAt *time.Time `json:"picked_up_at,omitempty"`
	// DeliveredAt is the delivery timestamp.
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	// CancelledAt is the cancellation timestamp.
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	// CancellationReason is the cancellation reason.
	CancellationReason *string `json:"cancellation_reason,omitempty"`
	// ReturnShipmentID is the linked return shipment ID.
	ReturnShipmentID *uuid.UUID `json:"return_shipment_id,omitempty"`
	// OriginalShipmentID is the original shipment ID (for returns).
	OriginalShipmentID *uuid.UUID `json:"original_shipment_id,omitempty"`
	// Metadata contains custom metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is the timestamp when the shipment was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the shipment was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// TrackingEvents contains the tracking history (populated on demand).
	TrackingEvents []TrackingEvent `json:"tracking_events,omitempty"`
}

// NewShipmentFromOffer creates a new Shipment from an Offer.
func NewShipmentFromOffer(
	organizationID uuid.UUID,
	idempotencyKey string,
	offer *Offer,
	origin, destination AddressSnapshot,
	parcel Parcel,
	codAmountKurus, declaredValueKurus *int64,
) *Shipment {
	now := time.Now().UTC()
	return &Shipment{
		ID:                 uuid.New(),
		OrganizationID:     organizationID,
		IdempotencyKey:     idempotencyKey,
		OfferID:            &offer.ID,
		ProviderCode:       offer.ProviderCode,
		ServiceCode:        offer.ServiceCode,
		ProviderAccountID:  offer.ProviderAccountID,
		IsMarketplace:      offer.IsMarketplace,
		OriginAddress:      origin,
		DestinationAddress: destination,
		Parcel:             parcel,
		CODAmountKurus:     codAmountKurus,
		DeclaredValueKurus: declaredValueKurus,
		Status:             ShipmentStatusDraft,
		PriceChargedKurus:  &offer.TotalPriceKurus,
		CostKurus:          offer.CostPriceKurus,
		Metadata:           make(map[string]interface{}),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// IsDraft returns true if the shipment is in draft status.
func (s *Shipment) IsDraft() bool {
	return s.Status == ShipmentStatusDraft
}

// IsLabelPurchased returns true if the label has been purchased.
func (s *Shipment) IsLabelPurchased() bool {
	return s.Status == ShipmentStatusLabelPurchased
}

// IsDelivered returns true if the shipment has been delivered.
func (s *Shipment) IsDelivered() bool {
	return s.Status == ShipmentStatusDelivered
}

// IsCancelled returns true if the shipment has been cancelled.
func (s *Shipment) IsCancelled() bool {
	return s.Status == ShipmentStatusCancelled
}

// IsReturn returns true if this is a return shipment.
func (s *Shipment) IsReturn() bool {
	return s.OriginalShipmentID != nil
}

// HasReturn returns true if this shipment has a return shipment.
func (s *Shipment) HasReturn() bool {
	return s.ReturnShipmentID != nil
}

// HasCOD returns true if the shipment has COD.
func (s *Shipment) HasCOD() bool {
	return s.CODAmountKurus != nil && *s.CODAmountKurus > 0
}

// HasInsurance returns true if the shipment has insurance.
func (s *Shipment) HasInsurance() bool {
	return s.DeclaredValueKurus != nil && *s.DeclaredValueKurus > 0
}

// HasLabel returns true if the shipment has a label.
func (s *Shipment) HasLabel() bool {
	return s.LabelDocumentID != nil
}

// HasTrackingNumber returns true if the shipment has a tracking number.
func (s *Shipment) HasTrackingNumber() bool {
	return s.ProviderTrackingNumber != nil && *s.ProviderTrackingNumber != ""
}

// CanBeCancelled returns true if the shipment can be cancelled.
func (s *Shipment) CanBeCancelled() bool {
	return s.Status.CanBeCancelled()
}

// CanSchedulePickup returns true if pickup can be scheduled.
func (s *Shipment) CanSchedulePickup() bool {
	return s.Status.CanSchedulePickup()
}

// CanCreateReturn returns true if a return can be created.
func (s *Shipment) CanCreateReturn() bool {
	return s.Status.CanCreateReturn() && !s.HasReturn()
}

// CanPurchaseLabel returns true if a label can be purchased.
func (s *Shipment) CanPurchaseLabel() bool {
	return s.IsDraft()
}

// PurchaseLabel marks the shipment as label purchased.
func (s *Shipment) PurchaseLabel(
	providerShipmentID, trackingNumber, barcode string,
	labelDocumentID uuid.UUID,
	labelFormat LabelFormat,
) {
	now := time.Now().UTC()
	s.ProviderShipmentID = &providerShipmentID
	s.ProviderTrackingNumber = &trackingNumber
	s.ProviderBarcode = &barcode
	s.LabelDocumentID = &labelDocumentID
	s.LabelFormat = &labelFormat
	s.Status = ShipmentStatusLabelPurchased
	s.UpdatedAt = now
}

// SchedulePickup schedules a pickup for the shipment.
func (s *Shipment) SchedulePickup(scheduledAt time.Time) {
	s.PickupScheduledAt = &scheduledAt
	s.Status = ShipmentStatusPickupScheduled
	s.UpdatedAt = time.Now().UTC()
}

// MarkPickedUp marks the shipment as picked up.
func (s *Shipment) MarkPickedUp(pickedUpAt time.Time) {
	s.PickedUpAt = &pickedUpAt
	s.Status = ShipmentStatusPickedUp
	s.UpdatedAt = time.Now().UTC()
}

// MarkInTransit marks the shipment as in transit.
func (s *Shipment) MarkInTransit() {
	s.Status = ShipmentStatusInTransit
	s.UpdatedAt = time.Now().UTC()
}

// MarkAtHub marks the shipment as at hub.
func (s *Shipment) MarkAtHub() {
	s.Status = ShipmentStatusAtHub
	s.UpdatedAt = time.Now().UTC()
}

// MarkOutForDelivery marks the shipment as out for delivery.
func (s *Shipment) MarkOutForDelivery() {
	s.Status = ShipmentStatusOutForDelivery
	s.UpdatedAt = time.Now().UTC()
}

// MarkDelivered marks the shipment as delivered.
func (s *Shipment) MarkDelivered(deliveredAt time.Time) {
	s.DeliveredAt = &deliveredAt
	s.Status = ShipmentStatusDelivered
	s.UpdatedAt = time.Now().UTC()
}

// MarkDeliveryFailed marks the shipment as delivery failed.
func (s *Shipment) MarkDeliveryFailed() {
	s.Status = ShipmentStatusDeliveryFailed
	s.UpdatedAt = time.Now().UTC()
}

// MarkReturning marks the shipment as returning.
func (s *Shipment) MarkReturning() {
	s.Status = ShipmentStatusReturning
	s.UpdatedAt = time.Now().UTC()
}

// MarkReturnedToSender marks the shipment as returned to sender.
func (s *Shipment) MarkReturnedToSender() {
	s.Status = ShipmentStatusReturnedToSender
	s.UpdatedAt = time.Now().UTC()
}

// Cancel cancels the shipment.
func (s *Shipment) Cancel(reason string) {
	now := time.Now().UTC()
	s.CancelledAt = &now
	s.CancellationReason = &reason
	s.Status = ShipmentStatusCancelled
	s.UpdatedAt = now
}

// MarkException marks the shipment as having an exception.
func (s *Shipment) MarkException() {
	s.Status = ShipmentStatusException
	s.UpdatedAt = time.Now().UTC()
}

// UpdateStatus updates the shipment status.
func (s *Shipment) UpdateStatus(status ShipmentStatus) {
	s.Status = status
	s.UpdatedAt = time.Now().UTC()
}

// LinkReturnShipment links a return shipment.
func (s *Shipment) LinkReturnShipment(returnShipmentID uuid.UUID) {
	s.ReturnShipmentID = &returnShipmentID
	s.UpdatedAt = time.Now().UTC()
}

// SetMetadata sets a metadata value.
func (s *Shipment) SetMetadata(key string, value interface{}) {
	if s.Metadata == nil {
		s.Metadata = make(map[string]interface{})
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now().UTC()
}

// GetMetadata gets a metadata value.
func (s *Shipment) GetMetadata(key string) (interface{}, bool) {
	if s.Metadata == nil {
		return nil, false
	}
	v, ok := s.Metadata[key]
	return v, ok
}

// MarshalOriginAddressJSON marshals the origin address to JSON.
func (s *Shipment) MarshalOriginAddressJSON() (json.RawMessage, error) {
	return json.Marshal(s.OriginAddress)
}

// UnmarshalOriginAddressJSON unmarshals the origin address from JSON.
func (s *Shipment) UnmarshalOriginAddressJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &s.OriginAddress)
}

// MarshalDestinationAddressJSON marshals the destination address to JSON.
func (s *Shipment) MarshalDestinationAddressJSON() (json.RawMessage, error) {
	return json.Marshal(s.DestinationAddress)
}

// UnmarshalDestinationAddressJSON unmarshals the destination address from JSON.
func (s *Shipment) UnmarshalDestinationAddressJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &s.DestinationAddress)
}

// MarshalParcelJSON marshals the parcel to JSON.
func (s *Shipment) MarshalParcelJSON() (json.RawMessage, error) {
	return json.Marshal(s.Parcel)
}

// UnmarshalParcelJSON unmarshals the parcel from JSON.
func (s *Shipment) UnmarshalParcelJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &s.Parcel)
}

// MarshalMetadataJSON marshals the metadata to JSON.
func (s *Shipment) MarshalMetadataJSON() (json.RawMessage, error) {
	if s.Metadata == nil {
		return json.RawMessage("{}"), nil
	}
	return json.Marshal(s.Metadata)
}

// UnmarshalMetadataJSON unmarshals the metadata from JSON.
func (s *Shipment) UnmarshalMetadataJSON(data json.RawMessage) error {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		s.Metadata = make(map[string]interface{})
		return nil
	}
	return json.Unmarshal(data, &s.Metadata)
}

// GetMargin returns the margin for marketplace shipments.
func (s *Shipment) GetMargin() int64 {
	if !s.IsMarketplace || s.PriceChargedKurus == nil || s.CostKurus == nil {
		return 0
	}
	return *s.PriceChargedKurus - *s.CostKurus
}

// Document represents a stored file (label, POD, etc.).
type Document struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// Type is the document type.
	Type DocumentType `json:"type"`
	// Format is the file format.
	Format string `json:"format"`
	// Filename is the original filename.
	Filename string `json:"filename"`
	// StoragePath is the object storage path.
	StoragePath string `json:"storage_path"`
	// StorageBucket is the storage bucket name.
	StorageBucket string `json:"storage_bucket"`
	// ContentType is the MIME type.
	ContentType string `json:"content_type"`
	// SizeBytes is the file size.
	SizeBytes int `json:"size_bytes"`
	// ContentHash is the SHA-256 hash of the file content.
	ContentHash string `json:"content_hash"`
	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is the timestamp when the document was created.
	CreatedAt time.Time `json:"created_at"`
}

// DocumentType represents the type of a document.
type DocumentType string

const (
	// DocumentTypeLabel is a shipping label.
	DocumentTypeLabel DocumentType = "label"
	// DocumentTypePOD is a proof of delivery.
	DocumentTypePOD DocumentType = "pod"
	// DocumentTypeInvoice is an invoice.
	DocumentTypeInvoice DocumentType = "invoice"
	// DocumentTypeCustoms is a customs document.
	DocumentTypeCustoms DocumentType = "customs"
	// DocumentTypeOther is any other document type.
	DocumentTypeOther DocumentType = "other"
)

// IsValid checks if the document type is valid.
func (t DocumentType) IsValid() bool {
	switch t {
	case DocumentTypeLabel, DocumentTypePOD, DocumentTypeInvoice, DocumentTypeCustoms, DocumentTypeOther:
		return true
	default:
		return false
	}
}

// String returns the string representation of the document type.
func (t DocumentType) String() string {
	return string(t)
}

// NewDocument creates a new Document.
func NewDocument(
	organizationID uuid.UUID,
	docType DocumentType,
	format, filename, storagePath, storageBucket, contentType, contentHash string,
	sizeBytes int,
) *Document {
	return &Document{
		ID:             uuid.New(),
		OrganizationID: organizationID,
		Type:           docType,
		Format:         format,
		Filename:       filename,
		StoragePath:    storagePath,
		StorageBucket:  storageBucket,
		ContentType:    contentType,
		SizeBytes:      sizeBytes,
		ContentHash:    contentHash,
		Metadata:       make(map[string]interface{}),
		CreatedAt:      time.Now().UTC(),
	}
}
