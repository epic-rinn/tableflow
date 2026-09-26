import { Award, ChefHat, LayoutGrid, Percent, Receipt, ScrollText, SquareMenu, Users, Wallet, type LucideIcon } from "lucide-react";

export const WORKSPACE_ICONS: Record<string, LucideIcon> = {
  "/host": LayoutGrid,
  "/kitchen": ChefHat,
  "/cashier": Wallet,
  "/receipts": Receipt,
  "/configuration": ScrollText,
  "/menu": SquareMenu,
  "/charges": Percent,
  "/loyalty": Award,
  "/staff": Users,
};
