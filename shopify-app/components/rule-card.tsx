"use client";

import {
  Card,
  Text,
  Badge,
  InlineStack,
  BlockStack,
  Box,
  Divider,
  List,
} from "@shopify/polaris";
import type { ShippingRule } from "@/lib/types";

interface RuleCardProps {
  rule: ShippingRule;
}

export function RuleCard({ rule }: RuleCardProps) {
  return (
    <Card>
      <BlockStack gap="300">
        <InlineStack align="space-between" blockAlign="center">
          <InlineStack gap="200" blockAlign="center">
            <Text variant="headingMd" as="h3">
              {rule.name}
            </Text>
            <Badge tone={rule.is_active ? "success" : undefined}>
              {rule.is_active ? "Active" : "Inactive"}
            </Badge>
          </InlineStack>
          <Text variant="bodySm" as="span" tone="subdued">
            Priority: {rule.priority}
          </Text>
        </InlineStack>

        <Divider />

        <BlockStack gap="200">
          <Text variant="headingSm" as="h4">
            Conditions
          </Text>
          {rule.conditions.length > 0 ? (
            <List>
              {rule.conditions.map((condition, idx) => (
                <List.Item key={idx}>
                  <Text as="span" variant="bodyMd">
                    {formatCondition(condition.field, condition.operator, condition.value)}
                  </Text>
                </List.Item>
              ))}
            </List>
          ) : (
            <Text as="p" variant="bodyMd" tone="subdued">
              No conditions (matches all orders)
            </Text>
          )}
        </BlockStack>

        <Divider />

        <InlineStack gap="200" blockAlign="center">
          <Text variant="headingSm" as="h4">
            Ship with:
          </Text>
          <Badge tone="info">{rule.action_provider}</Badge>
          {rule.action_service && (
            <Text as="span" variant="bodyMd" tone="subdued">
              / {rule.action_service}
            </Text>
          )}
        </InlineStack>
      </BlockStack>
    </Card>
  );
}

function formatCondition(
  field: string,
  operator: string,
  value: string
): string {
  const fieldLabels: Record<string, string> = {
    total_price: "Order total",
    weight: "Weight",
    destination_city: "City",
    destination_district: "District",
    item_count: "Item count",
    tag: "Order tag",
  };

  const operatorLabels: Record<string, string> = {
    eq: "equals",
    neq: "does not equal",
    gt: "greater than",
    gte: "greater than or equal to",
    lt: "less than",
    lte: "less than or equal to",
    contains: "contains",
    in: "is one of",
  };

  const fieldLabel = fieldLabels[field] || field;
  const operatorLabel = operatorLabels[operator] || operator;

  return `${fieldLabel} ${operatorLabel} ${value}`;
}
