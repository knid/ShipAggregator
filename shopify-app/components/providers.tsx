"use client";

import { AppProvider } from "@shopify/polaris";
import enTranslations from "@shopify/polaris/locales/en.json";
import { ReactNode } from "react";

interface ProvidersProps {
  children: ReactNode;
}

/**
 * Wraps the application with Shopify Polaris AppProvider.
 *
 * App Bridge is initialized automatically via the app-bridge.js script tag
 * injected in the root layout. The useAppBridge() hook from
 * @shopify/app-bridge-react is used in individual components as needed.
 */
export function Providers({ children }: ProvidersProps) {
  return (
    <AppProvider i18n={enTranslations}>
      {children}
    </AppProvider>
  );
}
