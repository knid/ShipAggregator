"use client";

import {
  IndexTable,
  Text,
  useBreakpoints,
} from "@shopify/polaris";
import type { OrderMapping } from "@/lib/types";
import { StatusBadge } from "./status-badge";

interface OrderTableProps {
  orders: OrderMapping[];
  loading?: boolean;
}

export function OrderTable({ orders, loading = false }: OrderTableProps) {
  const { smDown } = useBreakpoints();

  const resourceName = {
    singular: "order",
    plural: "orders",
  };

  const rowMarkup = orders.map((order, index) => {
    const totalFormatted = formatCurrency(
      order.total_price_kurus,
      order.currency
    );
    const syncedAt = order.synced_at
      ? new Date(order.synced_at).toLocaleString("tr-TR")
      : "-";

    return (
      <IndexTable.Row id={order.id} key={order.id} position={index}>
        <IndexTable.Cell>
          <Text variant="bodyMd" fontWeight="bold" as="span">
            #{order.shopify_order_number}
          </Text>
        </IndexTable.Cell>
        <IndexTable.Cell>
          <Text variant="bodyMd" as="span">
            {order.customer_name}
          </Text>
        </IndexTable.Cell>
        <IndexTable.Cell>
          <Text variant="bodyMd" as="span" alignment="end">
            {totalFormatted}
          </Text>
        </IndexTable.Cell>
        <IndexTable.Cell>
          <StatusBadge status={order.sync_status} />
        </IndexTable.Cell>
        <IndexTable.Cell>
          <Text variant="bodyMd" as="span">
            {syncedAt}
          </Text>
        </IndexTable.Cell>
      </IndexTable.Row>
    );
  });

  return (
    <IndexTable
      condensed={smDown}
      resourceName={resourceName}
      itemCount={orders.length}
      headings={[
        { title: "Order #" },
        { title: "Customer" },
        { title: "Total", alignment: "end" },
        { title: "Status" },
        { title: "Synced At" },
      ]}
      selectable={false}
      loading={loading}
    >
      {rowMarkup}
    </IndexTable>
  );
}

function formatCurrency(amountKurus: number, currency: string): string {
  const amount = amountKurus / 100;
  try {
    return new Intl.NumberFormat("tr-TR", {
      style: "currency",
      currency: currency || "TRY",
    }).format(amount);
  } catch {
    return `${amount.toFixed(2)} ${currency}`;
  }
}
