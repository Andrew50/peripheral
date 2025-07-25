//stores.ts
import { writable, type Writable, get } from 'svelte/store';
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
import { privateRequest } from '$lib/utils/helpers/backend';
import { browser } from '$app/environment';

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
export const leftMenuWidth = writable(0);
export let flagWatchlistId: number | undefined;
export const entryOpen = writable(false);
export const flagWatchlist: Writable<Instance[]> = writable([]);
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
export const userLastTickers = writable<Instance[]>([]);

// Subscription status store
export interface SubscriptionStatus {
	status: string;
	isActive: boolean;
	isCanceling: boolean;
	currentPlan: string;
	hasCustomer: boolean;
	hasSubscription: boolean;
	currentPeriodEnd: number | null;
	loading: boolean;
	error: string;
	// Credit information
	subscriptionCreditsRemaining?: number;
	purchasedCreditsRemaining?: number;
	totalCreditsRemaining?: number;
	subscriptionCreditsAllocated?: number;
	// Alert information
	activeAlerts?: number;
	alertsLimit?: number;
	activeStrategyAlerts?: number;
	strategyAlertsLimit?: number;
}

export const subscriptionStatus = writable<SubscriptionStatus>({
	status: 'inactive',
	isActive: false,
	isCanceling: false,
	currentPlan: '',
	hasCustomer: false,
	hasSubscription: false,
	currentPeriodEnd: null,
	loading: false,
	error: '',
	subscriptionCreditsRemaining: 0,
	purchasedCreditsRemaining: 0,
	totalCreditsRemaining: 0,
	subscriptionCreditsAllocated: 0
});

// Cache for subscription data to prevent unnecessary API calls
let lastFetchTime = 0;
const CACHE_DURATION = 30000; // 30 seconds

// Function to fetch and update subscription status
export async function fetchSubscriptionStatus() {
	console.log('fetchSubscriptionStatus called');

	// Only fetch if we're in the browser and have auth
	if (!browser) {
		console.log('Not in browser, skipping fetch');
		return;
	}

	try {
		const authToken = sessionStorage.getItem('authToken');
		if (!authToken) {
			console.log('No auth token found, skipping fetch');
			return;
		}

		console.log(
			'Starting subscription status fetch with token:',
			authToken.substring(0, 20) + '...'
		);
		subscriptionStatus.update((s) => ({ ...s, loading: true, error: '' }));

		console.log('Making privateRequest to getSubscriptionStatus');
		const response = await privateRequest<SubscriptionStatus>('getSubscriptionStatus', {});
		console.log('Received subscription status response:', response);

		subscriptionStatus.update((s) => ({
			...s,
			...response,
			loading: false,
			error: ''
		}));
		console.log('Updated subscription status store successfully');
	} catch {
		console.error('Failed to fetch subscription status');
		subscriptionStatus.update((s) => ({
			...s,
			loading: false,
			error: 'Failed to load subscription status'
		}));
	}
}

// Function to fetch user usage separately (for more frequent updates)
export async function fetchUserUsage() {
	console.log('fetchUserUsage called');

	// Only fetch if we're in the browser and have auth
	if (!browser) {
		console.log('Not in browser, skipping fetch');
		return;
	}

	try {
		const authToken = sessionStorage.getItem('authToken');
		if (!authToken) {
			console.log('No auth token found, skipping fetch');
			return;
		}

		console.log('Making privateRequest to getUserUsageStats');
		const response = await privateRequest<{
			subscription_credits_remaining: number;
			purchased_credits_remaining: number;
			total_credits_remaining: number;
			subscription_credits_allocated: number;
			active_alerts: number;
			alerts_limit: number;
			active_strategy_alerts: number;
			strategy_alerts_limit: number;
		}>('getUserUsageStats', {});
		console.log('Received user usage response:', response);

		subscriptionStatus.update((s) => ({
			...s,
			subscriptionCreditsRemaining: response.subscription_credits_remaining,
			purchasedCreditsRemaining: response.purchased_credits_remaining,
			totalCreditsRemaining: response.total_credits_remaining,
			subscriptionCreditsAllocated: response.subscription_credits_allocated,
			activeAlerts: response.active_alerts,
			alertsLimit: response.alerts_limit,
			activeStrategyAlerts: response.active_strategy_alerts,
			strategyAlertsLimit: response.strategy_alerts_limit
		}));
		console.log('Updated usage in subscription status store successfully');
	} catch (error) {
		console.error('Failed to fetch user usage:', error);
	}
}

