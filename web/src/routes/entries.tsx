import { useMutation, useQueryClient, useSuspenseInfiniteQuery, useSuspenseQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { Ellipsis, Repeat2, RotateCw, Trash2 } from "lucide-react";
import { useEffect, useEffectEvent, useId, useRef, useState, type PointerEvent, type MouseEvent } from "react";
import { toast } from "sonner";

import { useApi } from "@/api/context";
import { deleteFailed } from "@/api/client";
import {
  currenciesQuery,
  deleteEntry,
  entriesQuery,
  entriesQueryKey,
  invalidateEntries,
  settingsQuery,
  type Entry,
} from "@/api/entries";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { dayLabel, todayIn } from "@/lib/dates";
import { findCurrency, formatMoney, type Currency } from "@/lib/money";
import { useUndoToasts } from "@/lib/undo-toasts";
import { cn } from "@/lib/utils";

interface Day {
  date: string;
  entries: Entry[];
}

// The API lists newest entry date first, so one pass groups each date's entries together.
function byDay(entries: Entry[]) {
  const days: Day[] = [];
  for (const entry of entries) {
    const last = days.at(-1);
    if (last?.date === entry.entryDate) last.entries.push(entry);
    else days.push({ date: entry.entryDate, entries: [entry] });
  }
  return days;
}

function useWhenVisible(onVisible: () => void, enabled: boolean) {
  const ref = useRef<HTMLDivElement>(null);
  const callback = useEffectEvent(onVisible);

  useEffect(() => {
    const element = ref.current;
    if (!element || !enabled) return;
    const observer = new IntersectionObserver(
      (records) => {
        if (records.some((r) => r.isIntersecting)) callback();
      },
      { rootMargin: "400px" },
    );
    observer.observe(element);
    return () => {
      observer.disconnect();
    };
  }, [enabled]);

  return ref;
}

// The entry leaves the list before the server answers; a failed delete brings it back.
function useDeleteEntry() {
  const api = useApi();
  const queryClient = useQueryClient();
  const undoToasts = useUndoToasts();

  const listKey = entriesQuery(api).queryKey;

  return useMutation({
    mutationFn: (entry: Entry) => deleteEntry(api, entry.id),
    onMutate: async (entry) => {
      await queryClient.cancelQueries({ queryKey: entriesQueryKey });
      const pages = queryClient.getQueryData(listKey)?.pages ?? [];
      const page = pages.findIndex((p) => p.entries.some((e) => e.id === entry.id));
      const index = pages[page]?.entries.findIndex((e) => e.id === entry.id) ?? -1;
      queryClient.setQueryData(listKey, (data) =>
        data && { ...data, pages: data.pages.map((p) => ({ ...p, entries: p.entries.filter((e) => e.id !== entry.id) })) },
      );
      return { page, index };
    },
    onSuccess: (_, entry) => {
      undoToasts.deleted(entry.id);
    },
    // Puts the row back where it was, since the list may not reload either.
    onError: (_, entry, at) => {
      toast.error(deleteFailed);
      if (!at || at.index < 0) return;
      queryClient.setQueryData(listKey, (data) =>
        data && {
          ...data,
          pages: data.pages.map((p, i) =>
            i === at.page && !p.entries.some((e) => e.id === entry.id) ? { ...p, entries: p.entries.toSpliced(at.index, 0, entry) } : p,
          ),
        },
      );
    },
    onSettled: () => invalidateEntries(queryClient),
  });
}

interface RowActions {
  openRowId: string | undefined;
  onSwipe: (id: string | undefined) => void;
  onRepeat: (entry: Entry) => void;
  onDelete: (entry: Entry) => void;
}

export function EntriesScreen() {
  const api = useApi();
  const navigate = useNavigate();
  const { data: settings } = useSuspenseQuery(settingsQuery(api));
  const { data: currencies } = useSuspenseQuery(currenciesQuery(api));
  const entries = useSuspenseInfiniteQuery(entriesQuery(api));
  const remove = useDeleteEntry();
  const [openRowId, setOpenRowId] = useState<string>();
  const actions: RowActions = {
    openRowId,
    onSwipe: setOpenRowId,
    onRepeat: (entry) => {
      void navigate({ to: "/new", search: { repeat: entry.id } });
    },
    onDelete: (entry) => {
      setOpenRowId(undefined);
      remove.mutate(entry);
    },
  };
  const today = todayIn(settings.timeZone);
  const days = byDay(entries.data.pages.flatMap((page) => page.entries));
  const sentinel = useWhenVisible(() => {
    void entries.fetchNextPage();
  }, entries.hasNextPage && !entries.isFetching && !entries.isFetchNextPageError);

  return (
    <section className="flex flex-col gap-4 md:max-w-2xl">
      <header className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Entries</h1>
        <Button
          variant="ghost"
          size="icon"
          className="size-11"
          aria-label="Refresh"
          disabled={entries.isRefetching}
          onClick={() => {
            void entries.refetch();
          }}
        >
          <RotateCw className={cn("size-5", entries.isRefetching && "animate-spin")} aria-hidden />
        </Button>
      </header>

      {days.length === 0 && <p className="text-muted-foreground">No entries yet. Tap + to record one.</p>}

      {days.map((day) => (
        <DaySection
          key={day.date}
          label={dayLabel(day.date, today)}
          entries={day.entries}
          currencies={currencies}
          actions={actions}
        />
      ))}

      <div ref={sentinel} />
      {entries.isFetchingNextPage && <p className="text-muted-foreground text-center text-sm">Loading more…</p>}
      {entries.isFetchNextPageError && (
        <Button
          variant="outline"
          className="h-11 self-center"
          onClick={() => {
            void entries.fetchNextPage();
          }}
        >
          Could not load more. Try again
        </Button>
      )}
    </section>
  );
}

function DaySection({
  label,
  entries,
  currencies,
  actions,
}: {
  label: string;
  entries: Entry[];
  currencies: Currency[];
  actions: RowActions;
}) {
  const id = useId();
  return (
    <section aria-labelledby={id}>
      <h2 id={id} className="text-muted-foreground pb-1 text-xs font-semibold tracking-wide uppercase">
        {label}
      </h2>
      <ul>
        {entries.map((entry) => (
          <EntryRow key={entry.id} entry={entry} currency={findCurrency(currencies, entry.currency)} actions={actions} />
        ))}
      </ul>
    </section>
  );
}

const swipeActionsWidth = 160;

// A horizontal touch drag slides the row open over its actions; a vertical one stays a scroll (touch-action: pan-y).
function useSwipe(open: boolean, onOpenChange: (open: boolean) => void) {
  const gesture = useRef<{ x: number; y: number; from: number; swiping: boolean }>(null);
  const swallowClick = useRef(false);
  const [dragged, setDragged] = useState<number>();
  const rest = open ? -swipeActionsWidth : 0;
  const offsetAt = (from: number, dx: number) => Math.min(0, Math.max(-swipeActionsWidth, from + dx));

  return {
    offset: dragged ?? rest,
    dragging: dragged !== undefined,
    handlers: {
      onPointerDown(event: PointerEvent<HTMLElement>) {
        swallowClick.current = false;
        if (event.pointerType === "mouse") return;
        gesture.current = { x: event.clientX, y: event.clientY, from: rest, swiping: false };
      },
      onPointerMove(event: PointerEvent<HTMLElement>) {
        const g = gesture.current;
        if (!g) return;
        const dx = event.clientX - g.x;
        if (!g.swiping) {
          if (Math.abs(dx) < 8) return;
          if (Math.abs(event.clientY - g.y) > Math.abs(dx)) {
            gesture.current = null;
            return;
          }
          g.swiping = true;
          event.currentTarget.setPointerCapture(event.pointerId);
        }
        setDragged(offsetAt(g.from, dx));
      },
      onPointerUp(event: PointerEvent<HTMLElement>) {
        const g = gesture.current;
        gesture.current = null;
        if (!g?.swiping) return;
        swallowClick.current = true;
        setDragged(undefined);
        onOpenChange(offsetAt(g.from, event.clientX - g.x) < -swipeActionsWidth / 2);
      },
      onPointerCancel() {
        gesture.current = null;
        setDragged(undefined);
      },
      // A swipe ends with a click on the link under it, and a tap on an open row only closes it.
      onClickCapture(event: MouseEvent<HTMLElement>) {
        if (!swallowClick.current && !open) return;
        event.preventDefault();
        event.stopPropagation();
        if (!swallowClick.current) onOpenChange(false);
        swallowClick.current = false;
      },
    },
  };
}

function EntryRow({ entry, currency, actions }: { entry: Entry; currency: Currency; actions: RowActions }) {
  const isRefund = entry.amount < 0;
  const title = entry.vendorName ?? entry.categoryName;
  const details = [entry.vendorName ? entry.categoryName : "", entry.note].filter((part) => part !== "").join(" · ");
  const open = actions.openRowId === entry.id;
  const swipe = useSwipe(open, (next) => {
    actions.onSwipe(next ? entry.id : undefined);
  });

  return (
    <li className="relative overflow-hidden border-b">
      <div className="absolute inset-y-0 right-0 flex" inert={!open} aria-hidden={!open}>
        <SwipeAction
          label="Repeat"
          icon={Repeat2}
          className="bg-primary text-primary-foreground"
          onClick={() => {
            actions.onRepeat(entry);
          }}
        />
        <SwipeAction
          label="Delete"
          icon={Trash2}
          className="bg-destructive text-destructive-foreground"
          onClick={() => {
            actions.onDelete(entry);
          }}
        />
      </div>
      <div
        {...swipe.handlers}
        style={{ transform: `translateX(${String(swipe.offset)}px)` }}
        className={cn("bg-background relative flex touch-pan-y items-center", !swipe.dragging && "transition-transform")}
      >
        <Link
          to="/entries/$entryId"
          params={{ entryId: entry.id }}
          draggable={false}
          className="flex min-h-14 min-w-0 flex-1 items-center gap-3 py-2"
        >
          <div className="min-w-0 flex-1">
            <p className="truncate font-medium">{title}</p>
            {(isRefund || details) && (
              <p className="text-muted-foreground truncate text-sm">
                {isRefund && <span className="text-refund font-medium">Refund</span>}
                {isRefund && details && " · "}
                {details}
              </p>
            )}
          </div>
          <span
            className={cn(
              "shrink-0 font-semibold tabular-nums",
              isRefund && "text-refund",
              entry.kind === "income" && "text-income",
            )}
          >
            {formatMoney(entry.amount, currency)}
          </span>
        </Link>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="text-muted-foreground size-11 shrink-0"
              aria-label={`Actions for ${title}`}
            >
              <Ellipsis className="size-5" aria-hidden />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              className="min-h-11 text-base"
              onSelect={() => {
                actions.onRepeat(entry);
              }}
            >
              <Repeat2 aria-hidden />
              Repeat
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              className="min-h-11 text-base"
              onSelect={() => {
                actions.onDelete(entry);
              }}
            >
              <Trash2 aria-hidden />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </li>
  );
}

function SwipeAction({
  label,
  icon: Icon,
  className,
  onClick,
}: {
  label: string;
  icon: typeof Trash2;
  className: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn("flex w-20 flex-col items-center justify-center gap-1 text-xs font-medium", className)}
    >
      <Icon className="size-5" aria-hidden />
      {label}
    </button>
  );
}
