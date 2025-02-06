//stores.ts
import { writable } from 'svelte/store'
//export let currentTimestamp = writable(0);
import type { Settings, Algo, Setup, Instance, Watchlist, Alert, AlertLog, AlertData } from '$lib/core/types'
import type { Writable } from 'svelte/store'
import { privateRequest } from '$lib/core/backend'
export const setups: Writable<Setup[]> = writable([]);
export const watchlists: Writable<Watchlist[]> = writable([]);
export const activeAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const inactiveAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const alertLogs: Writable<AlertLog[] | undefined> = writable(undefined);
export const alertPopup: Writable<AlertData | null> = writable(null);
export const menuWidth = writable(0);
export let flagWatchlistId: number | undefined;
export const entryOpen = writable(false)
export let flagWatchlist: Writable<Instance[]>
export const streamInfo = writable<StreamInfo>({ replayActive: false, replaySpeed: 1, replayPaused: false, startTimestamp: 0, timestamp: 0, extendedHours: false })
export const systemClockOffset = 0;
export const dispatchMenuChange = writable("")
export const algos: Writable<Algo[]> = writable([])

export interface StreamInfo {
    replayActive: boolean,
    replaySpeed: number,
    replayPaused: boolean,
    startTimestamp: number,
    timestamp: number,
    extendedHours: boolean,
    lastUpdateTime?: number,
    pauseTime?: number
}
export interface TimeEvent {
    event: "newDay" | "replay" | null,
    UTCtimestamp: number
}
export let timeEvent: Writable<TimeEvent> = writable({ event: null, UTCtimestamp: 0 })
const defaultSettings = {
    chartRows: 1, chartColumns: 1, dolvol: false, adrPeriod: 20, divideTaS: false, filterTaS: false,
}
export let settings: Writable<Settings> = writable(defaultSettings)
export function initStores() {
    privateRequest<Settings>("getSettings", {})
        .then((s: Settings) => {
            settings.set({ ...defaultSettings, ...s })
        })
    privateRequest<Setup[]>('getSetups', {})
        .then((v: Setup[]) => {
            v = v.map((v: Setup) => {
                return {
                    ...v,
                    activeScreen: true
                }
            })
            setups.set(v);
        })
    function loadFlagWatchlist() {
        privateRequest<Instance[]>("getWatchlistItems", { watchlistId: flagWatchlistId })
            .then((v: Instance[]) => {
                flagWatchlist = writable(v)
            })
    }
    privateRequest<Watchlist[]>("getWatchlists", {})
        .then((list: Watchlist[]) => {
            watchlists.set(list)
            const flagWatch = list?.find((v: Watchlist) => v.watchlistName === "flag")
            if (flagWatch === undefined) {
                privateRequest<number>("newWatchlist", { watchlistName: "flag" }).then((v: number) => {
                    flagWatchlistId = v
                    loadFlagWatchlist()
                })
            } else {
                flagWatchlistId = flagWatch.watchlistId
            }
            loadFlagWatchlist()
        })
    function updateTime() {
        streamInfo.update((v: StreamInfo) => {
            if (v.replayActive && !v.replayPaused) {
                const currentTime = Date.now();
                const elapsedTime = v.lastUpdateTime ? currentTime - v.lastUpdateTime : 0;
                v.timestamp += elapsedTime * v.replaySpeed;
                v.lastUpdateTime = currentTime;  // Update the last update time
            }
            return v;
        });
    }
    setInterval(updateTime, 250)
}





export function formatTimestamp(timestamp: number) {
    const date = new Date(timestamp);
    return date.toLocaleDateString('en-US') + ' ' + date.toLocaleTimeString('en-US');
}


export const activeChartInstance = writable<Instance>({
    ticker: '',
    timestamp: 0,
    timeframe: '',
    securityId: 0,
    extendedHours: false
});