// Function to update user's last tickers when a ticker is selected
export function updateUserLastTickers(selectedTicker: Instance) {
	userLastTickers.update((tickers) => {
		// Remove the ticker if it already exists
		const filtered = tickers.filter((t) => t.ticker !== selectedTicker.ticker);
		// Add the selected ticker to the top
		return [selectedTicker, ...filtered.slice(0, 2)]; // Keep only top 3
	});
}

// Add constants for menu width
export const MIN_MENU_WIDTH = 200;

// Calculate default menu width based on screen size using continuous function
function getDefaultMenuWidth(): number {
	if (typeof window === 'undefined') return 250; // SSR fallback

	const screenWidth = window.innerWidth;

	// Continuous function for default menu width
	// Smoothly transitions from 25% at 800px to 20% at 2000px+
	const minScreenWidth = 800;
	const maxScreenWidth = 2000;
	const minPercentage = 0.2;  // 20% at large screens
	const maxPercentage = 0.25; // 25% at small screens

	// Calculate percentage using linear interpolation, clamped to range
	const clampedWidth = Math.max(minScreenWidth, Math.min(screenWidth, maxScreenWidth));
	const t = (clampedWidth - minScreenWidth) / (maxScreenWidth - minScreenWidth);
	const percentage = maxPercentage - (t * (maxPercentage - minPercentage));

	// Calculate width with continuous function
	const calculatedWidth = screenWidth * percentage;

	// Apply absolute limits with smooth transitions
	const absoluteMin = 200;
	const absoluteMax = 400;

	return Math.round(Math.max(absoluteMin, Math.min(calculatedWidth, absoluteMax)));
}

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
			privateRequest<Settings>('getSettings', {})
				.then((s: Settings) => {
					settings.set({ ...defaultSettings, ...s });
				})
				.catch(() => {
					console.warn('Failed to load settings');
					settings.set(defaultSettings);
				});

			privateRequest<Strategy[]>('getStrategies', {})
				.then((v: Strategy[]) => {
					console.log(v)
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
				})
				.catch(() => {
					console.warn('Failed to load strategies');
					strategies.set([]);
				});

			// Add alert initialization
			privateRequest<Alert[]>('getAlerts', { alertType: 'all' })
				.then((v: Alert[]) => {
					if (v === undefined || v === null) {
						inactiveAlerts.set([]);
						activeAlerts.set([]);
						return;
					}
					const inactive = v.filter((alert: Alert) => alert.active === false);
					inactiveAlerts.set(inactive);
					const active = v.filter((alert: Alert) => alert.active === true);
					activeAlerts.set(active);
				})
				.catch(() => {
					console.warn('Failed to load alerts');
					inactiveAlerts.set([]);
					activeAlerts.set([]);
				});

			privateRequest<AlertLog[]>('getAlertLogs', { alertType: 'all' })
				.then((v: AlertLog[]) => {
					alertLogs.set(v || []);
				})
				.catch(() => {
					console.warn('Failed to load alert logs');
					alertLogs.set([]);
				});

			privateRequest<Watchlist[]>('getWatchlists', {})
				.then((list: Watchlist[]) => {
					watchlists.set(list || []);
					const flagWatch = list?.find((v: Watchlist) => v.watchlistName === 'flag');
					if (flagWatch === undefined) {
						privateRequest<number>('newWatchlist', { watchlistName: 'flag' })
							.then((newId: number) => {
								flagWatchlistId = newId;
								watchlists.update((currentList) => {
									const newList = currentList || [];
									return [{ watchlistId: newId, watchlistName: 'flag' }, ...newList];
								});

								// Initialize the flagWatchlist store with empty array for new flag watchlist
								flagWatchlist.set([]);

								// Initialize visible watchlists and default selection after flag creation
								import('$lib/features/watchlist/watchlistUtils').then(
									({ initializeVisibleWatchlists, selectWatchlist }) => {
										const updatedList = [{ watchlistId: newId, watchlistName: 'flag' }, ...(list || [])];
										if (updatedList.length) {
											selectWatchlist(String(updatedList[0].watchlistId));
											initializeVisibleWatchlists(updatedList);
										}
									}
								).catch(() => {
									// Fail silently to avoid breaking the app
								});
							})
							.catch((err) => {
								console.error('Error creating flag watchlist:', err);
							});
					} else {
						flagWatchlistId = flagWatch.watchlistId;

						// Initialize the flagWatchlist store with existing items
						privateRequest<Instance[]>('getWatchlistItems', { watchlistId: flagWatch.watchlistId })
							.then((items: Instance[]) => {
								flagWatchlist.set(items || []);
							})
							.catch((err) => {
								console.error('Error loading flag watchlist items:', err);
								flagWatchlist.set([]);
							});

						// Initialize visible watchlists and default selection when watchlists are loaded
						import('$lib/features/watchlist/watchlistUtils').then(
							({ initializeVisibleWatchlists, selectWatchlist }) => {
								if (Array.isArray(list) && list.length) {
									// Select flag watchlist if it exists, otherwise first watchlist
									const defaultWatchlist = flagWatch || list[0];
									selectWatchlist(String(defaultWatchlist.watchlistId));
									initializeVisibleWatchlists(list);
								}
							}
						).catch(() => {
							// Fail silently to avoid breaking the app
						});
					}
				})
				.catch((err) => {
					console.error('Error fetching watchlists:', err);
					watchlists.set([]);
				});

			// Load user's last tickers
			privateRequest<Instance[]>('getUserLastTickers', {})
				.then((tickers: Instance[]) => {
					userLastTickers.set(tickers || []);
				})
				.catch(() => {
					console.warn('Failed to load user last tickers');
					userLastTickers.set([]);
				});
		});
	} catch {
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
			// Recalculate default width based on current screen size
			menuWidth.set(getDefaultMenuWidth());
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
			};
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

