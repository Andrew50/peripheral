//stores.ts
import { writable } from 'svelte/store';
//export let currentTimestamp = writable(0);
import type {
    Settings,
    Setup,
    Instance,
    Watchlist,
    Alert,
    AlertLog,
    AlertData
} from '$lib/core/types';
import type { Writable } from 'svelte/store';
import { privateRequest } from '$lib/core/backend';

// Define the Algo interface
export interface Algo {
    algoId: number;
    name: string;
    // Add other properties as needed
}

export const setups: Writable<Setup[]> = writable([]);
export const watchlists: Writable<Watchlist[]> = writable([]);
export const activeAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const inactiveAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const alertLogs: Writable<AlertLog[] | undefined> = writable(undefined);
export const alertPopup: Writable<AlertData | null> = writable(null);
export const menuWidth = writable(0);
export let flagWatchlistId: number | undefined;
export const entryOpen = writable(false);
export let flagWatchlist: Writable<Instance[]>;
export const streamInfo = writable<StreamInfo>({
    replayActive: false,
    replaySpeed: 1,
    replayPaused: false,
    startTimestamp: 0,
    timestamp: Date.now(),
    extendedHours: false,
    serverTimeOffset: undefined
});
export const systemClockOffset = 0;
export const dispatchMenuChange = writable('');
export const algos: Writable<Algo[]> = writable([]);

// Add constants for menu width
export const MIN_MENU_WIDTH = 200;
const DEFAULT_MENU_WIDTH = 300;

export interface StreamInfo {
    replayActive: boolean;
    replaySpeed: number;
    replayPaused: boolean;
    startTimestamp: number;
    timestamp: number;
    extendedHours: boolean;
    lastUpdateTime?: number;
    serverTimeOffset?: number;
}

export interface ReplayInfo extends StreamInfo {
    replayActive: boolean;
    replayPaused: boolean;
    replaySpeed: number;
    startTimestamp: number;
    pauseTime?: number;
    extendedHours: boolean;
}

export interface TimeEvent {
    event: 'newDay' | 'replay' | null;
    UTCtimestamp: number;
}
export const timeEvent: Writable<TimeEvent> = writable({ event: null, UTCtimestamp: 0 });
export const defaultSettings: Settings = {
    chartRows: 1,
    chartColumns: 1,
    dolvol: false,
    adrPeriod: 20,
    filterTaS: true,
    divideTaS: false,
    showFilings: true
};
export const settings: Writable<Settings> = writable(defaultSettings);
export function initStores() {
    privateRequest<Settings>('getSettings', {}).then((s: Settings) => {
        settings.set({ ...defaultSettings, ...s });
    });
    privateRequest<Setup[]>('getSetups', {}).then((v: Setup[]) => {
        if (!v) {
            setups.set([]);
            return;
        }
        v = v.map((v: Setup) => {
            return {
                ...v,
                activeScreen: true
            };
        });
        setups.set(v);
    });

    // Add alert initialization
    privateRequest<Alert[]>('getAlerts', {}).then((v: Alert[]) => {
        if (v === undefined || v === null) {
            inactiveAlerts.set([]);
            activeAlerts.set([]);
            return;
        }
        const inactive = v.filter((alert: Alert) => alert.active === false);
        inactiveAlerts.set(inactive);
        const active = v.filter((alert: Alert) => alert.active === true);
        activeAlerts.set(active);
    });

    privateRequest<AlertLog[]>('getAlertLogs', {}).then((v: AlertLog[]) => {
        alertLogs.set(v || []);
    });

    function loadFlagWatchlist() {
        privateRequest<Instance[]>('getWatchlistItems', { watchlistId: flagWatchlistId }).then(
            (v: Instance[]) => {
                flagWatchlist = writable(v);
            }
        );
    }
    privateRequest<Watchlist[]>('getWatchlists', {}).then((list: Watchlist[]) => {
        watchlists.set(list);
        const flagWatch = list?.find((v: Watchlist) => v.watchlistName === 'flag');
        if (flagWatch === undefined) {
            privateRequest<number>('newWatchlist', { watchlistName: 'flag' }).then((v: number) => {
                flagWatchlistId = v;
                loadFlagWatchlist();
            });
        } else {
            flagWatchlistId = flagWatch.watchlistId;
        }
        loadFlagWatchlist();
    });
    function updateTime() {
        streamInfo.update((v: StreamInfo) => {
            if (v.replayActive && !v.replayPaused) {
                const currentTime = Date.now();
                const elapsedTime = v.lastUpdateTime ? currentTime - v.lastUpdateTime : 0;
                v.timestamp += elapsedTime * v.replaySpeed;
                v.lastUpdateTime = currentTime;
            } else if (!v.replayActive && v.serverTimeOffset !== undefined) {
                v.timestamp = Date.now() + v.serverTimeOffset;
            }
            return v;
        });
    }
    setInterval(updateTime, 250);
}

export type Menu = 'none' | 'watchlist' | 'alerts' | 'study' | 'journal' | 'similar';

export const activeMenu = writable<Menu>('none');

export function changeMenu(menuName: Menu) {
    activeMenu.update((current) => {
        if (current === menuName || menuName === 'none') {
            menuWidth.set(0);
            return 'none';
        }
        if (current === 'none') {
            menuWidth.set(DEFAULT_MENU_WIDTH);
        }
        return menuName;
    });
}

export function formatTimestamp(timestamp: number) {
    if (timestamp === 0) {
        return new Date().toLocaleDateString('en-US') + ' ' + new Date().toLocaleTimeString('en-US');
    }
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

export function handleTimestampUpdate(serverTimestamp: number) {
    streamInfo.update((v) => {
        const now = Date.now();
        const newOffset = serverTimestamp - now;

        if (v.serverTimeOffset === undefined || Math.abs(newOffset - v.serverTimeOffset) > 1000) {
            return {
                ...v,
                timestamp: serverTimestamp,
                lastUpdateTime: now,
                serverTimeOffset: newOffset
            };
        }

        return {
            ...v,
            timestamp: serverTimestamp,
            lastUpdateTime: now
        };
    });
}
