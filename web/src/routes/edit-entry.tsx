import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { Trash2, Undo2 } from "lucide-react";
import { useId, useState, type ReactNode } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { z } from "zod";

import { deleteFailed, saveError } from "@/api/client";
import { useApi } from "@/api/context";
import {
  categoriesQuery,
  currenciesQuery,
  deleteEntry,
  entriesQueryKey,
  entryQuery,
  settingsQuery,
  updateEntry,
  type Category,
  type Entry,
  type EntryChange,
} from "@/api/entries";
import { vendorLabelFor, vendorsQueryKey } from "@/api/vendors";
import { EntrySheet } from "@/components/entry-sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { VendorPicker } from "@/components/vendor-picker";
import { todayIn } from "@/lib/dates";
import { currencyLabel, findCurrency, parseAmount, toTyped, type Currency } from "@/lib/money";
import { useBackToEntries } from "@/lib/navigation";
import { useUndoToasts } from "@/lib/undo-toasts";
import { cn } from "@/lib/utils";

function entryForm(currency: Currency) {
  const example = currency.minorUnits > 0 ? `1780.${"8".padEnd(currency.minorUnits, "0")}` : "1780";
  return z.object({
    amount: z
      .string()
      .refine((text) => parseAmount(text, currency) !== undefined, `Type an amount like ${example}.`)
      .refine((text) => parseAmount(text, currency) !== 0, "The amount must not be 0."),
    refund: z.boolean(),
    vendorName: z.string(),
    categoryId: z.string(),
    entryDate: z.iso.date("Pick a date."),
    note: z.string(),
  });
}

type EntryForm = z.infer<ReturnType<typeof entryForm>>;

// An archived category stays on its entries, so the entry's own one is offered even when the picker hides it.
function categoryChoices(categories: Category[], entry: Entry) {
  const choices: Pick<Category, "id" | "name">[] = categories.filter((c) => c.kind === entry.kind);
  if (!choices.some((c) => c.id === entry.categoryId)) choices.push({ id: entry.categoryId, name: entry.categoryName });
  return choices;
}


