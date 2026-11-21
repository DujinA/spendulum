import { useMutation, usePrefetchQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useCanGoBack, useNavigate, useRouter } from "@tanstack/react-router";
import { Check, Delete, Undo2 } from "lucide-react";
import { useEffect, useEffectEvent, useRef, useState, type ReactNode } from "react";

import { ApiError } from "@/api/client";
import { useApi } from "@/api/context";
import {
  categoriesQuery,
  createEntry,
  entriesQueryKey,
  settingsQuery,
  type Category,
  type Kind,
  type NewEntry,
} from "@/api/entries";
import { vendorsQuery, vendorsQueryKey, type Vendor } from "@/api/vendors";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Input } from "@/components/ui/input";
import { VendorPicker } from "@/components/vendor-picker";
import { addDays, dayLabel, todayIn } from "@/lib/dates";
import { currencyLabel, formatTyped, isAmountKey, pressKey, toMinorUnits } from "@/lib/money";
import { cn } from "@/lib/utils";

type Sheet = "vendor" | "category" | "date" | "note";

const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", ".", "0", "Backspace"] as const;

function isTextField(target: EventTarget | null) {
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement ||
    (target instanceof HTMLElement && target.isContentEditable)
  );
}

function isControl(target: EventTarget | null) {
  return target instanceof HTMLElement && target.closest("button, a") !== null;
}

function saveError(error: Error) {
  if (error instanceof ApiError && error.status === 422) return error.message;
  return "Could not save. Check your connection and try again.";
}

