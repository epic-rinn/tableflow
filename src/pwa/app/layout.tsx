import type { Metadata, Viewport } from "next";
import "@fontsource-variable/inter";
import "@fontsource-variable/noto-sans-thai";
import "./globals.css";
import { ServiceWorkerManager } from "@/components/ServiceWorkerManager";
import { LocaleProvider } from "@/lib/i18n";

export const metadata: Metadata = {
  title: "TableFlow",
  description: "Restaurant queue and table ordering",
  applicationName: "TableFlow",
  appleWebApp: { capable: true, title: "TableFlow", statusBarStyle: "default" },
  icons: { apple: "/icons/apple-touch-icon.png" },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: "#006b4f",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="th">
      <body className="min-h-dvh bg-muted/40 font-sans">
        <LocaleProvider>
          <ServiceWorkerManager />
          {children}
        </LocaleProvider>
      </body>
    </html>
  );
}
