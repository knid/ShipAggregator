import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,

  async headers() {
    return [
      {
        // Allow Shopify Admin to embed this app in an iframe
        source: "/:path*",
        headers: [
          {
            key: "X-Frame-Options",
            value: "",
          },
          {
            key: "Content-Security-Policy",
            value: [
              "frame-ancestors https://*.myshopify.com https://admin.shopify.com",
              "default-src 'self'",
              "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.shopify.com",
              "style-src 'self' 'unsafe-inline' https://cdn.shopify.com",
              "img-src 'self' data: https://cdn.shopify.com https://*.myshopify.com",
              "font-src 'self' https://cdn.shopify.com",
              "connect-src 'self' https://*.myshopify.com https://admin.shopify.com",
            ].join("; "),
          },
        ],
      },
    ];
  },
};

export default nextConfig;
