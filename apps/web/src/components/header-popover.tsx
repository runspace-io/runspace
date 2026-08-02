'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';

/**
 * A small dropdown anchored to a header button: no docked width cost when
 * closed, closes on an outside click or Escape, stays open across clicks on
 * its own contents (so multi-select lists and connect-dialog launches work).
 */
export function HeaderPopover({
  title,
  panelLabel,
  trigger,
  children,
}: {
  /** Static hover title AND accessible name, kept independent of the
   * dynamic visual label (a repository or agent name) so it can't collide
   * with unrelated accessible names elsewhere on the page. */
  title: string;
  panelLabel: string;
  trigger: (open: boolean) => ReactNode;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  return (
    <div className="header-popover" ref={rootRef}>
      <button
        type="button"
        className="header-popover-trigger"
        aria-expanded={open}
        aria-label={title}
        title={title}
        onClick={() => setOpen((value) => !value)}
      >
        {trigger(open)}
      </button>
      {open && (
        <div className="header-popover-panel" role="group" aria-label={panelLabel}>
          {children}
        </div>
      )}
    </div>
  );
}
