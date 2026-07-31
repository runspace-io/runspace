import type { Dispatch, SetStateAction } from 'react';
import type { ApiRun } from '../lib/api-types';

const terminalStatuses = new Set<ApiRun['status']>(['succeeded', 'failed', 'cancelled']);

export function setRunMonotonic(
  setRun: Dispatch<SetStateAction<ApiRun | undefined>>,
  next: ApiRun,
) {
  setRun((current) => {
    if (current?.id === next.id && terminalStatuses.has(current.status)) return current;
    return next;
  });
}
