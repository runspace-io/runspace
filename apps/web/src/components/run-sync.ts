import type { Dispatch, SetStateAction } from 'react';
import type { ApiRun } from '../lib/api-types';

export function updateActiveRun(
  setActiveRun: Dispatch<SetStateAction<ApiRun | undefined>>,
  status: ApiRun['status'],
  eventRunID: string | undefined,
  activeRunID: string | undefined,
) {
  setActiveRun((current) => {
    if (!current || (eventRunID && eventRunID !== current.id && eventRunID !== activeRunID))
      return current;
    return { ...current, status };
  });
}
