import type { MetadataRoute } from "next";

// PWA-001: installable where supported; the browser remains fully usable.
export default function manifest(): MetadataRoute.Manifest {
  return {
    id: "/",
    name: "TableFlow",
    short_name: "TableFlow",
    description: "Join the queue, order at your table and follow your bill.",
    lang: "th",
    start_url: "/",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    background_color: "#f6f8fa",
    theme_color: "#006b4f",
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icons/maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
