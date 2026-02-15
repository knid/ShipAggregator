/** Sync status for a Shopify order mapping */
export type SyncStatus =
  | "pending"
  | "processing"
  | "draft_created"
  | "label_purchased"
  | "fulfillment_synced"
  | "cancelled"
  | "failed"
  | "skipped";

/** Order mapping from Shopify to e-port */
export interface OrderMapping {
  id: string;
  store_id: string;
  shopify_order_id: number;
  shopify_order_number: string;
  customer_name: string;
  total_price_kurus: number;
  currency: string;
  sync_status: SyncStatus;
  eport_shipment_id: string | null;
  error_message: string | null;
  synced_at: string | null;
  created_at: string;
  updated_at: string;
}

/** Dashboard stats from the backend */
export interface DashboardStats {
  orders_synced_today: number;
  orders_pending: number;
  orders_failed: number;
  total_orders: number;
  last_sync_at: string | null;
}

/** Shipping rule configuration */
export interface ShippingRule {
  id: string;
  store_id: string;
  name: string;
  priority: number;
  is_active: boolean;
  conditions: RuleCondition[];
  action_provider: string;
  action_service: string;
  created_at: string;
  updated_at: string;
}

/** Condition for a shipping rule */
export interface RuleCondition {
  field: string;
  operator: string;
  value: string;
}

/** Store settings */
export interface StoreSettings {
  id: string;
  shop_domain: string;
  shop_name: string;
  sync_mode: "manual" | "auto";
  scopes: string;
  installed_at: string;
  is_active: boolean;
  organization_id: string;
}

/** API error response */
export interface ApiError {
  error: string;
  message: string;
  status: number;
}
