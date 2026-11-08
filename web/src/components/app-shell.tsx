import { Link, Outlet } from "@tanstack/react-router";
import { Ellipsis, LayoutDashboard, List, Plus, type LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface NavItem {
  to: "/" | "/entries" | "/new" | "/more";
  label: string;
  icon: LucideIcon;
}

const navItems: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/entries", label: "Entries", icon: List },
  { to: "/new", label: "Add entry", icon: Plus },
  { to: "/more", label: "More", icon: Ellipsis },
];

export function AppShell() {
  return (
    <div className="min-h-dvh md:flex">
      <Sidebar />
      <main
        className={cn(
          "flex-1 pt-[max(1rem,env(safe-area-inset-top))] pr-[max(1rem,env(safe-area-inset-right))] pl-[max(1rem,env(safe-area-inset-left))]",
          "pb-[calc(5rem+env(safe-area-inset-bottom))] md:px-8 md:pt-[max(1.5rem,env(safe-area-inset-top))] md:pb-8",
        )}
      >
        <Outlet />
      </main>
      <TabBar />
    </div>
  );
}

function TabBar() {
  return (
    <nav
      aria-label="Tabs"
      className="bg-background/95 fixed inset-x-0 bottom-0 z-40 border-t pr-[env(safe-area-inset-right)] pb-[env(safe-area-inset-bottom)] pl-[env(safe-area-inset-left)] backdrop-blur md:hidden"
    >
      <ul className="grid grid-cols-4">
        {navItems.map(({ to, label, icon: Icon }) => (
          <li key={to} className="flex justify-center">
            {to === "/new" ? (
              <Link
                to={to}
                aria-label={label}
                className="bg-primary text-primary-foreground my-1.5 flex size-12 items-center justify-center rounded-full shadow-md"
              >
                <Icon className="size-6" aria-hidden />
              </Link>
            ) : (
              <Link
                to={to}
                activeOptions={{ exact: to === "/" }}
                className="text-muted-foreground data-[status=active]:text-primary flex h-15 w-full flex-col items-center justify-center gap-1 text-xs font-medium"
              >
                <Icon className="size-5" aria-hidden />
                {label}
              </Link>
            )}
          </li>
        ))}
      </ul>
    </nav>
  );
}

function Sidebar() {
  return (
    <nav
      aria-label="Sidebar"
      className="bg-sidebar text-sidebar-foreground sticky top-0 hidden h-dvh w-60 shrink-0 flex-col gap-1 border-r px-3 pt-[max(1.5rem,env(safe-area-inset-top))] pl-[max(0.75rem,env(safe-area-inset-left))] md:flex"
    >
      <span className="mb-4 px-3 text-lg font-semibold">Spendulum</span>
      {navItems.map(({ to, label, icon: Icon }) => (
        <Link
          key={to}
          to={to}
          activeOptions={{ exact: to === "/" }}
          className="text-muted-foreground hover:bg-accent data-[status=active]:bg-accent data-[status=active]:text-foreground flex h-11 items-center gap-3 rounded-md px-3 text-sm font-medium"
        >
          <Icon className="size-5" aria-hidden />
          {label}
        </Link>
      ))}
    </nav>
  );
}
