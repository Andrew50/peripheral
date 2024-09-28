//stores.ts
import {writable} from 'svelte/store'
export let currentTimestamp = writable(0);
import type {Settings,Setup,Instance,Watchlist} from '$lib/core/types'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'
export let setups: Writable<Setup[]> = writable([]);
export let watchlists: Writable<Watchlist[]> = writable([]);
export let menuWidth = writable(0);
export let flagWatchlistId: number | undefined;
export let entryOpen = writable(false)
export let flagWatchlist: Writable<Instance[]>
export let replayInfo = writable<ReplayInfo>({status:"inactive",startTimestamp:0, replaySpeed:1,})
export let systemClockOffset = 0;
export interface ReplayInfo {
    status: "inactive" | "active" | "paused",
    startTimestamp: number,
    replaySpeed: number,
}
export interface TimeEvent {
    event:"newDay" | "replay" | null,
    UTCtimestamp: number
}
export let timeEvent: Writable<TimeEvent> = writable({event:null,UTCtimestamp:0})
const defaultSettings = {
    chartRows: 1, chartColumns:1, dolvol:false, adrPeriod:20, divideTaS:false, filterTaS:false,
}
export let settings:Writable<Settings> = writable(defaultSettings)
let prevTimestamp: number | null = null;
import { replayStream } from '$lib/utils/stream';
export function initStores(){
    privateRequest<Settings>("getSettings",{})
    .then((s:Settings)=>{
        settings.set({...defaultSettings,...s})
    })
    privateRequest<Setup[]>('getSetups', {})
    .then((v: Setup[]) => {
        v = v.map((v:Setup) => {
            return {...v,
              activeScreen: true}
        })
        setups.set(v);
    })
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
    currentTimestamp.subscribe((newTimestamp: number ) => {
        if (prevTimestamp !== null) {
            const prevDay = new Date(prevTimestamp).setHours(0, 0, 0, 0);
            const newDay = new Date(newTimestamp).setHours(0, 0, 0, 0);
            if (newDay !== prevDay) {
                timeEvent.set({event:"newDay",UTCtimestamp:(newTimestamp)})
            }
        }
        prevTimestamp = newTimestamp;
    });
    async function getSystemClockOffset() {
        try {
            const response = await fetch('https://worldtimeapi.org/api/ip');
            if (!response.ok) {
                throw new Error('Failed to fetch time data');
            }
            const data = await response.json();
            const serverTime = new Date(data.utc_datetime).getTime(); // Server time in milliseconds
            const localTime = Date.now();

            // Calculate the offset
            systemClockOffset = serverTime - localTime;
            console.log(`System clock offset: ${systemClockOffset} ms`);
        } catch (error) {
            console.error('Error fetching system time:', error);
        }
    }
    getSystemClockOffset()
}



export function updateTime() {

    // ADD IF REPLAY 
    if(replayStream.replayStatus) {
        currentTimestamp.set(replayStream.simulatedTime);
    }
    else {
        currentTimestamp.set(Date.now() + systemClockOffset); 
    }
}


export function formatTimestamp(timestamp : number) {
    const date = new Date(timestamp);
    return date.toLocaleDateString('en-US') + ' ' + date.toLocaleTimeString('en-US');
}
