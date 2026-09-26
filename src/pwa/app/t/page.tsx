import type { Metadata } from "next";
import { QrEntry } from "@/components/QrEntry";

export const metadata: Metadata = { title: "Your table · TableFlow", robots: { index: false } };

export default function TableEntry() {
  return <QrEntry kind="visit" />;
}
