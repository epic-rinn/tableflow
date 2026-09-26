// Customer links carry capability tokens in the fragment so they never reach
// servers or Referer headers.
const PWA = process.env.NEXT_PUBLIC_PWA_URL ?? "http://localhost:3000";

export const trackingLink = (token: string) => `${PWA}/q#${token}`;
export const diningLink = (token: string) => `${PWA}/t#${token}`;
