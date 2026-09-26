import type { Metadata } from "next";
import { QrEntry } from "@/components/QrEntry";

export const metadata: Metadata = { title: "Queue ticket · TableFlow", robots: { index: false } };

export default function QueueEntry() {
  return <QrEntry kind="queue" />;
}
