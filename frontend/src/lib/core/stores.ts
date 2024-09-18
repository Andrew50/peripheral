import type {Setup} from '$lib/core/types'
import {writable} from 'svelte/store'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'
import { onMount } from 'svelte';

privateRequest<Setup[]>('getSetups', {})
.then((v: Setup[]) => {
    v = v.map((v:Setup) => {
        return {...v,
          activeScreen: true}
    })
    console.log(v)
    setups.set(v);
})
.catch((error) => {
    console.error('Error fetching setups:', error);
});
export let setups: Writable<Setup[]> = writable([]);
export let currentTimestamp = writable(0);


export function updateTime() {

    // ADD IF REPLAY 
    currentTimestamp.set(Date.now()); 
}


export function formatTimestamp(timestamp : number) {
    const date = new Date(timestamp);
    return date.toLocaleDateString('en-US') + ' ' + date.toLocaleTimeString('en-US');
}