// Function to fetch combined subscription status and usage in a single call
export async function fetchCombinedSubscriptionAndUsage(forceRefresh = false) {
	console.log('fetchCombinedSubscriptionAndUsage called');

	// Only fetch if we're in the browser and have auth
	if (!browser) {
		console.log('Not in browser, skipping fetch');
		return;
	}

	// Check cache if not forcing refresh
	if (!forceRefresh && Date.now() - lastFetchTime < CACHE_DURATION) {
		console.log('Using cached subscription data');
		return;
	}

	try {
		const authToken = sessionStorage.getItem('authToken');
		if (!authToken) {
			console.log('No auth token found, skipping fetch');
			return;
		}

		console.log('Making privateRequest to getCombinedSubscriptionAndUsage');
		subscriptionStatus.update((s) => ({ ...s, loading: true, error: '' }));

		const response = await privateRequest<SubscriptionStatus>('getCombinedSubscriptionAndUsage', {});
		console.log('Received combined subscription and usage response:', response);

		subscriptionStatus.update((s) => ({
			...s,
			...response,
			loading: false,
			error: ''
		}));

		// Update cache timestamp
		lastFetchTime = Date.now();
		console.log('Updated subscription status store with combined data successfully');
	} catch (error) {
		console.error('Failed to fetch combined subscription and usage:', error);
		subscriptionStatus.update((s) => ({
			...s,
			loading: false,
			error: 'Failed to load subscription and usage data'
		}));
	}
}

// Add horizontal lines interface and store
export interface HorizontalLine {
	id: number;
	securityId: number;
	price: number;
	color: string;
	lineWidth: number;
}

export const horizontalLines: Writable<HorizontalLine[]> = writable([]);

// NEW: Centralized synchronization for flag watchlist
// This ensures that if the flag watchlist is being viewed, its contents
// are always in sync with the main 'currentWatchlistItems' store.
if (browser) {
	flagWatchlist.subscribe((flaggedItems) => {
		// Get the current value of the selected watchlist ID
		const selectedWatchlistId = get(currentWatchlistId);

		// If the currently selected watchlist is the flag watchlist, update
		// the main items store with the new content from the flag watchlist.
		if (selectedWatchlistId === flagWatchlistId) {
			currentWatchlistItems.set(flaggedItems || []);
		}
	});
}
