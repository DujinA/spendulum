import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
  type Modifier,
  type UniqueIdentifier,
} from "@dnd-kit/core";
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp, GripVertical, Plus } from "lucide-react";
import { useId, useState, type ReactNode, type SubmitEvent } from "react";

import {
  allCategoriesQuery,
  categoriesQueryKey,
  createCategory,
  deleteCategory,
  reorderCategories,
  updateCategory,
  type CategoryChange,
  type CategoryOrder,
} from "@/api/categories";
import { ApiError } from "@/api/client";
import { useApi } from "@/api/context";
import type { Category, Kind } from "@/api/entries";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

const kinds: { kind: Kind; label: string }[] = [
  { kind: "expense", label: "Expense" },
  { kind: "income", label: "Income" },
];

type Sheet = { mode: "add"; kind: Kind } | { mode: "edit"; category: Category };

// The server answers 409 and 422 with a detail written for the user.
function errorMessage(error: Error, fallback: string) {
  if (error instanceof ApiError && (error.status === 409 || error.status === 422)) return error.message;
  return fallback;
}

function inOrder(list: Category[], { kind, ids }: CategoryOrder) {
  const byId = new Map(list.map((c) => [c.id, c]));
  const all = [...list.filter((c) => c.kind !== kind), ...ids.flatMap((id) => byId.get(id) ?? [])];
  return kinds.flatMap((k) => all.filter((c) => c.kind === k.kind));
}

