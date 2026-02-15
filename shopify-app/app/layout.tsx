import type { Metadata } from "next";
import { Providers } from "@/components/providers";
import "@shopify/polaris/build/esm/styles.css";
import "./globals.css";

export const metadata: Metadata = {
  title: "E-Port Shipping",
  description: "Manage your shipping integrations with e-port",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const apiKey = process.env.NEXT_PUBLIC_SHOPIFY_API_KEY || "";

  return (
    <html lang="en">
      <head>
        {/* App Bridge script for embedded Shopify apps */}
        <script
          src="https://cdn.shopify.com/shopifycloud/app-bridge.js"
          data-api-key={apiKey}
        />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
