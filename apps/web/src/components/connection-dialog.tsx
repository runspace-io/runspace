'use client';

import type { ReactNode } from 'react';
import { X } from 'lucide-react';
import { useModalFocus } from './use-modal-focus';

export type ConnectionMethod<T extends string> = {
  id: T;
  label: string;
  description: string;
  icon: ReactNode;
};

export function ConnectionDialog({
  eyebrow,
  title,
  description,
  icon,
  children,
  onClose,
}: {
  eyebrow: string;
  title: string;
  description: string;
  icon: ReactNode;
  children: ReactNode;
  onClose: () => void;
}) {
  const dialogRef = useModalFocus(true, onClose);
  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <section
        ref={dialogRef}
        className="workspace-dialog connection-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="connection-dialog-title"
      >
        <div className="dialog-header">
          <div>
            <p className="eyebrow">{eyebrow}</p>
            <h2 id="connection-dialog-title">{title}</h2>
            <p className="connection-dialog-copy">{description}</p>
          </div>
          <button className="icon-button" aria-label="Close connection dialog" onClick={onClose}>
            <X size={17} />
          </button>
        </div>
        <div className="connection-dialog-mark" aria-hidden="true">
          {icon}
        </div>
        {children}
      </section>
    </div>
  );
}

export function ConnectionMethodPicker<T extends string>({
  label,
  methods,
  value,
  onChange,
}: {
  label: string;
  methods: readonly ConnectionMethod<T>[];
  value: T;
  onChange: (method: T) => void;
}) {
  return (
    <fieldset className="connection-method-picker">
      <legend>{label}</legend>
      <div className="connection-method-grid">
        {methods.map((method) => (
          <button
            type="button"
            className={`connection-method ${value === method.id ? 'is-selected' : ''}`}
            aria-pressed={value === method.id}
            key={method.id}
            onClick={() => onChange(method.id)}
          >
            <span>{method.icon}</span>
            <strong>{method.label}</strong>
            <small>{method.description}</small>
          </button>
        ))}
      </div>
    </fieldset>
  );
}