export function NewEntryScreen() {
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const router = useRouter();
  const canGoBack = useCanGoBack();
  const { data: settings } = useSuspenseQuery(settingsQuery(api));
  const { data: categories } = useSuspenseQuery(categoriesQuery(api));

  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [today] = useState(() => todayIn(settings.timeZone));
  const [kind, setKind] = useState<Kind>("expense");
  const [typed, setTyped] = useState("");
  const [refund, setRefund] = useState(false);
  const [vendor, setVendor] = useState<{ name: string; isNew: boolean }>();
  const [categoryId, setCategoryId] = useState<string>();
  // A category the user picked is never replaced; one from the vendor's latest entry shows "last time".
  const [categoryFrom, setCategoryFrom] = useState<"user" | "vendor">();
  const [entryDate, setEntryDate] = useState(today);
  const [note, setNote] = useState("");
  const [sheet, setSheet] = useState<Sheet>();
  // Only a control reached with Tab keeps its own Enter; after a tap or click, Enter saves.
  const focusedByTab = useRef(false);

  const currency = settings.primaryCurrency;
  const vendorLabel = kind === "income" ? "Source" : "Vendor";
  const magnitude = toMinorUnits(typed, currency);
  const kindCategories = categories.filter((c) => c.kind === kind);
  const category = kindCategories.find((c) => c.id === categoryId);
  const draft: NewEntry | undefined =
    magnitude > 0 && category
      ? {
          kind,
          amount: refund ? -magnitude : magnitude,
          categoryId: category.id,
          entryDate,
          note: note.trim(),
          ...(vendor && { vendorName: vendor.name }),
        }
      : undefined;

  usePrefetchQuery(vendorsQuery(api, kind, ""));

  const save = useMutation({
    mutationFn: (body: NewEntry) => createEntry(api, idempotencyKey, body),
    onSuccess: async () => {
      void queryClient.invalidateQueries({ queryKey: vendorsQueryKey });
      await queryClient.resetQueries({ queryKey: entriesQueryKey });
      await navigate({ to: "/entries", replace: true });
    },
  });

  function submit() {
    if (draft && !save.isPending) save.mutate(draft);
  }

  function switchKind(next: Kind) {
    if (next === kind) return;
    setKind(next);
    setCategoryId(undefined);
    setCategoryFrom(undefined);
    setRefund(false);
  }

  function forgetPrefill() {
    setCategoryFrom((from) => (from === "vendor" ? undefined : from));
  }

  function pickVendor(picked: Vendor) {
    const { latest } = picked;
    setVendor({ name: picked.name, isNew: false });
    if (categoryFrom === "user") return;
    if (latest && kindCategories.some((c) => c.id === latest.categoryId)) {
      setCategoryId(latest.categoryId);
      setCategoryFrom("vendor");
    } else {
      forgetPrefill();
    }
  }

  function toggleRefund() {
    if (kind === "expense") setRefund((on) => !on);
  }

  function cancel() {
    if (canGoBack) router.history.back();
    else void navigate({ to: "/entries" });
  }

  const onKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.key === "Tab") focusedByTab.current = true;
    if (sheet || event.ctrlKey || event.metaKey || event.altKey || isTextField(event.target)) return;

    if (isAmountKey(event.key)) {
      event.preventDefault();
      setTyped((t) => pressKey(t, event.key, currency));
    } else if (event.key === "-") {
      event.preventDefault();
      toggleRefund();
    } else if (event.key === "Enter" && !(focusedByTab.current && isControl(event.target))) {
      event.preventDefault();
      submit();
    }
  });

  useEffect(() => {
    const onPointerDown = () => {
      focusedByTab.current = false;
    };
    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("pointerdown", onPointerDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("pointerdown", onPointerDown);
    };
  }, []);

  return (
    <section className="flex min-h-0 flex-1 flex-col gap-3 md:mx-auto md:w-full md:max-w-sm md:flex-none">
      <h1 className="sr-only">Add entry</h1>
      <header className="grid grid-cols-[1fr_auto_1fr] items-center">
        <Button variant="ghost" className="text-primary h-11 justify-self-start px-2" onClick={cancel}>
          Cancel
        </Button>
        <KindSwitch kind={kind} onChange={switchKind} />
      </header>

      <div className="flex flex-1 flex-col items-center justify-center gap-3 py-2">
        <output
          aria-label="Amount"
          className={cn(
            "flex items-baseline gap-2 text-5xl font-light tracking-tight tabular-nums",
            !typed && "text-muted-foreground",
            refund && "text-refund",
            kind === "income" && "text-income",
          )}
        >
          <span>
            {refund && "−"}
            {formatTyped(typed)}
          </span>
          <span className="text-muted-foreground text-xl font-medium">{currencyLabel(currency)}</span>
        </output>
        {kind === "expense" && (
          <button
            type="button"
            aria-pressed={refund}
            onClick={toggleRefund}
            className={cn(
              "flex h-11 items-center gap-1.5 rounded-full border px-4 text-sm",
              refund ? "border-refund text-refund font-semibold" : "text-muted-foreground",
            )}
          >
            <Undo2 className="size-4" aria-hidden />
            Refund
          </button>
        )}
      </div>

      <div className="grid grid-cols-2 gap-2">
        <Tile
          label={vendorLabel}
          value={vendor?.name}
          hint={vendor?.isNew ? "new" : undefined}
          placeholder="Optional"
          onClick={() => { setSheet("vendor"); }}
        />
        <Tile
          label="Category"
          value={category?.name}
          hint={category && categoryFrom === "vendor" ? "last time" : undefined}
          placeholder="Pick one"
          onClick={() => { setSheet("category"); }}
        />
        <Tile label="Date" value={dayLabel(entryDate, today)} onClick={() => { setSheet("date"); }} />
        <Tile label="Note" value={note.trim() || undefined} placeholder="Optional" onClick={() => { setSheet("note"); }} />
      </div>

      {save.error && (
        <p role="alert" className="text-destructive text-sm">
          {saveError(save.error)}
        </p>
      )}

      <Keypad
        onKey={(key) => {
          setTyped((t) => pressKey(t, key, currency));
        }}
      />

      <Button className="h-12 text-base" disabled={!draft || save.isPending} onClick={submit}>
        Save
      </Button>

      <EntrySheet title={vendorLabel} open={sheet === "vendor"} onClose={() => { setSheet(undefined); }}>
        <VendorPicker
          kind={kind}
          label={vendorLabel}
          primaryCurrency={settings.primaryCurrency}
          initialName={vendor?.name ?? ""}
          onPick={(picked) => {
            pickVendor(picked);
            setSheet(undefined);
          }}
          onCreate={(name) => {
            setVendor({ name, isNew: true });
            forgetPrefill();
            setSheet(undefined);
          }}
          onLeaveEmpty={() => {
            setVendor(undefined);
            forgetPrefill();
            setSheet(undefined);
          }}
        />
      </EntrySheet>
      <EntrySheet title="Category" open={sheet === "category"} onClose={() => { setSheet(undefined); }}>
        <CategoryList
          categories={kindCategories}
          selected={category?.id}
          onPick={(id) => {
            setCategoryId(id);
            setCategoryFrom("user");
            setSheet(undefined);
          }}
        />
      </EntrySheet>
      <EntrySheet title="Date" open={sheet === "date"} onClose={() => { setSheet(undefined); }}>
        <DatePicker
          today={today}
          value={entryDate}
          onPick={(date) => {
            setEntryDate(date);
            setSheet(undefined);
          }}
          onChange={setEntryDate}
        />
      </EntrySheet>
      <EntrySheet title="Note" open={sheet === "note"} onClose={() => { setSheet(undefined); }}>
        <form
          className="px-4 pb-4"
          onSubmit={(event) => {
            event.preventDefault();
            setSheet(undefined);
          }}
        >
          <Input
            aria-label="Note"
            autoFocus
            enterKeyHint="done"
            placeholder="Optional note"
            className="h-11"
            value={note}
            onChange={(event) => {
              setNote(event.target.value);
            }}
          />
        </form>
      </EntrySheet>
    </section>
  );
}