// The list shows the new order at once and puts the old one back if saving fails.
function useReorder() {
  const api = useApi();
  const queryClient = useQueryClient();
  const { queryKey } = allCategoriesQuery(api);
  const mutationKey = [...categoriesQueryKey, "order"];

  return useMutation({
    mutationKey,
    // One save at a time, so quick moves reach the server in the order they were made.
    scope: { id: "category-order" },
    mutationFn: (order: CategoryOrder) => reorderCategories(api, order),
    onMutate: async (order) => {
      await queryClient.cancelQueries({ queryKey: categoriesQueryKey });
      const previous = queryClient.getQueryData(queryKey);
      queryClient.setQueryData(queryKey, (list) => list && inOrder(list, order));
      return { previous };
    },
    onError: (_error, _order, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: () =>
      queryClient.isMutating({ mutationKey }) === 1
        ? queryClient.invalidateQueries({ queryKey: categoriesQueryKey })
        : undefined,
  });
}

export function CategoriesScreen() {
  const api = useApi();
  const { data: categories } = useSuspenseQuery(allCategoriesQuery(api));
  const reorder = useReorder();
  const [sheet, setSheet] = useState<Sheet>();
  const [sheetOpen, setSheetOpen] = useState(false);
  // A new key per opening resets the sheet's fields and its Idempotency-Key.
  const [openings, setOpenings] = useState(0);

  function show(next: Sheet) {
    setSheet(next);
    setSheetOpen(true);
    setOpenings((n) => n + 1);
  }

  function close() {
    setSheetOpen(false);
  }

  function move(kind: Kind, id: string, toId: string) {
    const ofKind = categories.filter((c) => c.kind === kind);
    const active = ofKind.filter((c) => !c.archived).map((c) => c.id);
    const archived = ofKind.filter((c) => c.archived).map((c) => c.id);
    const ids = arrayMove(active, active.indexOf(id), active.indexOf(toId));
    reorder.mutate({ kind, ids: [...ids, ...archived] });
  }

  return (
    <section className="flex flex-col gap-6 md:max-w-2xl">
      <header className="flex flex-col items-start gap-1">
        <Link to="/more" className="text-primary -ml-1 flex h-11 items-center gap-0.5 text-sm font-medium">
          <ChevronLeft className="size-5" aria-hidden />
          More
        </Link>
        <h1 className="text-2xl font-semibold tracking-tight">Categories</h1>
      </header>

      {reorder.isError && (
        <p role="alert" className="text-destructive text-sm">
          Could not save the order. Check your connection and try again.
        </p>
      )}

      {kinds.map(({ kind, label }) => (
        <KindSection
          key={kind}
          label={label}
          categories={categories.filter((c) => c.kind === kind)}
          onAdd={() => {
            show({ mode: "add", kind });
          }}
          onEdit={(category) => {
            show({ mode: "edit", category });
          }}
          onMove={(id, toId) => {
            move(kind, id, toId);
          }}
        />
      ))}

      <Drawer
        open={sheetOpen}
        onOpenChange={(open) => {
          if (!open) close();
        }}
      >
        <DrawerContent aria-describedby={undefined} className="pb-[env(safe-area-inset-bottom)] md:mx-auto md:max-w-md">
          {sheet?.mode === "add" && <AddCategory key={openings} kind={sheet.kind} onDone={close} />}
          {sheet?.mode === "edit" && <EditCategory key={openings} category={sheet.category} onDone={close} />}
        </DrawerContent>
      </Drawer>
    </section>
  );
}

function KindSection({
  label,
  categories,
  onAdd,
  onEdit,
  onMove,
}: {
  label: string;
  categories: Category[];
  onAdd: () => void;
  onEdit: (category: Category) => void;
  onMove: (id: string, toId: string) => void;
}) {
  const headingId = useId();
  const active = categories.filter((c) => !c.archived);
  const archived = categories.filter((c) => c.archived);
  const sensors = useSensors(useSensor(PointerSensor));
  const nameOf = (id: UniqueIdentifier) => categories.find((c) => c.id === id)?.name ?? "";
  const announcements: Announcements = {
    onDragStart: ({ active: a }) => `Picked up ${nameOf(a.id)}.`,
    onDragOver: ({ active: a, over }) => (over ? `${nameOf(a.id)} is over ${nameOf(over.id)}.` : `${nameOf(a.id)} is over no category.`),
    onDragEnd: ({ active: a, over }) => (over ? `${nameOf(a.id)} was dropped over ${nameOf(over.id)}.` : `${nameOf(a.id)} was dropped.`),
    onDragCancel: ({ active: a }) => `Moving ${nameOf(a.id)} was cancelled.`,
  };

  function onDragEnd({ active: dragged, over }: DragEndEvent) {
    if (over && dragged.id !== over.id) onMove(String(dragged.id), String(over.id));
  }

  return (
    <section aria-labelledby={headingId} className="flex flex-col">
      <div className="flex items-center justify-between">
        <h2 id={headingId} className="text-muted-foreground text-xs font-semibold tracking-wide uppercase">
          {label}
        </h2>
        <Button
          variant="ghost"
          className="text-primary -mr-2 h-11 px-2"
          aria-label={`Add ${label.toLowerCase()} category`}
          onClick={onAdd}
        >
          <Plus aria-hidden />
          Add
        </Button>
      </div>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[verticalOnly]}
        accessibility={{ announcements }}
        onDragEnd={onDragEnd}
      >
        <SortableContext items={active.map((c) => c.id)} strategy={verticalListSortingStrategy}>
          <ul aria-label={`${label} categories`} className="border-t">
            {active.map((category, i) => (
              <SortableRow
                key={category.id}
                category={category}
                above={active[i - 1]?.id}
                below={active[i + 1]?.id}
                onEdit={() => {
                  onEdit(category);
                }}
                onMove={(toId) => {
                  onMove(category.id, toId);
                }}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>

      {archived.length > 0 && (
        <details className="group mt-1">
          <summary className="text-muted-foreground flex h-11 cursor-pointer list-none items-center gap-1 text-sm font-medium [&::-webkit-details-marker]:hidden">
            <ChevronRight className="size-4 transition-transform group-open:rotate-90" aria-hidden />
            Archived ({archived.length})
          </summary>
          <ul aria-label={`Archived ${label.toLowerCase()} categories`} className="border-t">
            {archived.map((category) => (
              <li key={category.id} aria-label={category.name} className="flex items-center border-b">
                <CategoryButton
                  category={category}
                  className="text-muted-foreground pl-11"
                  onClick={() => {
                    onEdit(category);
                  }}
                />
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}

const verticalOnly: Modifier = ({ transform }) => ({ ...transform, x: 0 });

// The handle is for pointer and touch only; keyboard and screen reader users have Move up and Move down.
function SortableRow({
  category,
  above,
  below,
  onEdit,
  onMove,
}: {
  category: Category;
  above: string | undefined;
  below: string | undefined;
  onEdit: () => void;
  onMove: (toId: string) => void;
}) {
  const { listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id,
  });

  return (
    <li
      ref={setNodeRef}
      aria-label={category.name}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn("bg-background flex items-center border-b", isDragging && "relative z-10 shadow-md")}
    >
      <button
        ref={setActivatorNodeRef}
        type="button"
        tabIndex={-1}
        aria-hidden
        className="text-muted-foreground flex size-11 shrink-0 cursor-grab touch-none items-center justify-center active:cursor-grabbing"
        {...listeners}
      >
        <GripVertical className="size-5" />
      </button>
      <CategoryButton category={category} onClick={onEdit} />
      <MoveButton label={`Move ${category.name} up`} toId={above} onMove={onMove}>
        <ChevronUp className="size-5" aria-hidden />
      </MoveButton>
      <MoveButton label={`Move ${category.name} down`} toId={below} onMove={onMove}>
        <ChevronDown className="size-5" aria-hidden />
      </MoveButton>
    </li>
  );
}

function MoveButton({
  label,
  toId,
  onMove,
  children,
}: {
  label: string;
  toId: string | undefined;
  onMove: (toId: string) => void;
  children: ReactNode;
}) {
  return (
    <Button
      variant="ghost"
      size="icon"
      className="size-11"
      aria-label={label}
      disabled={toId === undefined}
      onClick={() => {
        if (toId !== undefined) onMove(toId);
      }}
    >
      {children}
    </Button>
  );
}

function CategoryButton({ category, className, onClick }: { category: Category; className?: string; onClick: () => void }) {
  const noteId = useId();
  return (
    <button
      type="button"
      aria-label={category.name}
      aria-describedby={category.note ? noteId : undefined}
      onClick={onClick}
      className={cn("flex min-h-14 min-w-0 flex-1 flex-col items-start justify-center py-2 text-left", className)}
    >
      <span className="w-full truncate font-medium">{category.name}</span>
      {category.note && (
        <span id={noteId} className="text-muted-foreground w-full truncate text-sm">
          {category.note}
        </span>
      )}
    </button>
  );
}

function SheetHeader({ title, onClose }: { title: string; onClose: () => void }) {
  return (
    <div className="flex items-center justify-between px-4 pt-2 pb-1">
      <DrawerTitle className="text-base">{title}</DrawerTitle>
      <Button variant="ghost" className="text-primary h-11 px-2" onClick={onClose}>
        Cancel
      </Button>
    </div>
  );
}

function NameAndNote({
  name,
  note,
  onName,
  onNote,
}: {
  name: string;
  note: string;
  onName: (name: string) => void;
  onNote: (note: string) => void;
}) {
  const nameId = useId();
  const noteId = useId();
  return (
    <>
      <div className="flex flex-col gap-2">
        <Label htmlFor={nameId}>Name</Label>
        <Input
          id={nameId}
          autoComplete="off"
          className="h-11"
          value={name}
          onChange={(event) => {
            onName(event.target.value);
          }}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={noteId}>Note</Label>
        <Input
          id={noteId}
          autoComplete="off"
          placeholder="Optional"
          className="h-11"
          value={note}
          onChange={(event) => {
            onNote(event.target.value);
          }}
        />
      </div>
    </>
  );
}

function useInvalidateCategories() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: categoriesQueryKey });
}

function AddCategory({ kind, onDone }: { kind: Kind; onDone: () => void }) {
  const api = useApi();
  const invalidate = useInvalidateCategories();
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [name, setName] = useState("");
  const [note, setNote] = useState("");

  const create = useMutation({
    mutationFn: () => createCategory(api, idempotencyKey, { kind, name: name.trim(), note: note.trim() }),
    onSuccess: async () => {
      await invalidate();
      onDone();
    },
  });

  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (name.trim() && !create.isPending) create.mutate();
  }

  return (
    <>
      <SheetHeader title={`New ${kind} category`} onClose={onDone} />
      <form className="flex flex-col gap-4 px-4 pt-2 pb-4" onSubmit={submit}>
        <NameAndNote name={name} note={note} onName={setName} onNote={setNote} />
        {create.error && (
          <p role="alert" className="text-destructive text-sm">
            {errorMessage(create.error, "Could not create the category. Check your connection and try again.")}
          </p>
        )}
        <Button type="submit" className="h-11" disabled={!name.trim() || create.isPending}>
          Create
        </Button>
      </form>
    </>
  );
}

function EditCategory({ category, onDone }: { category: Category; onDone: () => void }) {
  const api = useApi();
  const invalidate = useInvalidateCategories();
  const [name, setName] = useState(category.name);
  const [note, setNote] = useState(category.note);

  const change = useMutation({
    mutationFn: (body: CategoryChange) => updateCategory(api, category.id, body),
    onSuccess: async () => {
      await invalidate();
      onDone();
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteCategory(api, category.id),
    onSuccess: async () => {
      await invalidate();
      onDone();
    },
  });
  const pending = change.isPending || remove.isPending;
  const inUse = remove.error instanceof ApiError && remove.error.status === 409;

  function run(action: () => void) {
    change.reset();
    remove.reset();
    action();
  }

  function archive(archived: boolean) {
    run(() => {
      change.mutate({ archived });
    });
  }

  function save(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (name.trim() && !pending) {
      run(() => {
        change.mutate({ name: name.trim(), note: note.trim() });
      });
    }
  }

  return (
    <>
      <SheetHeader title="Edit category" onClose={onDone} />
      <form className="flex flex-col gap-4 px-4 pt-2 pb-4" onSubmit={save}>
        <p className="text-muted-foreground text-sm">
          {category.kind === "expense" ? "Expense" : "Income"} category{category.archived && ", archived"}
        </p>
        <NameAndNote name={name} note={note} onName={setName} onNote={setNote} />
        {change.error && (
          <p role="alert" className="text-destructive text-sm">
            {errorMessage(change.error, "Could not save. Check your connection and try again.")}
          </p>
        )}
        {remove.error && (
          <div role="alert" className="text-destructive flex flex-col items-start gap-2 text-sm">
            {errorMessage(remove.error, "Could not delete. Check your connection and try again.")}
            {inUse && !category.archived && (
              <Button
                type="button"
                variant="outline"
                className="h-11 text-foreground"
                disabled={pending}
                onClick={() => {
                  archive(true);
                }}
              >
                Archive
              </Button>
            )}
          </div>
        )}
        <Button type="submit" className="h-11" disabled={!name.trim() || pending}>
          Save
        </Button>
        <div className="grid grid-cols-2 gap-2">
          <Button
            type="button"
            variant="outline"
            className="h-11"
            disabled={pending}
            onClick={() => {
              archive(!category.archived);
            }}
          >
            {category.archived ? "Unarchive" : "Archive"}
          </Button>
          <Button
            type="button"
            variant="outline"
            className="text-destructive h-11"
            disabled={pending}
            onClick={() => {
              run(() => {
                remove.mutate();
              });
            }}
          >
            Delete
          </Button>
        </div>
      </form>
    </>
  );
}
