"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Page,
  Layout,
  Banner,
  BlockStack,
  Spinner,
  Box,
  Text,
  Button,
  EmptyState,
  Card,
  SkeletonBodyText,
} from "@shopify/polaris";
import { useAppBridge } from "@shopify/app-bridge-react";
import { RuleCard } from "@/components/rule-card";
import type { ShippingRule } from "@/lib/types";

export default function RulesPage() {
  const shopify = useAppBridge();
  const [rules, setRules] = useState<ShippingRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const eportDashboardUrl =
    process.env.NEXT_PUBLIC_EPORT_DASHBOARD_URL || "https://app.e-port.com";

  const fetchRules = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const idToken = await shopify.idToken();
      const response = await fetch("/api/proxy/stores/current/rules", {
        headers: { Authorization: `Bearer ${idToken}` },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch rules: ${response.status}`);
      }

      const data = await response.json();
      setRules(Array.isArray(data) ? data : data.items || []);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load rules";
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [shopify]);

  useEffect(() => {
    fetchRules();
  }, [fetchRules]);

  return (
    <Page
      title="Shipping Rules"
      backAction={{ url: "/" }}
      primaryAction={{
        content: "Manage in E-Port",
        url: `${eportDashboardUrl}/shopify/rules`,
        external: true,
      }}
    >
      <BlockStack gap="400">
        {error && (
          <Banner title="Error" tone="critical" onDismiss={() => setError(null)}>
            <p>{error}</p>
          </Banner>
        )}

        <Banner tone="info">
          <p>
            Shipping rules determine which carrier and service is used for each
            order. Rules are evaluated in priority order. To create or edit rules,
            use the full E-Port dashboard.
          </p>
        </Banner>

        <Layout>
          <Layout.Section>
            {loading ? (
              <BlockStack gap="400">
                {[1, 2, 3].map((i) => (
                  <Card key={i}>
                    <SkeletonBodyText lines={4} />
                  </Card>
                ))}
              </BlockStack>
            ) : rules.length === 0 ? (
              <Card>
                <EmptyState
                  heading="No shipping rules configured"
                  image=""
                >
                  <p>
                    Create shipping rules in the E-Port dashboard to automatically
                    assign carriers to your Shopify orders.
                  </p>
                  <Box paddingBlockStart="300">
                    <Button
                      variant="primary"
                      url={`${eportDashboardUrl}/shopify/rules`}
                      external
                    >
                      Create rules in E-Port
                    </Button>
                  </Box>
                </EmptyState>
              </Card>
            ) : (
              <BlockStack gap="400">
                {rules.map((rule) => (
                  <RuleCard key={rule.id} rule={rule} />
                ))}
              </BlockStack>
            )}
          </Layout.Section>
        </Layout>
      </BlockStack>
    </Page>
  );
}
