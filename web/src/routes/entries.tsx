import { useSuspenseInfiniteQuery, useSuspenseQuery } from "@tanstack/react-query";
import { RotateCw } from "lucide-react";
import { useEffect, useEffectEvent, useId, useRef } from "react";

import { useApi } from "@/api/context";
import { entriesQuery, settingsQuery, type Entry } from "@/api/entries";
import { Button } from "@/components/ui/button";
import { dayLabel, todayIn } from "@/lib/dates";
import { formatMoney } from "@/lib/money";
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

export function EntriesScreen() {
  const api = useApi();
  const { data: settings } = useSuspenseQuery(settingsQuery(api));
  const entries = useSuspenseInfiniteQuery(entriesQuery(api));
  const today = todayIn(settings.timeZone);
  const days = byDay(entries.data.pages.flatMap((page) => page.entries));
  const sentinel = useWhenVisible(() => {
    void entries.fetchNextPage();
  }, entries.hasNextPage && !entries.isFetchingNextPage && !entries.isFetchNextPageError);

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
        <DaySection key={day.date} label={dayLabel(day.date, today)} entries={day.entries} />
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

function DaySection({ label, entries }: { label: string; entries: Entry[] }) {
  const id = useId();
  return (
    <section aria-labelledby={id}>
      <h2 id={id} className="text-muted-foreground pb-1 text-xs font-semibold tracking-wide uppercase">
        {label}
      </h2>
      <ul>
        {entries.map((entry) => (
          <EntryRow key={entry.id} entry={entry} />
        ))}
      </ul>
    </section>
  );
}

function EntryRow({ entry }: { entry: Entry }) {
  const isRefund = entry.amount < 0;
  return (
    <li className="flex min-h-14 items-center gap-3 border-b py-2">
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{entry.categoryName}</p>
        {(isRefund || entry.note) && (
          <p className="text-muted-foreground truncate text-sm">
            {isRefund && <span className="text-refund font-medium">Refund</span>}
            {isRefund && entry.note && " · "}
            {entry.note}
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
        {formatMoney(entry.amount, entry.currency)}
      </span>
    </li>
  );
}
