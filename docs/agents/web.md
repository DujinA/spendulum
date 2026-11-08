# Web app

The web app in `web/` is one PWA used on desktop and on iPhone from the home screen.

## Stack

- React + TypeScript (strict, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, no `any`) + Vite.
- TanStack Router, code-based routes in `src/router.tsx`. Put the period preset, filters and search in typed search params, so they live in the URL.
- TanStack Query for server state. Call the API only through `openapi-fetch`, typed by `src/api/schema.gen.ts` (`task gen`). No hand-written `fetch` calls, and no other state library. The first slice that calls the API adds these dependencies.
- Tailwind v4 + shadcn/ui (`components.json`). Add a component with `pnpm dlx shadcn@latest add <name>` from `web/`, then check its imports: they must use `@/lib/utils`. Use the Drawer for the fast-entry sheets and Sonner for the undo toast.
- Recharts for the dashboard. react-hook-form + zod for edit forms. The fast-entry keypad is a custom component.
- vite-plugin-pwa in `prompt` mode (`src/pwa.ts`): it precaches the app shell only, and `/api` is on the navigation-fallback denylist. `src/components/update-prompt.tsx` checks for a new service worker on resume and hourly.

## Rules

- An entry date is a `YYYY-MM-DD` string, never a JS `Date`.
- Amounts are integer minor units, formatted through one `formatMoney` helper (for example `1,780.80 din`). No float arithmetic.
- Inputs are at least 16 px, so iOS does not zoom on focus. Tap targets are at least 44 px.
- Respect safe-area insets with `env(safe-area-inset-*)`. The viewport uses `viewport-fit=cover`.
- Light and dark mode follow the system setting (`prefers-color-scheme`); use the theme colour tokens in `src/index.css`, never fixed colours.
- The installed web app has no reload button and no pull-to-refresh. It must refresh its own data and handle a `401` by itself.
- Phones get the bottom tab bar; from the `md` breakpoint up the same items are a sidebar (`src/components/app-shell.tsx`).
- List a new dependency under **Assumptions** in the PR body.
