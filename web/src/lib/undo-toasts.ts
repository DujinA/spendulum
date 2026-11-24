import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useApi } from "@/api/context";
import { deleteEntry, entriesQueryKey, restoreEntry } from "@/api/entries";
import { vendorsQueryKey } from "@/api/vendors";

// Undo is offered only while the toast shows (#2); afterwards a deleted entry cannot be restored.
export const undoToastDuration = 8000;

export function useUndoToasts() {
  const api = useApi();
  const queryClient = useQueryClient();

  function offerUndo(message: string, undo: () => Promise<unknown>) {
    toast(message, {
      duration: undoToastDuration,
      action: {
        label: "Undo",
        onClick: () => {
          undo().then(
            () =>
              Promise.all([
                queryClient.invalidateQueries({ queryKey: entriesQueryKey }),
                queryClient.invalidateQueries({ queryKey: vendorsQueryKey }),
              ]),
            () => toast.error("Could not undo. Check your connection and try again."),
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
