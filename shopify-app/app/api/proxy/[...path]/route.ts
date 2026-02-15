import { NextRequest, NextResponse } from "next/server";
import { verifySessionToken, extractShopDomain, getStoreByShopDomain } from "@/lib/session";
import { eportClient } from "@/lib/eport-client";

/**
 * API Proxy route handler.
 *
 * Proxies requests from the Shopify embedded app to the e-port backend.
 * All requests are authenticated via Shopify session tokens (JWT).
 *
 * The path after /api/proxy/ is forwarded to /v1/shopify/ on the backend.
 * For example:
 *   GET /api/proxy/stores/current/stats
 *   -> GET /v1/shopify/stores/{storeId}/stats on the e-port backend
 *
 * The "current" path segment in store paths is replaced with the actual
 * store ID looked up from the session token's shop domain.
 */

async function handleProxy(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  // Extract the session token from the Authorization header
  const authHeader = request.headers.get("authorization");
  if (!authHeader || !authHeader.startsWith("Bearer ")) {
    return NextResponse.json(
      { error: "Missing or invalid Authorization header" },
      { status: 401 }
    );
  }

  const token = authHeader.slice(7);

  // Verify the Shopify session token
  let sessionPayload;
  try {
    sessionPayload = await verifySessionToken(token);
  } catch (error) {
    console.error("Session token verification failed:", error);
    return NextResponse.json(
      { error: "Invalid or expired session token" },
      { status: 401 }
    );
  }

  // Look up the store by shop domain
  const shopDomain = extractShopDomain(sessionPayload);
  const store = await getStoreByShopDomain(shopDomain);

  if (!store) {
    return NextResponse.json(
      { error: "Store not found. Please reinstall the app." },
      { status: 404 }
    );
  }

  // Build the backend path
  const { path } = await params;
  let backendPath = `/v1/shopify/${path.join("/")}`;

  // Replace "current" with the actual store ID
  backendPath = backendPath.replace("/stores/current/", `/stores/${store.id}/`);
  backendPath = backendPath.replace("/stores/current", `/stores/${store.id}`);

  // Preserve query parameters
  const url = new URL(request.url);
  if (url.search) {
    backendPath += url.search;
  }

  // Read the request body if present
  let body: string | undefined;
  if (request.method !== "GET" && request.method !== "HEAD") {
    try {
      body = await request.text();
    } catch {
      // No body is fine for some requests
    }
  }

  // Forward the request to the e-port backend
  try {
    const backendResponse = await eportClient.proxy(request.method, backendPath, {
      body,
      organizationId: store.organization_id,
      headers: {
        "X-Store-ID": store.id,
        "X-Shop-Domain": shopDomain,
      },
    });

    // Forward the response back
    const responseBody = await backendResponse.text();
    const responseHeaders = new Headers();

    // Copy relevant headers from the backend response
    const contentType = backendResponse.headers.get("content-type");
    if (contentType) {
      responseHeaders.set("content-type", contentType);
    }

    return new NextResponse(responseBody, {
      status: backendResponse.status,
      headers: responseHeaders,
    });
  } catch (error) {
    console.error("Proxy request failed:", error);
    return NextResponse.json(
      { error: "Failed to communicate with E-Port backend" },
      { status: 502 }
    );
  }
}

export const GET = handleProxy;
export const POST = handleProxy;
export const PUT = handleProxy;
export const PATCH = handleProxy;
export const DELETE = handleProxy;
