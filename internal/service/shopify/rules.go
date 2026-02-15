package shopify

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// OrderContext holds the data needed to evaluate shipping rules.
type OrderContext struct {
	DestinationCityCode string
	WeightGrams         int
	OrderTotalKurus     int64
	HasCOD              bool
	Tags                []string
}

// EvaluateRules returns the first matching active rule for the given order context.
// Rules are evaluated in priority order (ascending). All non-null conditions must match (AND).
func EvaluateRules(ctx context.Context, queries *db.Queries, orgID uuid.UUID, storeID *uuid.UUID, orderCtx *OrderContext) (*db.ShopifyShippingRule, error) {
	params := db.ListActiveShopifyShippingRulesParams{
		OrganizationID: orgID,
	}
	if storeID != nil {
		params.StoreID = pgtype.UUID{Bytes: *storeID, Valid: true}
	}

	rules, err := queries.ListActiveShopifyShippingRules(ctx, params)
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		if matchesRule(rule, orderCtx) {
			return rule, nil
		}
	}

	return nil, nil
}

// matchesRule checks if an order context matches all non-null conditions of a rule.
func matchesRule(rule *db.ShopifyShippingRule, oc *OrderContext) bool {
	// City code condition
	if len(rule.ConditionDestinationCityCodes) > 0 {
		found := false
		for _, code := range rule.ConditionDestinationCityCodes {
			if code == oc.DestinationCityCode {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Weight range
	if rule.ConditionMinWeightGrams != nil && oc.WeightGrams < int(*rule.ConditionMinWeightGrams) {
		return false
	}
	if rule.ConditionMaxWeightGrams != nil && oc.WeightGrams > int(*rule.ConditionMaxWeightGrams) {
		return false
	}

	// Order total range
	if rule.ConditionMinOrderTotalKurus != nil && oc.OrderTotalKurus < *rule.ConditionMinOrderTotalKurus {
		return false
	}
	if rule.ConditionMaxOrderTotalKurus != nil && oc.OrderTotalKurus > *rule.ConditionMaxOrderTotalKurus {
		return false
	}

	// COD condition
	if rule.ConditionHasCod != nil && *rule.ConditionHasCod != oc.HasCOD {
		return false
	}

	// Tags condition (ANY match)
	if len(rule.ConditionTags) > 0 {
		found := false
		for _, ruleTag := range rule.ConditionTags {
			for _, orderTag := range oc.Tags {
				if strings.EqualFold(strings.TrimSpace(ruleTag), strings.TrimSpace(orderTag)) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
