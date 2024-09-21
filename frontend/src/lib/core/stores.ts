import type {Settings,Setup,Instance,Watchlist} from '$lib/core/types'
import {writable} from 'svelte/store'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'
import { onMount } from 'svelte';

export let setups: Writable<Setup[]> = writable([]);
export let watchlists: Writable<Watchlist[]> = writable([]);
export let flagWatchlistId: number | undefined;
export let flagWatchlist: Writable<Instance[]>
export let settings:Writable<Settings> = writable()
export let currentTimestamp = writable(0);
export function initStores(){
    privateRequest<Settings>("getOptions",{})
    .then((s:Settings)=>{
        settings.set(s)
    })
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
        privateRequest<Instance[]>("getWatchlistItems",{watchlistId:flagWatchlistId})
        .then((v:Instance[])=>{
            flagWatchlist = writable(v)
        })
    }


    privateRequest<Watchlist[]>("getWatchlists",{})
    .then((list:Watchlist[])=>{
        watchlists.set(list)
        const flagWatch = list?.find((v:Watchlist)=>v.watchlistName === "flag")
        if (flagWatch === undefined){
            privateRequest<number>("newWatchlist",{watchlistName:"flag"}).then((v:number)=>{
                flagWatchlistId = v
                loadFlagWatchlist()
            })
        }else{
            flagWatchlistId = flagWatch.watchlistId
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
