import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useApi } from "@/api/context";
import { deleteEntry, invalidateEntries, restoreEntry } from "@/api/entries";

// Undo is offered only while the toast shows (#2); afterwards a deleted entry cannot be restored.
export const undoToastDuration = 8000;

export function useUndoToasts() {
  const api = useApi();
  const queryClient = useQueryClient();

  // The toast stays until Undo succeeds, so a failed Undo can be tapped again while it shows.
  function offerUndo(message: string, undo: () => Promise<unknown>) {
    let pending = false;
    const id = toast(message, {
      duration: undoToastDuration,
      action: {
        label: "Undo",
        onClick: (event) => {
          event.preventDefault();
          if (pending) return;
          pending = true;
          undo().then(
            () => {
              toast.dismiss(id);
              return invalidateEntries(queryClient);
            },
            () => {
              pending = false;
              toast.error("Could not undo. Check your connection and try again.");
            },
          );
        },
      },
    });
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
