import { useEffect, useRef, type RefObject } from 'react';

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function useModalFocus<T extends HTMLElement>(
  open: boolean,
  onClose: () => void,
): RefObject<T | null> {
  const ref = useRef<T>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const element = ref.current;
    const first = element?.querySelector<HTMLElement>(FOCUSABLE);
    first?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== 'Tab' || !element) return;
      const items = [...element.querySelectorAll<HTMLElement>(FOCUSABLE)];
      if (items.length === 0) return;
      const index = items.indexOf(document.activeElement as HTMLElement);
      const next = focusTargetIndex(index, items.length, event.shiftKey);
      event.preventDefault();
      items[next]?.focus();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      previous?.focus();
    };
  }, [open]);
  return ref;
}

export function focusTargetIndex(current: number, count: number, reverse: boolean): number {
  if (count <= 0) return -1;
  if (reverse) return current <= 0 ? count - 1 : current - 1;
  return current < 0 || current >= count - 1 ? 0 : current + 1;
}
