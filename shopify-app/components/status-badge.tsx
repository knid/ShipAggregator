"use client";

import { Badge } from "@shopify/polaris";
import type { SyncStatus } from "@/lib/types";

type BadgeTone =
  | "success"
  | "info"
  | "attention"
  | "warning"
  | "critical"
  | undefined;

const STATUS_CONFIG: Record<SyncStatus, { label: string; tone: BadgeTone }> = {
  pending: { label: "Pending", tone: "attention" },
  processing: { label: "Processing", tone: "info" },
  draft_created: { label: "Draft Created", tone: "info" },
  label_purchased: { label: "Label Purchased", tone: "success" },
  fulfillment_synced: { label: "Fulfilled", tone: "success" },
  cancelled: { label: "Cancelled", tone: "warning" },
  failed: { label: "Failed", tone: "critical" },
  skipped: { label: "Skipped", tone: undefined },
};

interface StatusBadgeProps {
  status: SyncStatus;
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const config = STATUS_CONFIG[status] || {
    label: status,
    tone: undefined,
  };

  return <Badge tone={config.tone}>{config.label}</Badge>;
}
