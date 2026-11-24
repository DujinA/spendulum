import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useApi } from "@/api/context";
import { deleteEntry, invalidateEntries, restoreEntry } from "@/api/entries";

// Undo is offered only while the toast shows (#2); afterwards a deleted entry cannot be restored.
export const undoToastDuration = 8000;

const undoFailed = "Could not undo. Check your connection and try again.";

export function useUndoToasts() {
  const api = useApi();
  const queryClient = useQueryClient();

  // The toast stays until Undo succeeds, and a failure shows in it, so Undo can be tapped again while it shows.
  function offerUndo(message: string, undo: () => Promise<unknown>) {
    let pending = false;
    let open = true;
    const closed = () => {
      open = false;
    };
    const show = (id?: string | number, description?: string): string | number =>
      toast(message, {
        ...(id !== undefined && { id }),
        ...(description !== undefined && { description }),
        duration: undoToastDuration,
        onDismiss: closed,
        onAutoClose: closed,
        action: {
          label: "Undo",
          onClick: (event) => {
            event.preventDefault();
            if (pending) return;
            pending = true;
            undo().then(
              () => {
                toast.dismiss(shown);
                return invalidateEntries(queryClient);
              },
              () => {
                pending = false;
                // A closed toast never comes back: after it, nothing restores the entry.
                if (open) show(shown, undoFailed);
                else toast.error(undoFailed);
              },
            );
          },
        },
      });
    const shown = show();
  }

  return {
    saved(id: string, message: string) {
      offerUndo(message, () => deleteEntry(api, id));
    },
    deleted(id: string) {
      offerUndo("Deleted", () => restoreEntry(api, id));
    },
  };
}
