'use client';

import { useId, useState, type ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';

/**
 * A form section that starts collapsed and reveals its fields on demand.
 *
 * Channel creation used to show every field at once — name, resources, agent
 * runtime, an ACP command — before a first-time user had done anything. Most
 * of those are optional and rarely touched on the first pass; collapsing them
 * behind a one-line summary means the only thing demanding attention up front
 * is the one field that's actually required.
 */
export function DisclosureSection({
  label,
  summary,
  defaultOpen = false,
  children,
}: {
  label: string;
  summary: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const panelID = useId();
  return (
    <div className={`disclosure-section ${open ? 'is-open' : ''}`}>
      <button
        type="button"
        className="disclosure-trigger"
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => setOpen((current) => !current)}
      >
        <ChevronRight size={14} className="disclosure-chevron" />
        <span className="disclosure-label">{label}</span>
        {!open && <span className="disclosure-summary">{summary}</span>}
      </button>
      {open && (
        <div id={panelID} className="disclosure-panel">
          {children}
        </div>
      )}
    </div>
  );
}