export function EditEntryScreen({ entryId }: { entryId: string }) {
  const api = useApi();
  const queryClient = useQueryClient();
  const backToEntries = useBackToEntries();
  const undoToasts = useUndoToasts();
  const { data: entry } = useSuspenseQuery(entryQuery(api, entryId));
  const { data: settings } = useSuspenseQuery(settingsQuery(api));
  const { data: categories } = useSuspenseQuery(categoriesQuery(api));
  const { data: currencies } = useSuspenseQuery(currenciesQuery(api));
  const currency = findCurrency(currencies, entry.currency);
  const [today] = useState(() => todayIn(settings.timeZone));
  const [vendorSheet, setVendorSheet] = useState(false);
  const titleId = useId();
  const vendorLabel = vendorLabelFor(entry.kind);

  const { control, register, handleSubmit, setValue, formState } = useForm<EntryForm>({
    resolver: zodResolver(entryForm(currency)),
    defaultValues: {
      amount: toTyped(entry.amount, currency),
      refund: entry.amount < 0,
      vendorName: entry.vendorName ?? "",
      categoryId: entry.categoryId,
      entryDate: entry.entryDate,
      note: entry.note,
    },
  });
  const vendorName = useWatch({ control, name: "vendorName" });

  async function showList() {
    void queryClient.invalidateQueries({ queryKey: vendorsQueryKey });
    await queryClient.resetQueries({ queryKey: entriesQueryKey });
    backToEntries();
  }

  const save = useMutation({
    mutationFn: (body: EntryChange) => updateEntry(api, entry.id, body),
    onSuccess: async (updated) => {
      queryClient.setQueryData(entryQuery(api, updated.id).queryKey, updated);
      await showList();
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteEntry(api, entry.id),
    onSuccess: async () => {
      undoToasts.deleted(entry.id);
      await showList();
    },
  });

  const busy = save.isPending || remove.isPending;

  const submit = handleSubmit((values) => {
    const magnitude = parseAmount(values.amount, currency) ?? 0;
    const vendor = values.vendorName.trim();
    save.mutate({
      amount: values.refund && entry.kind === "expense" ? -magnitude : magnitude,
      categoryId: values.categoryId,
      entryDate: values.entryDate,
      note: values.note.trim(),
      ...(vendor && { vendorName: vendor }),
    });
  });

  function pickVendor(name: string) {
    setValue("vendorName", name, { shouldDirty: true });
    setVendorSheet(false);
  }

  return (
    <section className="flex flex-col gap-4 md:max-w-md">
      <header className="grid grid-cols-[1fr_auto_1fr] items-center">
        <Button variant="ghost" className="text-primary h-11 justify-self-start px-2" onClick={backToEntries}>
          Cancel
        </Button>
        <h1 id={titleId} className="text-lg font-semibold">
          Edit {entry.kind}
        </h1>
      </header>

      <form aria-labelledby={titleId} noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        <Field label="Amount" error={formState.errors.amount?.message}>
          {(field) => (
            <div className="flex items-center gap-3">
              <Input
                {...field}
                {...register("amount")}
                inputMode="decimal"
                autoComplete="off"
                className="h-11 text-base tabular-nums"
              />
              <span className="text-muted-foreground shrink-0 font-medium">{currencyLabel(currency.code)}</span>
            </div>
          )}
        </Field>

        {entry.kind === "expense" && (
          <Controller
            control={control}
            name="refund"
            render={({ field: { value, onChange } }) => (
              <button
                type="button"
                aria-pressed={value}
                onClick={() => {
                  onChange(!value);
                }}
                className={cn(
                  "flex h-11 items-center gap-1.5 self-start rounded-full border px-4 text-sm",
                  value ? "border-refund text-refund font-semibold" : "text-muted-foreground",
                )}
              >
                <Undo2 className="size-4" aria-hidden />
                Refund
              </button>
            )}
          />
        )}

        <button
          type="button"
          onClick={() => {
            setVendorSheet(true);
          }}
          className="bg-muted flex min-h-14 min-w-0 flex-col items-start justify-center rounded-xl px-3 py-2 text-left"
        >
          <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">{vendorLabel}</span>
          <span className={cn("w-full truncate font-medium", !vendorName && "text-muted-foreground font-normal")}>
            {vendorName || "Optional"}
          </span>
        </button>

        <Field label="Category">
          {(field) => (
            <select
              {...field}
              {...register("categoryId")}
              className="border-input dark:bg-input/30 h-11 w-full rounded-md border bg-transparent px-3 text-base"
            >
              {categoryChoices(categories, entry).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          )}
        </Field>

        <Field label="Date" error={formState.errors.entryDate?.message}>
          {(field) => <Input {...field} {...register("entryDate")} type="date" max={today} className="h-11" />}
        </Field>

        <Field label="Note">
          {(field) => <Input {...field} {...register("note")} placeholder="Optional note" className="h-11" />}
        </Field>

        {save.error && (
          <p role="alert" className="text-destructive text-sm">
            {saveError(save.error)}
          </p>
        )}
        {remove.error && (
          <p role="alert" className="text-destructive text-sm">
            {deleteFailed}
          </p>
        )}

        <Button type="submit" className="h-12 text-base" disabled={busy}>
          Save
        </Button>
        <Button
          type="button"
          variant="ghost"
          className="text-destructive h-11"
          disabled={busy}
          onClick={() => {
            remove.mutate();
          }}
        >
          <Trash2 className="size-4" aria-hidden />
          Delete
        </Button>
      </form>

      <EntrySheet
        title={vendorLabel}
        open={vendorSheet}
        onClose={() => {
          setVendorSheet(false);
        }}
      >
        <VendorPicker
          kind={entry.kind}
          label={vendorLabel}
          primaryCurrency={settings.primaryCurrency}
          initialName={vendorName}
          open={vendorSheet}
          onPick={(picked) => {
            pickVendor(picked.name);
          }}
          onCreate={pickVendor}
          onLeaveEmpty={() => {
            pickVendor("");
          }}
        />
      </EntrySheet>
    </section>
  );
}

interface FieldProps {
  id: string;
  "aria-invalid": boolean;
  "aria-describedby": string | undefined;
}

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string | undefined;
  children: (field: FieldProps) => ReactNode;
}) {
  const id = useId();
  const errorId = `${id}-error`;
  return (
    <div className="flex flex-col gap-2">
      <Label htmlFor={id}>{label}</Label>
      {children({ id, "aria-invalid": error !== undefined, "aria-describedby": error ? errorId : undefined })}
      {error && (
        <p id={errorId} className="text-destructive text-sm">
          {error}
        </p>
      )}
    </div>
  );
}
