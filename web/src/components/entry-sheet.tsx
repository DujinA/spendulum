import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";

export function EntrySheet({
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
