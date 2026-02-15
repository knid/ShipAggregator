import "@shopify/shopify-api/adapters/node";
import {
  shopifyApi,
  LATEST_API_VERSION,
  LogSeverity,
} from "@shopify/shopify-api";

const isDev = process.env.NODE_ENV === "development";

if (!process.env.SHOPIFY_API_KEY) {
  throw new Error("SHOPIFY_API_KEY environment variable is required");
}
if (!process.env.SHOPIFY_API_SECRET) {
  throw new Error("SHOPIFY_API_SECRET environment variable is required");
}

const shopify = shopifyApi({
  apiKey: process.env.SHOPIFY_API_KEY,
  apiSecretKey: process.env.SHOPIFY_API_SECRET,
  scopes: (process.env.SHOPIFY_SCOPES || "read_orders,write_fulfillments,read_shipping").split(","),
  hostName: (process.env.SHOPIFY_APP_URL || "https://localhost:3001").replace(/^https?:\/\//, ""),
  hostScheme: "https",
  apiVersion: LATEST_API_VERSION,
  isEmbeddedApp: true,
  logger: {
    level: isDev ? LogSeverity.Debug : LogSeverity.Warning,
  },
});

export default shopify;
export { LATEST_API_VERSION };
