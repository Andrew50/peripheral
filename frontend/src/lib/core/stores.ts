import type {Setup, Watch,Watchlist} from '$lib/core/types'
import {writable} from 'svelte/store'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'
import { onMount } from 'svelte';

export let setups: Writable<Setup[]> = writable([]);
export let watchlists: Writable<Watchlist[]> writable([]);
export let todaysWatchlistId: number;
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



privateRequest<Watch[]>("getWatchlists",{})
.then((lists:Watchlist[])=>{
    todaysWatchlistName = 
    const todaysWatchlistId = lists.find((v:Watchlist)=>v.watchlistName === "flag")
    if (!todaysWatchlistId){



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
