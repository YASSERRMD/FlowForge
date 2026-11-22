import { Sidebar } from "./sidebar";
import { TopNav } from "./top-nav";

export function AppShell({
  children,
  actions,
}: {
  children: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:m-2 focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground"
      >
        Skip to content
      </a>
      <TopNav actions={actions} />
      <div className="flex flex-1">
        <aside className="hidden w-56 shrink-0 border-r border-border md:block">
          <Sidebar />
        </aside>
        <main id="main-content" className="flex-1 p-4 md:p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
