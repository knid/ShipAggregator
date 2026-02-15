"use client";

import { useCallback, useEffect, useState } from "react";
import {
  Page,
  Layout,
  Card,
  Text,
  BlockStack,
  InlineStack,
  Select,
  Button,
  Banner,
  Box,
  Divider,
  SkeletonBodyText,
  Modal,
} from "@shopify/polaris";
import { useAppBridge } from "@shopify/app-bridge-react";
import type { StoreSettings } from "@/lib/types";

export default function SettingsPage() {
  const shopify = useAppBridge();
  const [settings, setSettings] = useState<StoreSettings | null>(null);
  const [syncMode, setSyncMode] = useState<string>("manual");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [disconnectModalOpen, setDisconnectModalOpen] = useState(false);
  const [isDirty, setIsDirty] = useState(false);

  const fetchSettings = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const idToken = await shopify.idToken();
      const response = await fetch("/api/proxy/stores/current/settings", {
        headers: { Authorization: `Bearer ${idToken}` },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch settings: ${response.status}`);
      }

      const data: StoreSettings = await response.json();
      setSettings(data);
      setSyncMode(data.sync_mode);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load settings";
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [shopify]);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  const handleSyncModeChange = useCallback(
    (value: string) => {
      setSyncMode(value);
      setIsDirty(value !== settings?.sync_mode);
    },
    [settings]
  );

  const handleSave = useCallback(async () => {
    try {
      setSaving(true);
      setError(null);
      setSuccess(null);

      const idToken = await shopify.idToken();
      const response = await fetch("/api/proxy/stores/current/settings", {
        method: "PATCH",
        headers: {
          Authorization: `Bearer ${idToken}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ sync_mode: syncMode }),
      });

      if (!response.ok) {
        throw new Error(`Failed to save settings: ${response.status}`);
      }

      const data: StoreSettings = await response.json();
      setSettings(data);
      setSyncMode(data.sync_mode);
      setIsDirty(false);
      setSuccess("Settings saved successfully");
      shopify.toast.show("Settings saved");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to save settings";
      setError(message);
    } finally {
      setSaving(false);
    }
  }, [shopify, syncMode]);

  const handleDisconnect = useCallback(async () => {
    try {
      const idToken = await shopify.idToken();
      const response = await fetch("/api/proxy/stores/current", {
        method: "DELETE",
        headers: { Authorization: `Bearer ${idToken}` },
      });

      if (!response.ok) {
        throw new Error(`Failed to disconnect: ${response.status}`);
      }

      shopify.toast.show("Store disconnected from E-Port");
      setDisconnectModalOpen(false);

      // Redirect to Shopify admin apps page
      window.top?.location.assign("https://admin.shopify.com");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to disconnect";
      setError(message);
      setDisconnectModalOpen(false);
    }
  }, [shopify]);

  const syncModeOptions = [
    { label: "Manual", value: "manual" },
    { label: "Auto (with rules)", value: "auto" },
  ];

  return (
    <Page
      title="Settings"
      backAction={{ url: "/" }}
      primaryAction={{
        content: "Save",
        disabled: !isDirty || saving,
        loading: saving,
        onAction: handleSave,
      }}
    >
      <BlockStack gap="400">
        {error && (
          <Banner title="Error" tone="critical" onDismiss={() => setError(null)}>
            <p>{error}</p>
          </Banner>
        )}

        {success && (
          <Banner
            title="Success"
            tone="success"
            onDismiss={() => setSuccess(null)}
          >
            <p>{success}</p>
          </Banner>
        )}

        <Layout>
          <Layout.AnnotatedSection
            title="Sync configuration"
            description="Choose how Shopify orders are synced to E-Port for shipping label creation."
          >
            <Card>
              {loading ? (
                <SkeletonBodyText lines={3} />
              ) : (
                <BlockStack gap="400">
                  <Select
                    label="Sync mode"
                    options={syncModeOptions}
                    value={syncMode}
                    onChange={handleSyncModeChange}
                    helpText={
                      syncMode === "auto"
                        ? "Orders will be automatically synced based on your shipping rules."
                        : "Orders must be manually synced from the orders page."
                    }
                  />
                </BlockStack>
              )}
            </Card>
          </Layout.AnnotatedSection>

          <Layout.AnnotatedSection
            title="Store information"
            description="Details about your connected Shopify store."
          >
            <Card>
              {loading ? (
                <SkeletonBodyText lines={5} />
              ) : settings ? (
                <BlockStack gap="300">
                  <InfoRow label="Shop domain" value={settings.shop_domain} />
                  <Divider />
                  <InfoRow label="Shop name" value={settings.shop_name} />
                  <Divider />
                  <InfoRow
                    label="Installed"
                    value={new Date(settings.installed_at).toLocaleDateString(
                      "tr-TR",
                      {
                        year: "numeric",
                        month: "long",
                        day: "numeric",
                      }
                    )}
                  />
                  <Divider />
                  <InfoRow label="Scopes" value={settings.scopes} />
                  <Divider />
                  <InfoRow
                    label="Organization ID"
                    value={settings.organization_id}
                  />
                </BlockStack>
              ) : (
                <Text as="p" variant="bodyMd" tone="subdued">
                  No store information available
                </Text>
              )}
            </Card>
          </Layout.AnnotatedSection>

          <Layout.AnnotatedSection
            title="Danger zone"
            description="Disconnect this store from E-Port. This will stop all order syncing and remove the integration."
          >
            <Card>
              <BlockStack gap="300">
                <Text as="p" variant="bodyMd">
                  Disconnecting will remove the link between this Shopify store
                  and your E-Port organization. Existing shipments will not be
                  affected, but no new orders will be synced.
                </Text>
                <Box>
                  <Button
                    variant="primary"
                    tone="critical"
                    onClick={() => setDisconnectModalOpen(true)}
                  >
                    Disconnect store
                  </Button>
                </Box>
              </BlockStack>
            </Card>
          </Layout.AnnotatedSection>
        </Layout>
      </BlockStack>

      <Modal
        open={disconnectModalOpen}
        onClose={() => setDisconnectModalOpen(false)}
        title="Disconnect store?"
        primaryAction={{
          content: "Disconnect",
          destructive: true,
          onAction: handleDisconnect,
        }}
        secondaryActions={[
          {
            content: "Cancel",
            onAction: () => setDisconnectModalOpen(false),
          },
        ]}
      >
        <Modal.Section>
          <Text as="p" variant="bodyMd">
            Are you sure you want to disconnect this store from E-Port? This
            action cannot be undone. You will need to reinstall the app to
            reconnect.
          </Text>
        </Modal.Section>
      </Modal>
    </Page>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <InlineStack align="space-between">
      <Text as="span" variant="bodyMd" fontWeight="semibold">
        {label}
      </Text>
      <Text as="span" variant="bodyMd">
        {value}
      </Text>
    </InlineStack>
  );
}
