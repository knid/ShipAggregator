import { NextRequest, NextResponse } from "next/server";
import crypto from "crypto";

/**
 * GET /auth
 *
 * Initiates the Shopify OAuth flow by redirecting the merchant to the
 * Shopify authorization page. The shop domain is passed as a query parameter.
 *
 * Query parameters:
 *   - shop: The merchant's myshopify.com domain (e.g., "mystore.myshopify.com")
 */
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const shop = searchParams.get("shop");

  if (!shop) {
    return NextResponse.json(
      { error: "Missing required 'shop' parameter" },
      { status: 400 }
    );
  }

  // Validate shop domain format to prevent open redirect
  const shopRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]*\.myshopify\.com$/;
  if (!shopRegex.test(shop)) {
    return NextResponse.json(
      { error: "Invalid shop domain format" },
      { status: 400 }
    );
  }

  const apiKey = process.env.SHOPIFY_API_KEY;
  const scopes = process.env.SHOPIFY_SCOPES || "read_orders,write_fulfillments,read_shipping";
  const appUrl = process.env.SHOPIFY_APP_URL;

  if (!apiKey || !appUrl) {
    return NextResponse.json(
      { error: "Server misconfiguration: missing SHOPIFY_API_KEY or SHOPIFY_APP_URL" },
      { status: 500 }
    );
  }

  const redirectUri = `${appUrl}/auth/callback`;

  // Generate a random state parameter for CSRF protection
  const state = crypto.randomBytes(16).toString("hex");

  // Build the Shopify OAuth authorize URL
  const authUrl = new URL(`https://${shop}/admin/oauth/authorize`);
  authUrl.searchParams.set("client_id", apiKey);
  authUrl.searchParams.set("scope", scopes);
  authUrl.searchParams.set("redirect_uri", redirectUri);
  authUrl.searchParams.set("state", state);

  // Set the state in a cookie so we can verify it on callback
  const response = NextResponse.redirect(authUrl.toString());
  response.cookies.set("shopify_oauth_state", state, {
    httpOnly: true,
    secure: true,
    sameSite: "lax",
    maxAge: 600, // 10 minutes
    path: "/",
  });

  return response;
}
