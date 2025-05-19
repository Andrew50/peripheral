import { writable, get } from 'svelte/store';
import { bottomWindowRequest, strategies } from '$lib/utils/stores/stores';
import { privateRequest } from '$lib/utils/helpers/backend';
import type { Strategy } from '$lib/utils/types/types';

export const backtestRunRequest = writable<number | null>(null);

export async function openBacktest(idOrName: number | string) {
  bottomWindowRequest.set('backtest');
  if (typeof idOrName === 'number') {
    backtestRunRequest.set(idOrName);
    return;
  }

  let list = get(strategies);
  let match = Array.isArray(list)
    ? list.find((s) => s.name.toLowerCase() === idOrName.toLowerCase())
    : undefined;

  if (!match) {
    try {
      const refreshed = await privateRequest<Strategy[]>('getStrategies', {});
      strategies.set(refreshed || []);
      match = refreshed?.find((s) => s.name.toLowerCase() === idOrName.toLowerCase());
    } catch {
      // ignore errors
    }
  }

  if (match) {
    backtestRunRequest.set(match.strategyId);
  }
}
