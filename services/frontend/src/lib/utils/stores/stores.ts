//stores.ts
import { writable } from 'svelte/store';
//export let currentTimestamp = writable(0);
import type {
    Settings,
    Strategy,
    Instance,
    Watchlist,
    Alert,
    AlertLog,
    AlertData
} from '$lib/utils/types/types';
import type { Writable } from 'svelte/store';
import { privateRequest } from '$lib/utils/helpers/backend';

// Define the Algo interface
export interface Algo {
    algoId: number;
    name: string;
    // Add other properties as needed
}

export const strategies: Writable<Strategy[]> = writable([]);
export const watchlists: Writable<Watchlist[]> = writable([]);
export const currentWatchlistId: Writable<number | undefined> = writable(undefined);
export const currentWatchlistItems: Writable<Instance[]> = writable([]);
export const activeAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const inactiveAlerts: Writable<Alert[] | undefined> = writable(undefined);
export const alertLogs: Writable<AlertLog[] | undefined> = writable(undefined);
export const alertPopup: Writable<AlertData | null> = writable(null);
export const menuWidth = writable(0);
export let flagWatchlistId: number | undefined;
export const entryOpen = writable(false);
export let flagWatchlist: Writable<Instance[]> = writable([]);
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
export const isPublicViewing = writable(false);

// Store for user's last used tickers
export const userLastTickers: Writable<any[]> = writable([]);

// Function to update user's last tickers when a ticker is selected
export function updateUserLastTickers(selectedTicker: any) {
    userLastTickers.update(tickers => {
        // Remove the ticker if it already exists
        const filtered = tickers.filter(t => t.ticker !== selectedTicker.ticker);
        // Add the selected ticker to the top
        return [selectedTicker, ...filtered.slice(0, 4)]; // Keep only top 5
    });
}

// Add constants for menu width
export const MIN_MENU_WIDTH = 200;
const DEFAULT_MENU_WIDTH = 450;

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
    showFilings: true,
    colorScheme: 'default'
};
export const settings: Writable<Settings> = writable(defaultSettings);
export function initStores() {
    initStoresWithAuth();
}

function initStoresWithAuth() {
    // Check if we're in public viewing mode first
    try {
        import('svelte/store').then(({ get }) => {
            if (get(isPublicViewing)) {
                // In public viewing mode, just set defaults
                settings.set(defaultSettings);
                strategies.set([]);
                activeAlerts.set([]);
                inactiveAlerts.set([]);
                alertLogs.set([]);
                watchlists.set([]);
                return;
            }

            // Normal initialization for authenticated users - move all private requests here
            privateRequest<Settings>('getSettings', {}).then((s: Settings) => {
                settings.set({ ...defaultSettings, ...s });
            }).catch((error) => {
                console.warn('Failed to load settings:', error);
                settings.set(defaultSettings);
            });

            privateRequest<Strategy[]>('getStrategies', {}).then((v: Strategy[]) => {
                if (!v) {
                    strategies.set([]);
                    return;
                }
                v = v.map((v: Strategy) => {
                    return {
                        ...v,
                        activeScreen: true
                    };
                });
                strategies.set(v);
            }).catch((error) => {
                console.warn('Failed to load strategies:', error);
                strategies.set([]);
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
            }).catch((error) => {
                console.warn('Failed to load alerts:', error);
                inactiveAlerts.set([]);
                activeAlerts.set([]);
            });

            privateRequest<AlertLog[]>('getAlertLogs', {}).then((v: AlertLog[]) => {
                alertLogs.set(v || []);
            }).catch((error) => {
                console.warn('Failed to load alert logs:', error);
                alertLogs.set([]);
            });

            privateRequest<Watchlist[]>('getWatchlists', {}).then((list: Watchlist[]) => {
                watchlists.set(list || []);
                const flagWatch = list?.find((v: Watchlist) => v.watchlistName === 'flag');
                if (flagWatch === undefined) {
                    privateRequest<number>('newWatchlist', { watchlistName: 'flag' }).then((newId: number) => {
                        flagWatchlistId = newId;
                        watchlists.update(currentList => {
                            const newList = currentList || [];
                            return [{ watchlistId: newId, watchlistName: 'flag' }, ...newList];
                        });
                    }).catch(err => {
                        console.error("Error creating flag watchlist:", err);
                    });
                } else {
                    flagWatchlistId = flagWatch.watchlistId;
                }
            }).catch(err => {
                console.error("Error fetching watchlists:", err);
                watchlists.set([]);
            });

            // Load user's last tickers
            privateRequest<any[]>('getUserLastTickers', {}).then((tickers: any[]) => {
                userLastTickers.set(tickers || []);
            }).catch((error) => {
                console.warn('Failed to load user last tickers:', error);
                userLastTickers.set([]);
            });
        });
    } catch (error) {
        console.warn('Failed to check public viewing mode, proceeding with auth initialization');
    }
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

export type Menu = 'none' | 'watchlist' | 'alerts' | 'news';

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

        if (v.replayActive) {
            const now = Date.now();
            const newOffset = serverTimestamp - now;
            return {
                ...v,
                serverTimeOffset: newOffset
            }
        }
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
