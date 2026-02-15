"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Page,
  Layout,
  Card,
  Banner,
  Text,
  BlockStack,
  InlineStack,
  Button,
  Spinner,
  Box,
  Divider,
  SkeletonBodyText,
} from "@shopify/polaris";
import { useAppBridge } from "@shopify/app-bridge-react";
import { OrderTable } from "@/components/order-table";
import type { DashboardStats, OrderMapping } from "@/lib/types";

export default function DashboardPage() {
  const shopify = useAppBridge();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [recentOrders, setRecentOrders] = useState<OrderMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchDashboardData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const idToken = await shopify.idToken();

      const [statsRes, ordersRes] = await Promise.all([
        fetch("/api/proxy/stores/current/stats", {
          headers: { Authorization: `Bearer ${idToken}` },
        }),
        fetch("/api/proxy/stores/current/order-mappings?limit=10&sort=created_at:desc", {
          headers: { Authorization: `Bearer ${idToken}` },
        }),
      ]);

      if (!statsRes.ok) {
        throw new Error(`Failed to fetch stats: ${statsRes.status}`);
      }
      if (!ordersRes.ok) {
        throw new Error(`Failed to fetch orders: ${ordersRes.status}`);
      }

      const statsData = await statsRes.json();
      const ordersData = await ordersRes.json();

      setStats(statsData);
      setRecentOrders(
        Array.isArray(ordersData) ? ordersData : ordersData.items || []
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load dashboard data";
      setError(message);
      console.error("Dashboard fetch error:", err);
    } finally {
      setLoading(false);
    }
  }, [shopify]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  const eportDashboardUrl = process.env.NEXT_PUBLIC_EPORT_DASHBOARD_URL || "https://app.e-port.com";

  return (
    <Page title="Dashboard">
      <BlockStack gap="400">
        <Banner title="Connected to E-Port" tone="success">
          <p>
            Your Shopify store is connected to E-Port shipping management.
            Orders will be synced based on your configuration.
          </p>
        </Banner>

        {error && (
          <Banner title="Error loading data" tone="critical" onDismiss={() => setError(null)}>
            <p>{error}</p>
          </Banner>
        )}

        <Layout>
          <Layout.Section>
            <Card>
              <BlockStack gap="400">
                <Text variant="headingMd" as="h2">
                  Quick Stats
                </Text>
                <Divider />
                {loading ? (
                  <SkeletonBodyText lines={3} />
                ) : stats ? (
                  <InlineStack gap="800" align="start">
                    <StatItem
                      label="Synced Today"
                      value={stats.orders_synced_today}
                    />
                    <StatItem label="Pending" value={stats.orders_pending} />
                    <StatItem label="Failed" value={stats.orders_failed} />
                    <StatItem label="Total Orders" value={stats.total_orders} />
                  </InlineStack>
                ) : (
                  <Text as="p" variant="bodyMd" tone="subdued">
                    No data available
                  </Text>
                )}
              </BlockStack>
            </Card>
          </Layout.Section>

          <Layout.Section>
            <Card>
              <BlockStack gap="400">
                <InlineStack align="space-between" blockAlign="center">
                  <Text variant="headingMd" as="h2">
                    Recent Orders
                  </Text>
                  <Button url="/orders" variant="plain">
                    View all orders
                  </Button>
                </InlineStack>
                <Divider />
                {loading ? (
                  <Box padding="400">
                    <BlockStack gap="200" inlineAlign="center">
                      <Spinner size="large" />
                      <Text as="p" variant="bodyMd" tone="subdued">
                        Loading orders...
                      </Text>
                    </BlockStack>
                  </Box>
                ) : (
                  <OrderTable orders={recentOrders} />
                )}
              </BlockStack>
            </Card>
          </Layout.Section>

          <Layout.Section>
            <Card>
              <BlockStack gap="300">
                <Text variant="headingMd" as="h2">
                  Full Management
                </Text>
                <Text as="p" variant="bodyMd">
                  Access the full E-Port dashboard for advanced shipping
                  management, analytics, and carrier configuration.
                </Text>
                <Box>
                  <Button
                    url={eportDashboardUrl}
                    variant="primary"
                    external
                  >
                    Open E-Port Dashboard
                  </Button>
                </Box>
              </BlockStack>
            </Card>
          </Layout.Section>
        </Layout>
      </BlockStack>
    </Page>
  );
}

function StatItem({ label, value }: { label: string; value: number }) {
  return (
    <BlockStack gap="100" inlineAlign="center">
      <Text variant="headingXl" as="p" alignment="center">
        {value.toLocaleString("tr-TR")}
      </Text>
      <Text variant="bodySm" as="p" tone="subdued" alignment="center">
        {label}
      </Text>
    </BlockStack>
  );
}