function KindSwitch({ kind, onChange }: { kind: Kind; onChange: (kind: Kind) => void }) {
  const options: { value: Kind; label: string }[] = [
    { value: "expense", label: "Expense" },
    { value: "income", label: "Income" },
  ];
  return (
    <div role="radiogroup" aria-label="Kind" className="bg-muted flex rounded-lg p-1">
      {options.map(({ value, label }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={kind === value}
          onClick={() => {
            onChange(value);
          }}
          className={cn(
            "text-muted-foreground h-9 rounded-md px-4 text-sm font-medium",
            kind === value && "bg-background text-foreground shadow-sm",
            kind === value && value === "income" && "text-income",
          )}
        >
          {label}
        </button>
      ))}
    </div>
  );
}

function Tile({
  label,
  value,
  hint,
  placeholder,
  onClick,
}: {
  label: string;
  value: string | undefined;
  hint?: string | undefined;
  placeholder?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="bg-muted flex min-h-14 min-w-0 flex-col items-start justify-center rounded-xl px-3 py-2 text-left"
    >
      <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
        {label}
        {hint && <span className="text-primary tracking-normal normal-case"> · {hint}</span>}
      </span>
      <span className={cn("w-full truncate font-medium", !value && "text-muted-foreground font-normal")}>
        {value ?? placeholder}
      </span>
    </button>
  );
}

// The keys never take focus, so Enter on a physical keyboard still saves after a click.
function Keypad({ onKey }: { onKey: (key: string) => void }) {
  return (
    <div role="group" aria-label="Keypad" className="grid grid-cols-3 border-t">
      {keys.map((key) => (
        <button
          key={key}
          type="button"
          tabIndex={-1}
          aria-label={key === "." ? "Decimal point" : key === "Backspace" ? "Delete" : undefined}
          onMouseDown={(event) => {
            event.preventDefault();
          }}
          onClick={() => {
            onKey(key);
          }}
          className="active:bg-muted flex h-13 items-center justify-center border-b text-2xl [&:not(:nth-child(3n))]:border-r"
        >
          {key === "Backspace" ? <Delete className="size-6" aria-hidden /> : key}
        </button>
      ))}
    </div>
  );
}

function EntrySheet({
  title,
  open,
  onClose,
  children,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <Drawer
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      <DrawerContent aria-describedby={undefined} className="pb-[env(safe-area-inset-bottom)] md:mx-auto md:max-w-md">
        <div className="flex items-center justify-between px-4 pt-2 pb-1">
          <DrawerTitle className="text-base">{title}</DrawerTitle>
          <Button variant="ghost" className="text-primary h-11 px-2" onClick={onClose}>
            Done
          </Button>
        </div>
        <div className="overflow-y-auto pb-2">{children}</div>
      </DrawerContent>
    </Drawer>
  );
}

function CategoryList({
  categories,
  selected,
  onPick,
}: {
  categories: Category[];
  selected: string | undefined;
  onPick: (id: string) => void;
}) {
  return (
    <ul>
      {categories.map((c) => (
        <li key={c.id}>
          <button
            type="button"
            aria-pressed={c.id === selected}
            onClick={() => {
              onPick(c.id);
            }}
            className={cn(
              "flex h-12 w-full items-center justify-between border-b px-4 text-left",
              c.id === selected && "text-primary font-semibold",
            )}
          >
            {c.name}
            {c.id === selected && <Check className="size-4" aria-hidden />}
          </button>
        </li>
      ))}
    </ul>
  );
}

function DatePicker({
  today,
  value,
  onPick,
  onChange,
}: {
  today: string;
  value: string;
  onPick: (date: string) => void;
  onChange: (date: string) => void;
}) {
  const shortcuts = [today, addDays(today, -1), addDays(today, -2)];
  return (
    <div className="flex flex-col gap-4 px-4 pb-4">
      <div className="flex flex-wrap gap-2">
        {shortcuts.map((date) => (
          <button
            key={date}
            type="button"
            aria-pressed={date === value}
            onClick={() => {
              onPick(date);
            }}
            className={cn(
              "bg-muted h-11 rounded-full px-4 text-sm",
              date === value && "bg-primary text-primary-foreground font-semibold",
            )}
          >
            {dayLabel(date, today)}
          </button>
        ))}
      </div>
      <Input
        type="date"
        aria-label="Pick a date"
        max={today}
        className="h-11"
        value={value}
        onChange={(event) => {
          if (event.target.value) onChange(event.target.value);
        }}
      />
    </div>
  );
}
