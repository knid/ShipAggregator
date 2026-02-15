"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Page,
  Card,
  Banner,
  BlockStack,
  Spinner,
  Box,
  Text,
  IndexFilters,
  useSetIndexFiltersMode,
  TabProps,
  ChoiceList,
} from "@shopify/polaris";
import { useAppBridge } from "@shopify/app-bridge-react";
import { OrderTable } from "@/components/order-table";
import type { OrderMapping, SyncStatus } from "@/lib/types";

const STATUS_TABS: { id: string; content: string; filter: string | null }[] = [
  { id: "all", content: "All", filter: null },
  { id: "pending", content: "Pending", filter: "pending" },
  { id: "processing", content: "Processing", filter: "processing" },
  { id: "synced", content: "Synced", filter: "fulfillment_synced" },
  { id: "failed", content: "Failed", filter: "failed" },
  { id: "skipped", content: "Skipped", filter: "skipped" },
];

export default function OrdersPage() {
  const shopify = useAppBridge();
  const [orders, setOrders] = useState<OrderMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTab, setSelectedTab] = useState(0);
  const [queryValue, setQueryValue] = useState("");
  const [statusFilter, setStatusFilter] = useState<string[]>([]);

  const { mode, setMode } = useSetIndexFiltersMode();

  const fetchOrders = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const idToken = await shopify.idToken();
      const params = new URLSearchParams({ limit: "50", sort: "created_at:desc" });

      // Apply tab filter
      const tabFilter = STATUS_TABS[selectedTab]?.filter;
      if (tabFilter) {
        params.set("status", tabFilter);
      }

      // Apply manual status filter (overrides tab)
      if (statusFilter.length > 0) {
        params.set("status", statusFilter.join(","));
      }

      // Apply search query
      if (queryValue) {
        params.set("search", queryValue);
      }

      const response = await fetch(
        `/api/proxy/stores/current/order-mappings?${params.toString()}`,
        {
          headers: { Authorization: `Bearer ${idToken}` },
        }
      );

      if (!response.ok) {
        throw new Error(`Failed to fetch orders: ${response.status}`);
      }

      const data = await response.json();
      setOrders(Array.isArray(data) ? data : data.items || []);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load orders";
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [shopify, selectedTab, statusFilter, queryValue]);

  useEffect(() => {
    fetchOrders();
  }, [fetchOrders]);

  const handleQueryChange = useCallback((value: string) => {
    setQueryValue(value);
  }, []);

  const handleQueryClear = useCallback(() => {
    setQueryValue("");
  }, []);

  const handleStatusFilterChange = useCallback((value: string[]) => {
    setStatusFilter(value);
  }, []);

  const handleStatusFilterRemove = useCallback(() => {
    setStatusFilter([]);
  }, []);

  const handleClearAll = useCallback(() => {
    setStatusFilter([]);
    setQueryValue("");
  }, []);

  const statusOptions: SyncStatus[] = [
    "pending",
    "processing",
    "draft_created",
    "label_purchased",
    "fulfillment_synced",
    "cancelled",
    "failed",
    "skipped",
  ];

  const filters = [
    {
      key: "status",
      label: "Status",
      filter: (
        <ChoiceList
          title="Status"
          titleHidden
          choices={statusOptions.map((s) => ({
            label: s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase()),
            value: s,
          }))}
          selected={statusFilter}
          onChange={handleStatusFilterChange}
          allowMultiple
        />
      ),
      shortcut: true,
    },
  ];

  const appliedFilters = statusFilter.length > 0
    ? [
        {
          key: "status",
          label: `Status: ${statusFilter.join(", ")}`,
          onRemove: handleStatusFilterRemove,
        },
      ]
    : [];

  const tabs: TabProps[] = STATUS_TABS.map((tab) => ({
    id: tab.id,
    content: tab.content,
    accessibilityLabel: `${tab.content} orders`,
    panelID: `${tab.id}-panel`,
  }));

  return (
    <Page title="Orders" backAction={{ url: "/" }}>
      <BlockStack gap="400">
        {error && (
          <Banner title="Error" tone="critical" onDismiss={() => setError(null)}>
            <p>{error}</p>
          </Banner>
        )}

        <Card padding="0">
          <IndexFilters
            queryValue={queryValue}
            queryPlaceholder="Search orders..."
            onQueryChange={handleQueryChange}
            onQueryClear={handleQueryClear}
            tabs={tabs}
            selected={selectedTab}
            onSelect={setSelectedTab}
            filters={filters}
            appliedFilters={appliedFilters}
            onClearAll={handleClearAll}
            mode={mode}
            setMode={setMode}
            cancelAction={{
              onAction: handleClearAll,
              disabled: false,
              loading: false,
            }}
          />
          {loading ? (
            <Box padding="800">
              <BlockStack gap="200" inlineAlign="center">
                <Spinner size="large" />
                <Text as="p" variant="bodyMd" tone="subdued">
                  Loading orders...
                </Text>
              </BlockStack>
            </Box>
          ) : orders.length === 0 ? (
            <Box padding="800">
              <BlockStack inlineAlign="center">
                <Text as="p" variant="bodyMd" tone="subdued">
                  No orders found matching your filters.
                </Text>
              </BlockStack>
            </Box>
          ) : (
            <OrderTable orders={orders} />
          )}
        </Card>
      </BlockStack>
    </Page>
  );
}
