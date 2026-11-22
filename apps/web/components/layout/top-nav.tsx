"use client";

import * as React from "react";
import Link from "next/link";
import { Menu, Workflow } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Sidebar } from "./sidebar";

export function TopNav({ actions }: { actions?: React.ReactNode }) {
  const [open, setOpen] = React.useState(false);

  return (
    <header className="sticky top-0 z-40 flex h-14 items-center gap-4 border-b border-border bg-background px-4">
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            aria-label="Open navigation menu"
          >
            <Menu className="h-5 w-5" aria-hidden="true" />
          </Button>
        </DialogTrigger>
        <DialogContent
          className="left-0 top-0 h-full max-w-xs translate-x-0 translate-y-0 rounded-none border-r p-0 sm:rounded-none"
          onClick={() => setOpen(false)}
        >
          <DialogTitle className="sr-only">Navigation</DialogTitle>
          <div className="flex h-14 items-center border-b border-border px-4">
            <BrandMark />
          </div>
          <Sidebar />
        </DialogContent>
      </Dialog>

      <Link
        href="/"
        className="hidden items-center gap-2 font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:flex"
      >
        <BrandMark />
      </Link>

      <div className="ml-auto flex items-center gap-2">{actions}</div>
    </header>
  );
}

function BrandMark() {
  return (
    <span className="flex items-center gap-2">
      <Workflow className="h-5 w-5 text-primary" aria-hidden="true" />
      <span>FlowForge</span>
    </span>
  );
}
