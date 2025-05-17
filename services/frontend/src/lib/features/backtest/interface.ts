import { writable } from 'svelte/store';
import { bottomWindowRequest } from '$lib/utils/stores/stores';

export const backtestRunRequest = writable<number | null>(null);

export function openBacktest(strategyId: number) {
  bottomWindowRequest.set('backtest');
  backtestRunRequest.set(strategyId);
}
