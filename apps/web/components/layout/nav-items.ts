import { LayoutDashboard, Play, LayoutTemplate, Settings } from "lucide-react";

export interface NavItem {
  href: string;
  label: string;
  icon: typeof LayoutDashboard;
}

export const NAV_ITEMS: NavItem[] = [
  { href: "/workflows", label: "Workflows", icon: LayoutDashboard },
  { href: "/runs", label: "Runs", icon: Play },
  { href: "/templates", label: "Templates", icon: LayoutTemplate },
  { href: "/settings", label: "Settings", icon: Settings },
];
