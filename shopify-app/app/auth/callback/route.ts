import { NextRequest, NextResponse } from "next/server";
import crypto from "crypto";
import { eportClient } from "@/lib/eport-client";

/**
 * GET /auth/callback
 *
 * Handles the OAuth callback from Shopify after the merchant authorizes the app.
 * Exchanges the temporary code for an access token, then registers the store
 * with the e-port backend.
 *
 * Query parameters:
 *   - code: Temporary authorization code from Shopify
 *   - shop: The merchant's myshopify.com domain
 *   - state: CSRF protection token (must match cookie)
 *   - hmac: Shopify's HMAC signature of the query params
 */
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const code = searchParams.get("code");
  const shop = searchParams.get("shop");
  const state = searchParams.get("state");
  const hmac = searchParams.get("hmac");

  // Validate required parameters
  if (!code || !shop || !state || !hmac) {
    return NextResponse.json(
      { error: "Missing required OAuth callback parameters" },
      { status: 400 }
    );
  }

  // Validate shop domain format
  const shopRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]*\.myshopify\.com$/;
  if (!shopRegex.test(shop)) {
    return NextResponse.json(
      { error: "Invalid shop domain format" },
      { status: 400 }
    );
  }

  // Verify state parameter matches the cookie
  const savedState = request.cookies.get("shopify_oauth_state")?.value;
  if (!savedState || savedState !== state) {
    return NextResponse.json(
      { error: "Invalid state parameter - possible CSRF attack" },
      { status: 403 }
    );
  }

  // Verify HMAC signature
  const apiSecret = process.env.SHOPIFY_API_SECRET;
  if (!apiSecret) {
    return NextResponse.json(
      { error: "Server misconfiguration: missing SHOPIFY_API_SECRET" },
      { status: 500 }
    );
  }

  if (!verifyHmac(searchParams, apiSecret)) {
    return NextResponse.json(
      { error: "Invalid HMAC signature" },
      { status: 403 }
    );
  }

  // Exchange the code for an access token
  const apiKey = process.env.SHOPIFY_API_KEY;
  if (!apiKey) {
    return NextResponse.json(
      { error: "Server misconfiguration: missing SHOPIFY_API_KEY" },
      { status: 500 }
    );
  }

  let accessToken: string;
  let shopifyScopes: string;

  try {
    const tokenResponse = await fetch(
      `https://${shop}/admin/oauth/access_token`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          client_id: apiKey,
          client_secret: apiSecret,
          code,
        }),
      }
    );

    if (!tokenResponse.ok) {
      const errorBody = await tokenResponse.text();
      console.error("Token exchange failed:", tokenResponse.status, errorBody);
      return NextResponse.json(
        { error: "Failed to exchange authorization code for access token" },
        { status: 502 }
      );
    }

    const tokenData = await tokenResponse.json();
    accessToken = tokenData.access_token;
    shopifyScopes = tokenData.scope || "";
  } catch (error) {
    console.error("Token exchange error:", error);
    return NextResponse.json(
      { error: "Failed to communicate with Shopify" },
      { status: 502 }
    );
  }

  // Fetch shop info from Shopify to get the shop name
  let shopName = shop;
  try {
    const shopResponse = await fetch(
      `https://${shop}/admin/api/2024-10/shop.json`,
      {
        headers: {
          "X-Shopify-Access-Token": accessToken,
        },
      }
    );

    if (shopResponse.ok) {
      const shopData = await shopResponse.json();
      shopName = shopData.shop?.name || shop;
    }
  } catch {
    // Non-critical - we can proceed with just the domain as the name
    console.warn("Failed to fetch shop info, using domain as name");
  }

  // Register the store with the e-port backend
  // The org_id comes from a query parameter or a pre-configured value
  const orgId = searchParams.get("org_id") || process.env.EPORT_DEFAULT_ORG_ID || "";

  try {
    const registerResponse = await eportClient.post(
      "/v1/integrations/shopify/register-store",
      {
        shop_domain: shop,
        shop_name: shopName,
        access_token: accessToken,
        scopes: shopifyScopes,
        organization_id: orgId,
      }
    );

    if (!registerResponse.ok) {
      const errorBody = await registerResponse.text();
      console.error(
        "Store registration failed:",
        registerResponse.status,
        errorBody
      );
      return NextResponse.json(
        { error: "Failed to register store with E-Port backend" },
        { status: 502 }
      );
    }
  } catch (error) {
    console.error("Store registration error:", error);
    return NextResponse.json(
      { error: "Failed to communicate with E-Port backend" },
      { status: 502 }
    );
  }

  // Redirect back to the embedded app inside Shopify Admin
  const appUrl = process.env.SHOPIFY_APP_URL || "";
  const embeddedUrl = `https://${shop}/admin/apps/${apiKey}`;

  const response = NextResponse.redirect(embeddedUrl);

  // Clear the OAuth state cookie
  response.cookies.delete("shopify_oauth_state");

  return response;
}

/**
 * Verifies the HMAC signature from Shopify's OAuth callback.
 */
function verifyHmac(
  searchParams: URLSearchParams,
  secret: string
): boolean {
  const hmac = searchParams.get("hmac");
  if (!hmac) return false;

  // Build the message string from all params except hmac
  const params = new URLSearchParams();
  searchParams.forEach((value, key) => {
    if (key !== "hmac") {
      params.set(key, value);
    }
  });

  // Sort parameters alphabetically
  const sortedParams = new URLSearchParams(
    [...params.entries()].sort(([a], [b]) => a.localeCompare(b))
  );

  const message = sortedParams.toString();
  const generatedHmac = crypto
    .createHmac("sha256", secret)
    .update(message)
    .digest("hex");

  // Use timing-safe comparison to prevent timing attacks
  try {
    return crypto.timingSafeEqual(
      Buffer.from(hmac, "hex"),
      Buffer.from(generatedHmac, "hex")
    );
  } catch {
    return false;
  }
}
