"use client";

import * as React from "react";

import { useEditorStore } from "@/lib/editor/store";

function isTextField(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement ||
    target.isContentEditable
  );
}

/**
 * Editor keyboard shortcuts. Never fires while focus is in a text field,
 * so inline label editing and form inputs are never hijacked.
 *
 * Delete/Backspace: delete selection · Ctrl/Cmd+C/V/D: copy/paste/duplicate
 * Ctrl/Cmd+A: select all · Ctrl/Cmd+Z: undo · Ctrl/Cmd+Shift+Z or Ctrl+Y: redo
 */
export function useEditorKeyboard(enabled = true) {
  React.useEffect(() => {
    if (!enabled) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (isTextField(event.target)) return;
      const s = useEditorStore.getState();
      const mod = event.metaKey || event.ctrlKey;
      const key = event.key.toLowerCase();

      if (key === "delete" || key === "backspace") {
        event.preventDefault();
        s.deleteSelection();
        return;
      }
      if (!mod) return;

      if (key === "z" && !event.shiftKey) {
        event.preventDefault();
        s.undo();
      } else if ((key === "z" && event.shiftKey) || key === "y") {
        event.preventDefault();
        s.redo();
      } else if (key === "c") {
        s.copySelection();
      } else if (key === "v") {
        event.preventDefault();
        s.pasteClipboard();
      } else if (key === "d") {
        event.preventDefault();
        s.duplicateSelection();
      } else if (key === "a") {
        event.preventDefault();
        s.selectAll();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [enabled]);
}
