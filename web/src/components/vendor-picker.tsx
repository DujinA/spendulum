import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useId, useState, type ReactNode } from "react";

import { useApi } from "@/api/context";
import type { Kind } from "@/api/entries";
import { vendorsQuery, type Vendor } from "@/api/vendors";
import { Input } from "@/components/ui/input";
import { currencyLabel } from "@/lib/money";

function vendorNote(vendor: Vendor, primaryCurrency: string) {
  const parts = [`${String(vendor.useCount)}×`];
  if (vendor.archived) parts.unshift("Archived");
  if (vendor.latest) {
    parts.push(vendor.latest.categoryName);
    if (vendor.latest.currency !== primaryCurrency) parts.push(currencyLabel(vendor.latest.currency));
  }
  return parts.join(" · ");
}

export function VendorPicker({
  kind,
  label,
  primaryCurrency,
  initialName,
  onPick,
  onCreate,
  onLeaveEmpty,
}: {
  kind: Kind;
  label: string;
  primaryCurrency: string;
  initialName: string;
  onPick: (vendor: Vendor) => void;
  onCreate: (name: string) => void;
  onLeaveEmpty: () => void;
}) {
  const api = useApi();
  const [typed, setTyped] = useState(initialName);
  const name = typed.trim();
  const { data, isFetching, isPlaceholderData } = useQuery({
    ...vendorsQuery(api, kind, name),
    placeholderData: keepPreviousData,
  });
  const didYouMeanId = useId();

  const listed = data ? [...(data.match ? [data.match] : []), ...data.vendors] : [];
  const nearMatches = data?.nearMatches ?? [];
  const offersCreate = name !== "" && data !== undefined && !data.match;

  return (
    <div className="flex flex-col gap-2 pb-2">
      <form
        role="search"
        className="px-4"
        onSubmit={(event) => {
          event.preventDefault();
          if (data?.match) onPick(data.match);
          else if (name) onCreate(name);
        }}
      >
        <Input
          type="search"
          aria-label="Search"
          autoFocus
          autoComplete="off"
          enterKeyHint="done"
          placeholder="Type a name"
          className="h-11"
          value={typed}
          onFocus={(event) => {
            event.currentTarget.select();
          }}
          onChange={(event) => {
            setTyped(event.target.value);
          }}
        />
      </form>

      <section aria-label="Results" aria-busy={isFetching || isPlaceholderData}>
        <button
          type="button"
          onClick={onLeaveEmpty}
          className="text-muted-foreground flex h-12 w-full items-center border-b px-4 text-left"
        >
          Leave {label.toLowerCase()} empty
        </button>

        {listed.length > 0 && (
          <ul aria-label={`${label}s`}>
            {listed.map((vendor) => (
              <VendorRow key={vendor.id} vendor={vendor} primaryCurrency={primaryCurrency} onPick={onPick} />
            ))}
          </ul>
        )}

        {offersCreate && nearMatches.length > 0 && (
          <>
            <p
              id={didYouMeanId}
              className="text-muted-foreground px-4 pt-3 pb-1 text-xs font-semibold tracking-wide uppercase"
            >
              Did you mean …?
            </p>
            <ul aria-labelledby={didYouMeanId}>
              {nearMatches.map((vendor) => (
                <VendorRow key={vendor.id} vendor={vendor} primaryCurrency={primaryCurrency} onPick={onPick} />
              ))}
              <li>
                <CreateButton
                  onClick={() => {
                    onCreate(name);
                  }}
                >
                  No, create «{name}»
                </CreateButton>
              </li>
            </ul>
          </>
        )}

        {offersCreate && nearMatches.length === 0 && (
          <CreateButton
            onClick={() => {
              onCreate(name);
            }}
          >
            Create "{name}"
          </CreateButton>
        )}
      </section>
    </div>
  );
}

function VendorRow({
  vendor,
  primaryCurrency,
  onPick,
}: {
  vendor: Vendor;
  primaryCurrency: string;
  onPick: (vendor: Vendor) => void;
}) {
  const noteId = useId();
  return (
    <li>
      <button
        type="button"
        aria-label={vendor.name}
        aria-describedby={noteId}
        onClick={() => {
          onPick(vendor);
        }}
        className="flex min-h-12 w-full items-center justify-between gap-3 border-b px-4 py-2 text-left"
      >
        <span className="min-w-0 truncate">{vendor.name}</span>
        <span id={noteId} className="text-muted-foreground shrink-0 text-sm tabular-nums">
          {vendorNote(vendor, primaryCurrency)}
        </span>
      </button>
    </li>
  );
}

function CreateButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="text-primary flex min-h-12 w-full items-center border-b px-4 py-2 text-left font-semibold"
    >
      <span className="min-w-0 break-words">{children}</span>
    </button>
  );
}
