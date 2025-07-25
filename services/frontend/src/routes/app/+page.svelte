<script lang="ts">
	import '$lib/styles/global.css';
	import '$lib/styles/app.css';
	import ChartContainer from '$lib/features/chart/chartContainer.svelte';
	import Alerts from '$lib/features/alerts/alert.svelte';
	import RightClick from '$lib/components/rightClick.svelte';
	import StrategiesPopup from '$lib/components/strategiesPopup.svelte';
	import Input from '$lib/components/input/input.svelte';
	import ExtendedHoursToggle from '$lib/components/extendedHoursToggle/extendedHoursToggle.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import Watchlist from '$lib/features/watchlist/watchlist.svelte';
	import WatchlistTabs from '$lib/features/watchlist/watchlistTabs.svelte';
	import Quote from '$lib/features/quotes/quote.svelte';
	import { activeMenu, changeMenu } from '$lib/utils/stores/stores';
	// Define PageData interface locally since auto-generated types aren't available yet
	interface PageData {
		defaultChartData: any;
	}

	// Constants
	const SIDEBAR_BUTTONS_WIDTH = 45; // Width of the sidebar buttons panel in pixels

	// Replay logic
	import {
		startReplay,
		stopReplay,
		pauseReplay,
		resumeReplay,
		changeSpeed,
		nextDay
	} from '$lib/utils/stream/interface';
	import { queryInstanceInput, inputQuery } from '$lib/components/input/input.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { browser } from '$app/environment';
	import { onMount, onDestroy, tick } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { page } from '$app/stores';
	import { get, writable } from 'svelte/store';
	import {
		initStores,
		streamInfo,
		formatTimestamp,
		dispatchMenuChange,
		menuWidth,
		leftMenuWidth,
		settings,
		isPublicViewing as isPublicViewingStore
	} from '$lib/utils/stores/stores';
	import { colorSchemes, applyColorScheme } from '$lib/styles/colorSchemes';

	// Import Instance from types
	import type { Instance } from '$lib/utils/types/types';

	// Add new import for Query component
	import Query from '$lib/features/chat/chat.svelte';

	import { requestChatOpen } from '$lib/features/chat/interface'; // Import the store

	// Import the standalone calendar component
	import Calendar from '$lib/components/calendar/calendar.svelte';

	// Import auth modal
	import AuthModal from '$lib/components/authModal.svelte';
	import { authModalStore, hideAuthModal } from '$lib/stores/authModal';
	import { subscriptionStatus, fetchSubscriptionStatus } from '$lib/utils/stores/stores';

	// Import mobile device detection
	import { isMobileDevice } from '$lib/utils/stores/device';

	// Debug logging for interface selection
	$: if (browser && $isMobileDevice !== undefined) {
		console.log('📱 [interface] Device detection result:', {
			isMobileDevice: $isMobileDevice,
			userAgent: navigator.userAgent,
			screenWidth: window.innerWidth,
			interface: $isMobileDevice ? 'mobile-chat-only' : 'desktop-full'
		});
	}

	// Export data prop for server-side preloaded data
	export let data: PageData;

	// Import extended hours toggle store
	import {
		extendedHoursToggleVisible,
		hideExtendedHoursToggle,
		activeChartInstance
	} from '$lib/features/chart/interface';

	import { newPriceAlert } from '$lib/features/alerts/interface';

	// Import mobile banner component
	import MobileBanner from '$lib/components/mobileBanner.svelte';

	//type Menu = 'none' | 'watchlist' | 'alerts' | 'study' | 'news';
	type Menu = 'none' | 'watchlist' | 'alerts' | 'news';

	let lastSidebarMenu: Menu | null = null;
	const sidebarMenus: Menu[] = ['watchlist', 'alerts'];

	// ─── Alert tabs ────────────────────────────────────────────────────────────
	const alertTabs = ['price', 'strategy', 'logs'] as const;
	type AlertView = (typeof alertTabs)[number];
	let alertView: AlertView = 'price';

	// Bottom windows
	type BottomWindowType =
		| 'screener'
		//| 'options'
		| 'strategies'
		| 'settings'
		| 'deploy'
		//| 'news'
		| 'query';
	interface BottomWindow {
		id: number;
		type: BottomWindowType;
		x: number;
		y: number;
		width: number;
		height: number;
		visible: boolean;
	}

	let bottomWindows: BottomWindow[] = [];
	let nextWindowId = 1;
	let activeTab: 'interface' | 'account' | 'appearance' | 'usage' = 'interface'; // For settings window

	// Replay controls
	let replaySpeed = 1.0;

	// Resizing the bottom windows
	let bottomWindowsHeight = 0;
	let bottomResizing = false;
	const MIN_BOTTOM_HEIGHT = 50;
	const MAX_BOTTOM_HEIGHT = 1200;

	// Add these state variables near the top with other state declarations
	let lastBottomWindow: BottomWindow | null = null;

	// Initialize with default question mark avatar
	//let profilePic = '';
	// Username removed - using email for avatar generation when needed
	//let profilePicError = false;
	let profileIconKey = 0;
	let currentProfileDisplay = ''; // Add this to hold the current display value

	let sidebarResizing = false;
	let tickerInfoContainerHeight = 500; // Initial height
	const MIN_TICKER_INFO_CONTAINER_HEIGHT = 100;
	const MAX_TICKER_INFO_CONTAINER_HEIGHT = 600;

	// Calendar state
	let calendarVisible = false;

	// Mobile banner state
	let showMobileBanner = false;
	const MOBILE_BANNER_STORAGE_KEY = 'atlantis-mobile-banner-dismissed';

	// Initialize mobile banner visibility based on device and dismissal state
	$: if (browser && $isMobileDevice !== undefined) {
		if ($isMobileDevice) {
			const dismissed = localStorage.getItem(MOBILE_BANNER_STORAGE_KEY);
			showMobileBanner = !dismissed;
		} else {
			showMobileBanner = false;
		}
	}

	function dismissMobileBanner() {
		showMobileBanner = false;
		if (browser) {
			localStorage.setItem(MOBILE_BANNER_STORAGE_KEY, 'true');
		}
	}

	// Get shared conversation ID from server-side layout data
	$: layoutData = $page.data;
	$: sharedConversationId = layoutData?.sharedConversationId || '';

	// Set isPublicViewing store based on server-side data
	$: if (layoutData?.isPublicViewing !== undefined) {
		isPublicViewingStore.set(layoutData.isPublicViewing);
	}

	// Import connect
	import { connect } from '$lib/utils/stream/socket';

	// Apply color scheme reactively based on the store
	$: if ($settings.colorScheme && browser) {
		const scheme = colorSchemes[$settings.colorScheme];
		if (scheme) {
			applyColorScheme(scheme);
		}
	}

	// Sync store values with CSS custom properties
	$: if (browser && $leftMenuWidth !== undefined) {
		document.documentElement.style.setProperty('--left-sidebar-width', `${$leftMenuWidth}px`);
	}

	$: if (browser && $menuWidth !== undefined) {
		document.documentElement.style.setProperty('--right-sidebar-width', `${$menuWidth}px`);
	}

	// Add a reactive statement to handle window events
	$: if (draggingWindowId !== null) {
		if (browser) {
			window.addEventListener('mousemove', onDrag);
			window.addEventListener('mouseup', stopDrag);
		}
	} else {
		if (browser) {
			window.removeEventListener('mousemove', onDrag);
			window.removeEventListener('mouseup', stopDrag);
		}
	}

	// Track the last auto-input trigger to prevent rapid successive calls
	let lastAutoInputTime = 0;
	const AUTO_INPUT_DEBOUNCE_MS = 100; // Prevent auto-input triggers within 100ms of each other

	// Define the overscroll prevention handler with a stable reference
	const preventOverscroll = (e: TouchEvent) => {
		// Check if we're at the top or bottom of the page
		const { scrollTop, scrollHeight, clientHeight } = document.documentElement;
		const isAtTop = scrollTop === 0;
		const isAtBottom = scrollTop + clientHeight >= scrollHeight;

		// If at top or bottom and trying to scroll further, prevent default
		if (
			(isAtTop && e.touches[0].clientY > e.touches[0].clientY) ||
			(isAtBottom && e.touches[0].clientY < e.touches[0].clientY)
		) {
			e.preventDefault();
		}
	};

	// Define the keydown handler with a stable reference outside of onMount
	const keydownHandler = (event: KeyboardEvent) => {
		// Check if input component is already active or recently triggered
		const currentInputStatus = get(inputQuery).status;
		const inputWindowExists = document.getElementById('input-window') !== null;
		const now = Date.now();

		// Don't trigger if input is active, input window exists, or we recently triggered auto-input
		if (
			currentInputStatus !== 'inactive' ||
			inputWindowExists ||
			now - lastAutoInputTime < AUTO_INPUT_DEBOUNCE_MS
		) {
			return;
		}

		// Check if the user is currently in any standard input field
		const activeElement = document.activeElement;
		const isInputField =
			activeElement?.tagName === 'INPUT' ||
			activeElement?.tagName === 'TEXTAREA' ||
			activeElement?.getAttribute('contenteditable') === 'true';

		// If user is typing in any input field, don't intercept keystrokes
		if (isInputField) {
			return;
		}

		// Handle alphanumeric keys for auto-input capture
		if (/^[a-zA-Z0-9]$/.test(event.key) && !event.ctrlKey && !event.metaKey) {
			// Update last trigger time
			lastAutoInputTime = now;

			// Prevent the event from propagating to avoid double capture
			event.preventDefault();
			event.stopPropagation();

			// Create an initial instance with the first key as the inputString
			const initialKey = event.key.toUpperCase();

			// Use type assertion to allow the inputString property
			const instanceWithInput = {
				inputString: initialKey
			} as any;

			queryInstanceInput('any', ['ticker', 'timeframe'], instanceWithInput)
				.then((updatedInstance) => {
					queryChart(updatedInstance, true);
				})
				.catch((error) => {
					// Handle cancellation silently
					if (error.message !== 'User cancelled input') {
						console.error('Error in auto-input capture:', error);
					}
				});

			// Focus the chart container after input activation
			const chartContainer = document.getElementById(`chart_container-0`);
			if (chartContainer) {
				setTimeout(() => chartContainer.focus(), 0);
			}
			return;
		}

		// For non-alphanumeric keys, delegate to chart container if no specific element is focused
		if (!document.activeElement || document.activeElement === document.body) {
			const chartContainer = document.getElementById(`chart_container-0`);

			if (chartContainer) {
				// Focus the chart container
				chartContainer.focus();

				// Get the native event handlers from the chart container
				const nativeHandlers = (chartContainer as any)._svelte?.events?.keydown;

				if (nativeHandlers) {
					// Call each handler directly with the original event
					nativeHandlers.forEach((handler: Function) => {
						handler.call(chartContainer, event);
					});
				}
			}
		}
	};

	onMount(() => {
		if (!browser) return;

		// Set CSS variable for sidebar buttons width
		document.documentElement.style.setProperty(
			'--sidebar-buttons-width',
			`${SIDEBAR_BUTTONS_WIDTH}px`
		);
		initStores();
		// Initialize CSS custom properties for sidebar widths if not already set
		if (!getComputedStyle(document.documentElement).getPropertyValue('--left-sidebar-width')) {
			document.documentElement.style.setProperty('--left-sidebar-width', `${$leftMenuWidth}px`);
		}
		if (!getComputedStyle(document.documentElement).getPropertyValue('--right-sidebar-width')) {
			document.documentElement.style.setProperty('--right-sidebar-width', `${$menuWidth}px`);
		}

		// Async initialization function
		async function init() {
			// Check for Stripe checkout success session_id parameter
			const urlParams = new URLSearchParams(window.location.search);
			const sessionId = urlParams.get('session_id');

			if (sessionId) {
				console.log('🎯 [onMount] Stripe checkout success detected, session_id:', sessionId);

				// Clear the session_id from URL for cleaner UX
				urlParams.delete('session_id');
				const newUrl = `${window.location.pathname}${urlParams.toString() ? '?' + urlParams.toString() : ''}`;
				window.history.replaceState({}, '', newUrl);

				// Defer verification until after page is fully loaded
				// This ensures verification happens AFTER the redirect to the app page
				setTimeout(async () => {
					console.log('⏰ [onMount] Deferred verification starting for session:', sessionId);
					await verifyAndUpdateSubscriptionStatus(sessionId);
				}, 100); // Small delay to ensure page is fully rendered
			}

			// Initialize subscription status if user is authenticated
			const authToken = sessionStorage.getItem('authToken');
			if (authToken) {
				// Only fetch if we haven't already triggered it above
				if (!sessionId) {
					fetchSubscriptionStatus();
				}
			}
		}

		// Start async initialization
		init();

		// Expose mobile device override for debugging
		import('$lib/utils/stores/device').then(({ setMobileDeviceOverride }) => {
			(window as any).setMobileMode = setMobileDeviceOverride;
			console.log(
				'🛠️ [debug] Mobile device override available via window.setMobileMode(true/false/null)'
			);
			console.log('🛠️ [debug] URL override: add ?mobile=1 or ?mobile=0 to the URL');
		});
	});

	// Defer socket connection until after initial render
	onMount(async () => {
		// Wait for initial render to complete
		await tick();
		// Now establish socket connection
		connect();
	});

	// Background preload components after critical path is complete
	onMount(async () => {
		// Wait for critical path to complete
		await tick();

		// Use requestIdleCallback for true background loading
		if (browser && 'requestIdleCallback' in window) {
			requestIdleCallback(() => {
				preloadComponents();
			});
		} else {
			// Fallback for browsers without requestIdleCallback
			setTimeout(() => {
				preloadComponents();
			}, 2000); // 2 seconds after page is interactive
		}
	});

	async function preloadComponents() {
		try {
			const preloadPromises = [
				import('$lib/features/screener/screener.svelte'),
				import('$lib/features/strategies/strategies.svelte'),
				import('$lib/features/settings/settings.svelte')
			];

			// Await them to know when done (optional)
			await Promise.all(preloadPromises);
		} catch (error) {}
	}

	onDestroy(() => {
		// Clean up all activity listeners
		if (browser && document) {
			// Remove global keyboard event listener using the stable function reference
			document.removeEventListener('keydown', keydownHandler);
			// Remove overscroll prevention listeners
			document.removeEventListener('touchstart', preventOverscroll);
			document.removeEventListener('touchmove', preventOverscroll);
		}
	});

	// Sidebar resizing
	let minWidth = 120; // Default minimum width, will be updated in browser

	function startRightSidebarResize(event: PointerEvent) {
		event.preventDefault();
		const startX = event.clientX;
		const start = parseInt(
			getComputedStyle(document.documentElement).getPropertyValue('--right-sidebar-width'),
			10
		);

		const maxSidebarWidth = Math.min(600, window.innerWidth - SIDEBAR_BUTTONS_WIDTH);

		const onMove = (ev: PointerEvent) => {
			const delta = startX - ev.clientX; // inverse for right sidebar!
			let newWidth = Math.max(start + delta, 0);
			const dynamicMinWidth = window.innerWidth * 0.10; // Calculate minimum width dynamically

			// Handle collapsing logic
			if (newWidth < dynamicMinWidth && lastSidebarMenu !== null) {
				lastSidebarMenu = null;
				menuWidth.set(0);
				document.documentElement.style.setProperty('--right-sidebar-width', '0px');
			}
			// Restore state if dragging back
			else if (newWidth >= dynamicMinWidth && lastSidebarMenu) {
				newWidth = Math.min(newWidth, maxSidebarWidth);
				lastSidebarMenu = null;
				menuWidth.set(newWidth);
				document.documentElement.style.setProperty('--right-sidebar-width', `${newWidth}px`);
			}
			// Normal resize
			else if (newWidth >= dynamicMinWidth) {
				newWidth = Math.min(newWidth, maxSidebarWidth);
				menuWidth.set(newWidth);
				document.documentElement.style.setProperty('--right-sidebar-width', `${newWidth}px`);
			}
		};

		const onUp = () => {
			window.removeEventListener('pointermove', onMove);
			document.body.style.cursor = 'default';
		};

		document.body.style.cursor = 'ew-resize';
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp, { once: true });
	}

	// Bottom windows
	function openBottomWindow(type: BottomWindowType) {
		const existing = bottomWindows.find((w) => w.type === type);
		// Close if same window is clicked
		if (existing) {
			bottomWindowsHeight = 0;
			bottomWindows = [];
			return;
		}

		// Preserve current height if another window is already open
		const currentHeight = bottomWindows.length > 0 ? bottomWindowsHeight : 200; // Use default only if no window is open

		// Replace current if a different window is clicked
		bottomWindowsHeight = currentHeight;
		bottomWindows = [
			{
				id: nextWindowId++,
				type,
				x: 0,
				y: 0,
				width: window.innerWidth,
				height: bottomWindowsHeight,
				visible: true
			}
		];
	}

	function minimizeBottomWindow() {
		lastBottomWindow = null; // Clear stored state
		bottomWindowsHeight = 0;
		bottomWindows = [];
	}

	// Draggable logic for popups (if needed)
	let draggingWindowId: number | null = null;
	let offsetX = 0;
	let offsetY = 0;

	function onDrag(event: MouseEvent) {
		if (draggingWindowId === null) return;
		const w = bottomWindows.find((win) => win.id === draggingWindowId);
		if (!w) return;
		w.x = event.clientX - offsetX;
		w.y = event.clientY - offsetY;
		bottomWindows = [...bottomWindows];
	}

	function stopDrag() {
		draggingWindowId = null;
	}

	// Replay controls
	function handlePlay() {
		if (!$streamInfo.replayActive) {
			queryInstanceInput(['timestamp'], ['timestamp'], { timestamp: 0, extendedHours: false })
				.then((v: Instance) => {
					startReplay(v);
				})
				.catch(() => {});
		} else {
			if ($streamInfo.replayPaused) {
				resumeReplay();
			}
		}
	}

	function handleCalendar() {
		calendarVisible = true;
	}

	function handlePause() {
		if ($streamInfo.replayActive && !$streamInfo.replayPaused) {
			pauseReplay();
		}
	}

	function handleStop() {
		stopReplay();
	}

	function handleReset() {
		if ($streamInfo.replayActive) {
			stopReplay();
			startReplay({
				timestamp: $streamInfo.startTimestamp,
				extendedHours: $streamInfo.extendedHours
			});
		}
	}

	function handleNextDay() {
		if ($streamInfo.replayActive) {
			nextDay();
		}
	}

	function handleChangeSpeed(event: Event) {
		const val = parseFloat((event.target as HTMLInputElement).value);
		if (!isNaN(val) && val > 0) {
			changeSpeed(val);
			replaySpeed = val;
		}
	}

	// Settings popup
	let showSettingsPopup = false;
	function toggleSettings() {
		showSettingsPopup = !showSettingsPopup;
	}

	// Bottom resizing
	function startBottomResize(event: MouseEvent) {
		event.preventDefault();
		bottomResizing = true;
		document.addEventListener('mousemove', handleBottomResize);
		document.addEventListener('mouseup', stopBottomResize);
		document.body.style.cursor = 'ns-resize';
	}

	function handleBottomResize(event: MouseEvent) {
		if (!bottomResizing) return;

		// Get container dimensions
		const containerBottom = window.innerHeight - 40; // 40px is bottom-bar height
		const maxWidth = Math.max(window.innerWidth * 0.4, 600); // Reduce to 40% of window width

		// Calculate new height and constrain horizontal position
		const newHeight = containerBottom - event.clientY;
		const newX = Math.min(event.clientX, maxWidth);

		if (newHeight < MIN_BOTTOM_HEIGHT && bottomWindows.length > 0) {
			// Store state before closing
			lastBottomWindow = bottomWindows[0];
			bottomWindowsHeight = 0;
			bottomWindows = [];
		}
		// Restore state if dragging back
		else if (newHeight >= MIN_BOTTOM_HEIGHT && lastBottomWindow) {
			bottomWindowsHeight = newHeight;
			bottomWindows = [
				{
					...lastBottomWindow,
					width: Math.min(window.innerWidth, maxWidth),
					x: Math.min(lastBottomWindow.x, maxWidth - 100) // Ensure handle is always reachable
				}
			];
			lastBottomWindow = null;
		}
		// Normal resize
		else if (newHeight >= MIN_BOTTOM_HEIGHT && newHeight <= MAX_BOTTOM_HEIGHT) {
			bottomWindowsHeight = newHeight;
			// Update window width if it exists
			if (bottomWindows.length > 0) {
				bottomWindows = bottomWindows.map((w) => ({
					...w,
					width: Math.min(window.innerWidth, maxWidth),
					x: Math.min(w.x, maxWidth - 100) // Ensure handle is always reachable
				}));
			}
		}
	}

	function stopBottomResize() {
		bottomResizing = false;
		document.removeEventListener('mousemove', handleBottomResize);
		document.removeEventListener('mouseup', stopBottomResize);
		document.body.style.cursor = 'default';
	}

	// Add reactive statement for profile display
	$: {
		// Recalculate the profile display whenever these values change
		if (profilePic || profilePicError) {
			currentProfileDisplay = calculateProfileDisplay();
		}
	}

	function calculateProfileDisplay() {
		// If profile pic is available and no loading error, use it
		if (profilePic && !profilePicError) {
			return profilePic;
		}

		// Generate default avatar with '?' initial since we no longer have username
		{
			const initial = '?';
			// Use a simpler SVG format to ensure browser compatibility
			const avatar = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">${initial}</text></svg>`;

			// Update the profilePic value so we don't regenerate each time
			profilePic = avatar;
			if (browser) {
				sessionStorage.setItem('profilePic', avatar);
			}

			return avatar;
		}
	}

	// Keep the getProfileDisplay function for backward compatibility
	function getProfileDisplay() {
		return currentProfileDisplay;
	}

	function handleProfilePicError() {
		profilePicError = true;

		// Generate a fallback immediately
		// Generate default avatar with '?' initial since we no longer have username
		{
			const initial = '?';
			profilePic = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">${initial}</text></svg>`;
		}

		// Update the stored value with our fallback
		if (browser) {
			sessionStorage.setItem('profilePic', profilePic);
		}

		// Force refresh
		currentProfileDisplay = profilePic;
		profileIconKey++;
	}

	function startSidebarResize(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		sidebarResizing = true;
		document.body.style.cursor = 'ns-resize';
		document.addEventListener('mousemove', handleRightSidebarMenusResize);
		document.addEventListener('mouseup', stopSidebarResize);
		document.addEventListener('touchmove', handleRightSidebarMenusResize);
		document.addEventListener('touchend', stopSidebarResize);
	}

	function handleRightSidebarMenusResize(event: MouseEvent | TouchEvent) { // for tickerinfo/quote 
		if (!sidebarResizing) return;

		let currentY;
		if (event instanceof MouseEvent) {
			currentY = event.clientY;
		} else {
			currentY = event.touches[0].clientY;
		}

		// Account for the bottom bar height (40px) and calculate height from the bottom
		// Since quote is now on bottom, we calculate height from the bottom up
		const bottomBarHeight = 40;
		const newHeight = window.innerHeight - currentY - bottomBarHeight;

		// Clamp the height between min and max values
		tickerInfoContainerHeight = Math.min(Math.max(newHeight, MIN_TICKER_INFO_CONTAINER_HEIGHT), MAX_TICKER_INFO_CONTAINER_HEIGHT);

		// Update the CSS variable
		document.documentElement.style.setProperty('--ticker-info-container-height', `${tickerInfoContainerHeight}px`);
	}

	function stopSidebarResize() {
		if (!browser) return;

		sidebarResizing = false;
		document.body.style.cursor = '';
		document.removeEventListener('mousemove', handleRightSidebarMenusResize);
		document.removeEventListener('mouseup', stopSidebarResize);
		document.removeEventListener('touchmove', handleRightSidebarMenusResize);
		document.removeEventListener('touchend', stopSidebarResize);
	}

	// Add reactive statements to update the profile icon when data changes
	$: if (profilePic) {
		// Increment key to force re-render when profile data changes
		profileIconKey++;
	}

	function handleKeyboardBottomResize(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			startBottomResize(new MouseEvent('mousedown'));
		}
	}

	function handleKeyboardSidebarResize(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			startSidebarResize(new MouseEvent('mousedown'));
		}
	}

	function handleKeyboardResize(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			startRightSidebarResize(new PointerEvent('pointerdown'));
		}
	}

	function handleKeyboardLeftResize(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			startLeftSidebarResize(new PointerEvent('pointerdown'));
		}
	}

	// Left sidebar resizing
	function startLeftSidebarResize(event: PointerEvent) {
		event.preventDefault();
		const startX = event.clientX;
		const start = parseInt(
			getComputedStyle(document.documentElement).getPropertyValue('--left-sidebar-width'),
			10
		);

		// Constraints
		const minLeftSidebarWidth = window.innerWidth * 0.15;
		const maxLeftSidebarWidth = window.innerWidth*0.3;

		const onMove = (ev: PointerEvent) => {
			const delta = ev.clientX - startX;
			const newWidth = Math.round(
				Math.max(minLeftSidebarWidth, Math.min(start + delta, maxLeftSidebarWidth))
			);

			// Update CSS custom property
			document.documentElement.style.setProperty('--left-sidebar-width', `${newWidth}px`);

			// Update store for other components
			leftMenuWidth.set(newWidth);
		};

		const onUp = () => {
			window.removeEventListener('pointermove', onMove);
			document.body.style.cursor = 'default';
		};

		document.body.style.cursor = 'ew-resize';
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp, { once: true });
	}

	// Toggle left pane for Query
	function toggleLeftSidebar() {
		if ($leftMenuWidth > 0) {
			leftMenuWidth.set(0);
			document.documentElement.style.setProperty('--left-sidebar-width', '0px');
		} else {
			// Set to 30% of screen width when opening
			const width = window.innerWidth * 0.3;
			leftMenuWidth.set(width);
			document.documentElement.style.setProperty('--left-sidebar-width', `${width}px`);
		}
	}
	function toggleMainSidebar(menuName: Menu) {
		if (menuName === $activeMenu) {
			// If clicking the same menu, close it
			lastSidebarMenu = null;
			menuWidth.set(0);
			document.documentElement.style.setProperty('--right-sidebar-width', '0px');
			changeMenu('none');
		} else {
			// Open new menu
			lastSidebarMenu = null;
			// Only set width if sidebar is currently closed, otherwise preserve current width
			if ($menuWidth === 0) {
				const width = 180;
				menuWidth.set(width);
				document.documentElement.style.setProperty('--right-sidebar-width', `${width}px`);
			}
			changeMenu(menuName);
		}
	}
	// Subscribe to the requestChatOpen store
	$: if ($requestChatOpen && browser) {
		if ($leftMenuWidth === 0) {
			toggleLeftSidebar(); // Open the left pane if closed
		}
		// Reset the trigger after handling
		// Use setTimeout to ensure the pane opens before resetting,
		// although direct reset might work fine with Svelte's reactivity.
		setTimeout(() => {
			requestChatOpen.set(false);
		}, 0);
	}

	let alertsComponent: any; // Reference to the Alerts component

	async function createPriceAlert() {
		if (alertsComponent) {
			alertsComponent.showPriceForm();
		}
	}

	async function createStrategyAlert() {
		if (alertsComponent) {
			alertsComponent.showStrategyForm();
		}
	}

	// Stripe-recommended pattern: verify checkout session and update subscription status
	async function verifyAndUpdateSubscriptionStatus(sessionId: string) {
		console.log(
			'🔍 [verifyAndUpdateSubscriptionStatus] Starting verification for session:',
			sessionId
		);

		try {
			// Verify the checkout session directly with Stripe via our backend
			const verificationResult = await privateRequest<{
				status: string;
				isActive: boolean;
				currentPlan: string;
				hasCustomer: boolean;
				hasSubscription: boolean;
				currentPeriodEnd: number | null;
				subscriptionCreditsRemaining: number;
				purchasedCreditsRemaining: number;
				totalCreditsRemaining: number;
				subscriptionCreditsAllocated: number;
			}>('verifyCheckoutSession', { sessionId });
			console.log(
				'✅ [verifyAndUpdateSubscriptionStatus] Verification result:',
				verificationResult
			);

			// Refresh subscription status to ensure UI is up to date
			await fetchSubscriptionStatus();

			console.log('🎉 [verifyAndUpdateSubscriptionStatus] Subscription verification completed');
		} catch (error) {
			console.error(
				'❌ [verifyAndUpdateSubscriptionStatus] Error verifying checkout session:',
				error
			);
			// Fallback to simple refresh
			console.log(
				'🔄 [verifyAndUpdateSubscriptionStatus] Falling back to simple subscription refresh'
			);
			await fetchSubscriptionStatus();
		}
	}

	// Update the site title to reflect the active chart's ticker (or fallback when none)
	$: if (browser) {
		const siteTitle = $activeChartInstance?.ticker + ' | Peripheral';
		if (document.title !== siteTitle) {
			document.title = siteTitle;
		}
	}

	// Default profile picture generation
	let profilePic = '';
	let profilePicError = false;
	let userEmail = '';

	onMount(async () => {
		const authToken = sessionStorage.getItem('authToken');
		if (authToken) {
		}
	});

	// Generate initial avatar SVG from email address
	function generateInitialAvatar(email: string) {
		const initial = email ? email.charAt(0).toUpperCase() : 'U';
		const colors = ['#3B82F6', '#EF4444', '#10B981', '#F59E0B', '#8B5CF6', '#EC4899'];
		const colorIndex = initial.charCodeAt(0) % colors.length;
		const bgColor = colors[colorIndex];

		return `data:image/svg+xml,${encodeURIComponent(`
			<svg width="32" height="32" viewBox="0 0 32 32" xmlns="http://www.w3.org/2000/svg">
				<rect width="32" height="32" rx="16" fill="${bgColor}"/>
				<text x="16" y="20" text-anchor="middle" fill="white" font-family="Arial, sans-serif" font-size="14" font-weight="bold">${initial}</text>
			</svg>
		`)}`;
	}

	// Update avatar generation logic
	$: if (profilePic || userEmail) {
		// Use profilePic if available, otherwise generate from email
		profilePic = profilePic || generateInitialAvatar(userEmail);
	} else {
		// Default placeholder
		profilePic = generateInitialAvatar('');
	}

	// Update subscription check condition
	$: if (
		browser &&
		sessionStorage.getItem('authToken') &&
		!$subscriptionStatus.isActive &&
		!$subscriptionStatus.loading
	) {
		// ... existing subscription logic ...
	}
