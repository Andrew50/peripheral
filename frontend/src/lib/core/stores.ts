import type {Setup, Watch,Watchlist} from '$lib/core/types'
import {writable} from 'svelte/store'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'
import { onMount } from 'svelte';

export let setups: Writable<Setup[]> = writable([]);
export let watchlists: Writable<Watchlist[]> = writable([]);
export let flagWatchlistId: number | undefined;
export let flagWatchlist: Writable<Watch[]>
export let chartLayout = writable({rows:1,columns:1})
export let currentTimestamp = writable(0);
export function initStores(){
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

    function loadFlagWatchlist(){
        privateRequest<Watch[]>("getWatchlistItems",{watchlistId:flagWatchlistId})
        .then((v:Watch[])=>{
            flagWatchlist = writable(v)
        })
    }


    privateRequest<Watchlist[]>("getWatchlists",{})
    .then((list:Watchlist[])=>{
        watchlists.set(list)
        flagWatchlistId = list?.find((v:Watchlist)=>v.watchlistName === "flag").watchlistId
        if (!flagWatchlistId){
            privateRequest<number>("newWatchlist",{watchlistName:"flag"}).then((v:number)=>{
                flagWatchlistId = v
                loadFlagWatchlist()
            })
        }
        loadFlagWatchlist()

    })
}



export function updateTime() {

    // ADD IF REPLAY 
    currentTimestamp.set(Date.now()); 
}


export function formatTimestamp(timestamp : number) {
    const date = new Date(timestamp);
    return date.toLocaleDateString('en-US') + ' ' + date.toLocaleTimeString('en-US');
}
