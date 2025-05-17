import { writable } from 'svelte/store';
import { dispatchMenuChange } from '$lib/utils/stores/stores';

export const openNewsEventId = writable<number | null>(null);

export function openNews(eventId?: number) {
  dispatchMenuChange.set('news');
  openNewsEventId.set(eventId ?? null);
}
