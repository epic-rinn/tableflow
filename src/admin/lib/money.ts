const thb = new Intl.NumberFormat("th-TH", { style: "currency", currency: "THB" });

// Prices are integer satang on the wire; display as baht.
export const formatTHB = (satang: number) => thb.format(satang / 100);

// Parses a baht amount typed by staff ("120" or "120.50") into satang.
export function bahtToSatang(input: string): number | null {
  if (!/^\d{1,6}(\.\d{1,2})?$/.test(input.trim())) return null;
  const [whole, frac = ""] = input.trim().split(".");
  return Number(whole) * 100 + Number(frac.padEnd(2, "0"));
}

export const satangToBaht = (satang: number) => (satang / 100).toFixed(2).replace(/\.00$/, "");

// Rates travel as basis points (1/100 %); staff type percentages ("7", "7.25").
export function percentToBP(input: string): number | null {
  if (!/^\d{1,3}(\.\d{1,2})?$/.test(input.trim())) return null;
  const [whole, frac = ""] = input.trim().split(".");
  const bp = Number(whole) * 100 + Number(frac.padEnd(2, "0"));
  return bp <= 10000 ? bp : null;
}

export const bpToPercent = (bp: number) => (bp / 100).toFixed(2).replace(/\.?0+$/, "");