</script>

<!-- svelte-ignore a11y-no-noninteractive-element-interactions-->
<div
	class="page"
	role="application"
	tabindex="-1"
	on:keydown={(e) => {
		if (e.key === 'Escape') {
			minimizeBottomWindow();
		}
	}}
>
	<!-- Global Popups (always available) -->
	<Input />
	<RightClick />
	<StrategiesPopup />
	<Calendar bind:visible={calendarVisible} initialTimestamp={$streamInfo.timestamp} />
	<ExtendedHoursToggle
		instance={$activeChartInstance || {}}
		visible={$extendedHoursToggleVisible}
		on:change={() => hideExtendedHoursToggle()}
		on:close={() => hideExtendedHoursToggle()}
	/>
	<AuthModal
		visible={$authModalStore.visible}
		defaultMode={$authModalStore.mode}
		on:success={() => {
			hideAuthModal();
		}}
		on:close={hideAuthModal}
	/>

	{#if $isMobileDevice}
		<!-- Mobile-only full-screen chat interface -->
		<div class="mobile-chat-container">
			<!-- Mobile banner at the top -->
			{#if showMobileBanner}
				<MobileBanner on:dismiss={dismissMobileBanner} />
			{/if}
			<Query isPublicViewing={$isPublicViewingStore} {sharedConversationId} />
		</div>
	{:else}
		<!-- Desktop interface -->

		<!-- Main area wrapper -->
		<div class="app-container">
			<div class="content-wrapper">
				<!-- Main horizontal container -->
				<div class="main-horizontal-container">
					<!-- Left sidebar for Query -->
					{#if $leftMenuWidth > 0}
						<div class="left-sidebar">
							<div class="sidebar-content">
								<Query isPublicViewing={$isPublicViewingStore} {sharedConversationId} />
							</div>
						</div>
						<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
						<div
							class="resizer-left"
							role="separator"
							aria-orientation="vertical"
							aria-label="Resize left panel"
							on:pointerdown={startLeftSidebarResize}
							on:keydown={handleKeyboardLeftResize}
							tabindex="0"
						/>
					{/if}
					<!-- Center section (chart + top bar) -->
					<div class="center-section">
						<!-- Top bar -->
						<TopBar instance={$activeChartInstance || {}} {handleCalendar} />

						<!-- Main content area -->
						<div class="main-content">
							<!-- Chart area -->
							<div class="chart-wrapper">
								<ChartContainer defaultChartData={data.defaultChartData} />
							</div>

							<!-- Bottom windows container -->
							<div
								class="bottom-windows-container"
								style="--bottom-height: {bottomWindowsHeight}px"
							>
								{#each bottomWindows as w}
									<div class="bottom-window">
										<div class="window-content">
											{#if w.type === 'screener'}
												{#await import('$lib/features/screener/screener.svelte') then module}
													<svelte:component this={module.default} />
												{/await}
											{:else if w.type === 'strategies'}
												{#await import('$lib/features/strategies/strategies.svelte') then module}
													<svelte:component this={module.default} />
												{/await}
											{:else if w.type === 'settings'}
												{#await import('$lib/features/settings/settings.svelte') then module}
													<svelte:component this={module.default} />
												{/await}
											{/if}
										</div>
									</div>
								{/each}
								{#if bottomWindows.length > 0}
									<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
									<div
										class="bottom-resize-handle"
										role="separator"
										aria-orientation="horizontal"
										aria-label="Resize bottom panel"
										on:mousedown={startBottomResize}
										on:keydown={handleKeyboardBottomResize}
										tabindex="0"
									></div>
								{/if}
							</div>
						</div>
					</div>

					<!-- Sidebar -->
					{#if $menuWidth > 0}
						<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
						<div
							class="resizer-right"
							role="separator"
							aria-orientation="vertical"
							aria-label="Resize sidebar"
							on:pointerdown={startRightSidebarResize}
							on:keydown={handleKeyboardResize}
							tabindex="0"
						/>
						<div class="sidebar">
							<!-- Sidebar header -->
							<div class="sidebar-header">
								{#if $activeMenu === 'alerts'}
									<!-- Alert Controls -->
									<div class="alert-tab-container">
										{#each alertTabs as tab}
											<button
												class="watchlist-tab {alertView === tab ? 'active' : ''}"
												on:click={() => (alertView = tab)}
												title={tab.charAt(0).toUpperCase() + tab.slice(1) + ' Alerts'}
											>
												{tab.charAt(0).toUpperCase() + tab.slice(1)}
											</button>
										{/each}

										{#if alertView !== 'logs'}
											<button
												class="watchlist-tab plus-button"
												on:click={() => {
													if (alertView === 'price') createPriceAlert();
													else if (alertView === 'strategy') createStrategyAlert();
												}}
												title="Create New {alertView === 'price' ? 'Price' : 'Strategy'} Alert"
												style="margin-left: auto;"
											>
												+
											</button>
										{/if}
									</div>
								{/if}
								{#if $activeMenu === 'watchlist'}
									<WatchlistTabs />
								{/if}
							</div>
							<div class="sidebar-content">
								<div class="main-sidebar-content">
									{#if $activeMenu === 'watchlist'}
										<Watchlist showTabs={false} />
									{:else if $activeMenu === 'alerts'}
										<Alerts bind:this={alertsComponent} view={alertView} />
										<!--{:else if $activeMenu === 'news'}
									<News />-->
									{/if}
								</div>

								<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
								<div
									class="sidebar-resize-handle"
									role="separator"
									aria-orientation="horizontal"
									aria-label="Resize quote panel"
									on:mousedown={startSidebarResize}
									on:touchstart|preventDefault={startSidebarResize}
									on:keydown={handleKeyboardSidebarResize}
									tabindex="0"
								></div>

								<!-- Quote section now on bottom -->
								<div class="ticker-info-container" style="height: {tickerInfoContainerHeight}px">
									<Quote />
								</div>
							</div>
						</div>
					{/if}
				</div>
			</div>

			<!-- Sidebar toggle buttons -->
			<div class="sidebar-buttons">
				{#each sidebarMenus as menu}
					<button
						class="toggle-button side-btn {$activeMenu === menu ? 'active' : ''}"
						on:click={() => toggleMainSidebar(menu)}
						title={menu.charAt(0).toUpperCase() + menu.slice(1)}
					>
						<img src="{menu}.png" alt={menu} class="menu-icon" />
					</button>
				{/each}
			</div>
		</div>

		<!-- Bottom bar -->
		<div class="bottom-bar">
			<div class="bottom-bar-left">
				<button
					class="toggle-button query-feature {$leftMenuWidth > 0 ? 'active' : ''}"
					on:click={toggleLeftSidebar}
					title="Open AI Chat"
				>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="16"
						height="16"
						fill="currentColor"
						class="chat-icon bi bi-square-half"
						viewBox="0 0 16 16"
					>
						<path
							d="M8 15V1h6a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1zm6 1a2 2 0 0 0 2-2V2a2 2 0 0 0-2-2H2a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2z"
						/>
					</svg>
				</button>

				<!-- 
				<button
					class="toggle-button {bottomWindows.some((w) => w.type === 'strategies') ? 'active' : ''}"
					on:click={() => openBottomWindow('strategies')}
				>
					Strategies
				</button>
				<button
					class="toggle-button {bottomWindows.some((w) => w.type === 'screener') ? 'active' : ''}"
					on:click={() => openBottomWindow('screener')}
				>
					Screener
				</button>
				-->
			</div>

			<div class="bottom-bar-right">
				<!-- Replay buttons commented out -->
				<!-- 
			<button
				class="toggle-button replay-button {!$streamInfo.replayActive || $streamInfo.replayPaused
					? 'play'
					: 'pause'}"
				on:click={() => {
					if (!$streamInfo.replayActive) {
						handlePlay();
					} else if ($streamInfo.replayPaused) {
						handlePlay();
					} else {
						handlePause();
					}
				}}
				title={$streamInfo.replayActive && !$streamInfo.replayPaused
					? 'Pause Replay'
					: 'Start/Resume Replay'}
			>
				{#if !$streamInfo.replayActive}
					<svg viewBox="0 0 24 24"><path d="M8,5.14V19.14L19,12.14L8,5.14Z" /></svg>
					<span>Replay</span>
				{:else if $streamInfo.replayPaused}
					<svg viewBox="0 0 24 24"><path d="M8,5.14V19.14L19,12.14L8,5.14Z" /></svg>
					<span>Play</span>
				{:else}
					<svg viewBox="0 0 24 24"><path d="M14,19H18V5H14M6,19H10V5H6V19Z" /></svg>
					<span>Pause</span>
				{/if}
			</button>

			{#if $streamInfo.replayActive}
				<button class="toggle-button replay-button stop" on:click={handleStop} title="Stop Replay">
					<svg viewBox="0 0 24 24"><path d="M18,18H6V6H18V18Z" /></svg>
				</button>
				<button
					class="toggle-button replay-button reset"
					on:click={handleReset}
					title="Reset Replay"
				>
					<svg viewBox="0 0 24 24"
						><path
							d="M12,5V1L7,6L12,11V8C15.31,8 18,10.69 18,14C18,17.31 15.31,20 12,20C8.69,20 6,17.31 6,14H4C4,18.42 7.58,22 12,22C16.42,22 20,18.42 20,14C20,9.58 16.42,6 12,6V5Z"
						/></svg
					>
				</button>
				<button
					class="toggle-button replay-button next-day"
					on:click={handleNextDay}
					title="Next Day"
				>
					<svg viewBox="0 0 24 24"
						><path
						/></svg
					>
				</button>

				<label class="speed-label">
					Speed:
					<input
						type="number"
						step="0.1"
						min="0.1"
						value={replaySpeed}
						on:input={handleChangeSpeed}
						class="speed-input"
					/>
				</label>
			{/if}
			-->

				<span class="value">
					{#if $streamInfo.timestamp !== undefined}
						{formatTimestamp($streamInfo.timestamp)}
					{:else}
						Loading Time...
					{/if}
				</span>
				<!-- Site logo (clickable) -->
				<button
					class="bottom-logo-link"
					on:click={() => (window.location.href = '/')}
					aria-label="Go to home page"
				>
					<img src="/atlantis_logo_transparent.png" alt="Logo" class="bottom-logo" />
				</button>
			</div>
		</div>

		{#if showSettingsPopup}
			<div
				class="settings-overlay"
				role="dialog"
				aria-label="Settings"
				on:click|self={toggleSettings}
				on:keydown={(e) => {
					if (e.key === 'Escape') {
						toggleSettings();
					}
				}}
			>
				<div class="settings-modal">
					<div class="settings-header">
						<h2>Settings</h2>
						<button class="close-btn" on:click={toggleSettings}>×</button>
					</div>
					<div class="settings-content">
						{#if showSettingsPopup}
							{#await import('$lib/features/settings/settings.svelte') then module}
								<svelte:component this={module.default} initialTab={activeTab} />
							{/await}
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- Profile bar (top-right) -->
		<div class="profile-bar">
			<button class="profile-button" on:click={toggleSettings} aria-label="Toggle Settings">
				{#key profileIconKey}
					<img
						src={getProfileDisplay()}
						alt="Profile"
						class="pfp"
						on:error={handleProfilePicError}
					/>
				{/key}
			</button>
		</div>
	{/if}
	<!-- End desktop interface -->
</div>

<style>
	:root {
		--left-sidebar-width: 0px;
		--right-sidebar-width: 0px;
		--left-gutter: clamp(0px, var(--left-sidebar-width), 4px);
		--right-gutter: clamp(0px, var(--right-sidebar-width), 4px);
		--gutter: 4px;
	}

	/* Profile bar container */
	.profile-bar {
		position: fixed;
		top: 0;
		right: 0;
		width: var(--sidebar-buttons-width, 45px); /* use CSS variable with fallback */
		height: 40px; /* same height as top bar */
		background-color: #121212;
		display: flex;
		align-items: center;
		justify-content: center;
		border-bottom: 4px solid var(--c1);
		border-left: 4px solid var(--c1);
		z-index: 11; /* above top bar */
	}

	/* Profile picture inside profile bar */
	.profile-bar .pfp {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		cursor: pointer;
		background-color: var(--c3);
		border: 1px solid var(--c4);
		overflow: hidden;
		display: block;
	}

	/* Bottom bar logo */
	.bottom-bar .bottom-logo {
		height: 28px;
		width: auto;
		display: block;
	}

	.bottom-logo-link {
		display: inline-flex;
		align-items: center;
		cursor: pointer;
		background: none;
		border: none;
		padding: 0;
	}

	/* New layout structure styles */
	.main-horizontal-container {
		display: grid;
		height: 100%;
		width: 100%;
		grid-template-columns: var(--left-sidebar-width) var(--left-gutter) 1fr var(--right-gutter) var(--right-sidebar-width);
		grid-template-areas: 'left g1 center g2 right';
	}

	.left-sidebar {
		grid-area: left;
		overflow: hidden;
	}
	.center-section {
		grid-area: center;
		display: flex;
		flex-direction: column;
	}

	.sidebar {
		grid-area: right;
		overflow: hidden;
	}
	.resizer-left {
		width: var(--left-gutter);
		cursor: ew-resize;
		background: transparent;
		z-index: 10; /* sit above charts for easy grab */
	}
	.resizer-right {
		width: var(--right-gutter);
		cursor: ew-resize;
		background: transparent;
		z-index: 10; /* sit above charts for easy grab */
	}
	.resizer-left {
		grid-area: g1;
	}
	.resizer-right {
		grid-area: g2;
	}

	.sidebar-header {
		height: 40px;
		min-height: 40px;
		background-color: #121212;
		display: flex;
		align-items: center;
		padding: 0 10px;
		flex-shrink: 0;
		width: 100%;
		z-index: 10;
		border-bottom: 4px solid var(--c1);
	}

	.sidebar-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	/* ───── Alert tab styles ─────────────────────────────────────────────────── */
	.alert-tab-container {
		display: flex;
		align-items: center;
		flex-grow: 1;
		min-width: 0;
		gap: 0;
	}

	.sidebar-header .watchlist-tab {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgba(255, 255, 255, 0.9);
		padding: 6px 12px;
		background: transparent;
		border-radius: 6px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid transparent;
		cursor: pointer;
		transition: none;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.sidebar-header .watchlist-tab:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: transparent;
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	.sidebar-header .watchlist-tab:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.4);
	}

	.sidebar-header .watchlist-tab.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Mobile full-screen chat container */
	.mobile-chat-container {
		position: fixed;
		inset: 0; /* top:0; right:0; bottom:0; left:0; */
		z-index: 9999; /* above everything */
		background: #121212; /* match site background so it feels native */
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	.center-section {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-width: 0; /* Allows flex child to shrink below content size */
	}

	.sidebar {
		display: flex !important;
		flex-direction: column;
		height: 100%;
	}

	.sidebar-header {
		height: 40px;
		min-height: 40px;
		background-color: #121212;
		display: flex;
		align-items: center;
		padding: 0 10px;
		flex-shrink: 0;
		width: 100%;
		z-index: 10;
		border-bottom: 4px solid var(--c1);
	}

	.sidebar-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	/* ───── Alert tab styles ─────────────────────────────────────────────────── */
	.alert-tab-container {
		display: flex;
		align-items: center;
		flex-grow: 1;
		min-width: 0;
		gap: 0;
	}

	.sidebar-header .watchlist-tab {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgba(255, 255, 255, 0.9);
		padding: 6px 12px;
		background: transparent;
		border-radius: 6px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid transparent;
		cursor: pointer;
		transition: none;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.sidebar-header .watchlist-tab:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: transparent;
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	.sidebar-header .watchlist-tab:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.4);
	}

	.sidebar-header .watchlist-tab.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Mobile full-screen chat container */
	.mobile-chat-container {
		position: fixed;
		inset: 0; /* top:0; right:0; bottom:0; left:0; */
		z-index: 9999; /* above everything */
		background: #121212; /* match site background so it feels native */
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}
</style>
