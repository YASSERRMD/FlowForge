import { AppShell } from "@/components/layout/app-shell";
import { ThemeToggle } from "@/components/theme-toggle";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AppShell actions={<ThemeToggle />}>{children}</AppShell>;
}
