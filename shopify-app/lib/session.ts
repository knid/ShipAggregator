import * as jose from "jose";
import { eportClient } from "./eport-client";

export interface SessionTokenPayload {
  /** The shop's myshopify.com domain */
  dest: string;
  /** The shop's admin domain */
  iss: string;
  /** Audience: the app's API key */
  aud: string;
  /** Subject: the user ID */
  sub: string;
  /** Expiration */
  exp: number;
  /** Not before */
  nbf: number;
  /** Issued at */
  iat: number;
  /** JWT ID */
  jti: string;
  /** Session ID */
  sid: string;
}

/**
 * Verifies a Shopify session token (JWT) from the Authorization header.
 * Returns the decoded payload if valid, or throws an error.
 */
export async function verifySessionToken(
  token: string
): Promise<SessionTokenPayload> {
  const apiSecret = process.env.SHOPIFY_API_SECRET;
  const apiKey = process.env.SHOPIFY_API_KEY;

  if (!apiSecret || !apiKey) {
    throw new Error("Missing SHOPIFY_API_SECRET or SHOPIFY_API_KEY env vars");
  }

  const secret = new TextEncoder().encode(apiSecret);

  const { payload } = await jose.jwtVerify(token, secret, {
    algorithms: ["HS256"],
    audience: apiKey,
    clockTolerance: 10,
  });

  return payload as unknown as SessionTokenPayload;
}

/**
 * Extracts the shop domain from a session token's `dest` claim.
 * The dest field is like "https://mystore.myshopify.com"
 */
export function extractShopDomain(payload: SessionTokenPayload): string {
  try {
    const url = new URL(payload.dest);
    return url.hostname;
  } catch {
    // If dest is already just a hostname
    return payload.dest;
  }
}

export interface StoreInfo {
  id: string;
  organization_id: string;
  shop_domain: string;
  shop_name: string;
  access_token_set: boolean;
  scopes: string;
  sync_mode: string;
  installed_at: string;
  is_active: boolean;
}

/**
 * Looks up a store in the e-port backend by its shop domain.
 */
export async function getStoreByShopDomain(
  shopDomain: string
): Promise<StoreInfo | null> {
  try {
    const response = await eportClient.get(
      `/v1/shopify/stores/lookup?shop_domain=${encodeURIComponent(shopDomain)}`
    );

    if (!response.ok) {
      if (response.status === 404) {
        return null;
      }
      throw new Error(`Failed to lookup store: ${response.status}`);
    }

    return response.json();
  } catch (error) {
    console.error("Error looking up store:", error);
    return null;
  }
}
