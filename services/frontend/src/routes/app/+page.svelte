<script lang="ts">
	import '$lib/styles/global.css';
	import ChartContainer from '$lib/features/chart/chartContainer.svelte';
	import Alerts from '$lib/features/alerts/alert.svelte';
	import RightClick from '$lib/components/rightClick.svelte';
	import StrategiesPopup from '$lib/components/strategiesPopup.svelte';
	import Input from '$lib/components/input/input.svelte';
	//import Similar from '$lib/features/similar/similar.svelte';
	//import Study from '$lib/features/study.svelte';
	import Watchlist from '$lib/features/watchlist/watchlist.svelte';
	//import TickerInfo from '$lib/features/quotes/tickerInfo.svelte';
	import Quote from '$lib/features/quotes/quote.svelte';
	//import Algo from '$lib/components/algo.svelte';
	import { activeMenu, changeMenu } from '$lib/utils/stores/stores';

	// Windows that will be opened in draggable divs
	import Screener from '$lib/features/screener/screener.svelte';
	import Account from '$lib/features/account/account.svelte';
	import Strategies from '$lib/features/strategies/strategies.svelte';
	import Settings from '$lib/features/settings/settings.svelte';
	import News from '$lib/features/news/news.svelte';
    import Deploy from '$lib/features/deploy/deploy.svelte'
    import Backtest from '$lib/features/backtest/backtest.svelte'

	// Replay logic
	import {
		startReplay,
		stopReplay,
		pauseReplay,
		resumeReplay,
		changeSpeed,
		nextDay
	} from '$lib/utils/stream/interface';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { browser } from '$app/environment';
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import {
		initStores,
		streamInfo,
		formatTimestamp,
		dispatchMenuChange,
		menuWidth,
		settings
	} from '$lib/utils/stores/stores';
	import { writable, type Writable } from 'svelte/store';
	import { colorSchemes, applyColorScheme } from '$lib/styles/colorSchemes';

	// Import Instance from types
	import type { Instance } from '$lib/utils/types/types';

	// Add import near the top with other imports
	import Screensaver from '$lib/features/screensaver/screensaver.svelte';

	// Add new import for Query component
	import Query from '$lib/features/chat/chat.svelte';

	import { requestChatOpen } from '$lib/features/chat/interface'; // Import the store

	// Import the standalone calendar component
	import Calendar from '$lib/components/calendar/calendar.svelte';

	//type Menu = 'none' | 'watchlist' | 'alerts' | 'study' | 'news';
	type Menu = 'none' | 'watchlist' | 'alerts'  | 'news';

	let lastSidebarMenu: Menu | null = null;
	let sidebarWidth = 0;
	//const sidebarMenus: Menu[] = ['watchlist', 'alerts', 'study', 'news'];
	const sidebarMenus: Menu[] = ['watchlist', 'alerts',  'news'];

	// Initialize chartWidth with a default value
	let chartWidth = 0;

	// Bottom windows
	type BottomWindowType =
		| 'screener'
		| 'account'
		//| 'options'
		| 'strategies'
		| 'settings'
        | 'deploy'
        | 'backtest'
		//| 'news'
		| 'query'
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
	let profilePic = '';
	let username = '';
	let profilePicError = false;
	let profileIconKey = 0;
	let currentProfileDisplay = ''; // Add this to hold the current display value

	let sidebarResizing = false;
	let tickerHeight = 500; // Initial height
	const MIN_TICKER_HEIGHT = 100;
	const MAX_TICKER_HEIGHT = 600;

	// Add state variables after other state declarations
	let screensaverActive = false;
	let inactivityTimer: ReturnType<typeof setTimeout> | null = null;
	const INACTIVITY_TIMEOUT = 5 * 1000; // 5 seconds in milliseconds

	// Add left sidebar state variables next to the other state variables
	let leftMenuWidth = 550; // <-- Set initial width to 300
	let leftResizing = false;

	// Calendar state
	let calendarVisible = false;

	// Apply color scheme reactively based on the store
	$: if ($settings.colorScheme && browser) {
		const scheme = colorSchemes[$settings.colorScheme];
		if (scheme) {
			applyColorScheme(scheme);
		}
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

	// Add reactive statement to update chart width when menuWidth changes

	function updateChartWidth() {
		if (browser) {
			const rightSidebarWidth = $menuWidth;
			const maxRightSidebarWidth = Math.min(600, window.innerWidth - 45); // Restored to 600px
			const maxLeftSidebarWidth = Math.min(800, window.innerWidth - 45);

			// Only reduce chart width if sidebar widths are within bounds
			if (rightSidebarWidth <= maxRightSidebarWidth && leftMenuWidth <= maxLeftSidebarWidth) {
				chartWidth = window.innerWidth - rightSidebarWidth - leftMenuWidth - 45;
			}
		}
	}

	let keydownHandler: (event: KeyboardEvent) => void;

	onMount(() => {
		// Load profile data FIRST, before doing anything else
		const storedProfilePic = sessionStorage.getItem('profilePic') || '';
		username = sessionStorage.getItem('username') || '';

		// Check if the stored profile pic is a real image URL or a generated SVG
		if (storedProfilePic && !storedProfilePic.startsWith('data:image/svg+xml')) {
			// It's a real image URL (like from Google)
			profilePic = storedProfilePic;
		} else if (storedProfilePic) {
			// It's an SVG - use it directly
			profilePic = storedProfilePic;
		} else if (username) {
			// Generate avatar based on username
			const initial = username.charAt(0).toUpperCase();
			profilePic = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">${initial}</text></svg>`;

			// Store it for future use
			sessionStorage.setItem('profilePic', profilePic);
		} else {
			// No username available, use a more visible question mark
			profilePic = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">?</text></svg>`;
		}

		// Reset error state
		profilePicError = false;

		// Set up a single menuWidth subscription
		const unsubscribe = menuWidth.subscribe((width) => {
			updateChartWidth();
		});

		if (browser) {
			document.title = 'Atlantis';
			// Set initial state once
			lastSidebarMenu = null;
			menuWidth.set(0);

			updateChartWidth();
			window.addEventListener('resize', updateChartWidth);

			// Define the keydown handler
			keydownHandler = (event: KeyboardEvent) => {
				// Check if input window is active - don't handle keyboard events when input is active
				const inputWindow = document.getElementById('input-window');
				const hiddenInput = document.getElementById('hidden-input');

				// Don't interfere with the input component's keyboard events
				//if (inputWindow || hiddenInput === document.activeElement) {
				if (hiddenInput === document.activeElement) {
					return;
				}

				// Only handle events when no element is focused or body is focused
				if (!document.activeElement || document.activeElement === document.body) {
					const chartContainer = document.getElementById(`chart_container-0`); // Assuming first chart has ID 0

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

			// Add global keyboard event listener
			document.removeEventListener('keydown', keydownHandler); // Remove any existing listener first
			document.addEventListener('keydown', keydownHandler);
		}
		privateRequest<string>('verifyAuth', {}).catch(() => {
			goto('/login');
		});
		initStores();

		dispatchMenuChange.subscribe((menuName: string) => {
			if (sidebarMenus.includes(menuName as Menu)) {
				toggleMenu(menuName as Menu);
			}
        });

		// Force profile display to update
		currentProfileDisplay = calculateProfileDisplay();

		// Force refresh of the profile icon
		profileIconKey++;

		// Setup activity listeners
		if (browser) {
			// Use more specific events that indicate user activity
			const activityEvents = ['mousedown', 'mousemove', 'keydown', 'scroll', 'touchstart', 'click'];

			// Remove any existing listeners first to avoid duplicates
			activityEvents.forEach((event) => {
				document.removeEventListener(event, resetInactivityTimer);
			});

			// Add the listeners
			activityEvents.forEach((event) => {
				document.addEventListener(event, resetInactivityTimer);
			});

			// Initialize the timer
			resetInactivityTimer();
		}

		// Clean up subscription on component destroy
		return () => {
			unsubscribe();
		};
	});

	onDestroy(() => {
		if (inactivityTimer) {
			clearTimeout(inactivityTimer);
		}

		// Clean up all activity listeners
		if (browser && document) {
			window.removeEventListener('resize', updateChartWidth);
			// Remove global keyboard event listener using the stored handler
			document.removeEventListener('keydown', keydownHandler);
			stopSidebarResize();
			stopLeftResize();

			// Clean up all activity listeners
			const activityEvents = ['mousedown', 'mousemove', 'keydown', 'scroll', 'touchstart', 'click'];
			activityEvents.forEach((event) => {
				document.removeEventListener(event, resetInactivityTimer);
			});
		}
	});

	function toggleMenu(menuName: Menu) {
		if (menuName === $activeMenu) {
			// If clicking the same menu, close it
			lastSidebarMenu = null;
			menuWidth.set(0);
			changeMenu('none');
		} else {
			// Open new menu
			lastSidebarMenu = null;
			menuWidth.set(180); // Reduced from 225 to 180 (smaller sidebar)
			changeMenu(menuName);
		}

		if (browser) {
			document.title =
				menuName === 'none'
					? 'Atlantis'
					: `${menuName.charAt(0).toUpperCase() + menuName.slice(1)} - Atlantis`;
		}
	}

	// Sidebar resizing
	let resizing = false;
	let minWidth = 120; // Reduced from 150 to 120 (smaller minimum)
	let maxWidth = 600; // Restored to 600px maximum

	function startResize(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		resizing = true;
		document.addEventListener('mousemove', resize);
		document.addEventListener('mouseup', stopResize);
		document.addEventListener('touchmove', resize);
		document.addEventListener('touchend', stopResize);
		document.body.style.cursor = 'ew-resize';
	}

	function resize(event: MouseEvent | TouchEvent) {
		if (!resizing) return;

		let clientX = 0;
		if (event instanceof MouseEvent) {
			clientX = event.clientX;
		} else {
			clientX = event.touches[0].clientX;
		}

		// Calculate width from right edge of window, excluding the sidebar buttons width
		let newWidth = window.innerWidth - clientX - 45; // 45px is the width of sidebar buttons
		const maxSidebarWidth = Math.min(600, window.innerWidth - 45); // Restored to 600px max

		// Store state before closing
		if (newWidth < minWidth && lastSidebarMenu !== null) {
			lastSidebarMenu = null;
			menuWidth.set(0);
		}
		// Restore state if dragging back
		else if (newWidth >= minWidth && lastSidebarMenu) {
			lastSidebarMenu = lastSidebarMenu;
			menuWidth.set(Math.min(newWidth, maxSidebarWidth));
			lastSidebarMenu = null;
		}
		// Normal resize
		else if (newWidth >= minWidth) {
			// Only update if we're within the maximum width
			menuWidth.set(Math.min(newWidth, maxSidebarWidth));
		}

		// Only update chart width if we're within bounds
		if (newWidth <= maxSidebarWidth) {
			updateChartWidth();
		}
	}

	function stopResize() {
		resizing = false;
		document.removeEventListener('mousemove', resize);
		document.removeEventListener('mouseup', stopResize);
		document.removeEventListener('touchmove', resize);
		document.removeEventListener('touchend', stopResize);
		document.body.style.cursor = 'default';
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

	function startDrag(event: MouseEvent, windowId: number) {
		draggingWindowId = windowId;
		offsetX = event.offsetX;
		offsetY = event.offsetY;
	}

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

		updateChartWidth();
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
		if (profilePic || username || profilePicError) {
			currentProfileDisplay = calculateProfileDisplay();
		}
	}

	function calculateProfileDisplay() {
		// If profile pic is available and no loading error, use it
		if (profilePic && !profilePicError) {
			return profilePic;
		}

		// If username is available, generate avatar with initial
		if (username) {
			const initial = username.charAt(0).toUpperCase();
			// Use a simpler SVG format to ensure browser compatibility
			const avatar = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">${initial}</text></svg>`;

			// Update the profilePic value so we don't regenerate each time
			profilePic = avatar;
			if (browser) {
				sessionStorage.setItem('profilePic', avatar);
			}

			return avatar;
		}

		// Fallback if nothing else is available - improved visibility with simpler SVG format
		// Use a simpler SVG format to ensure browser compatibility
		const fallbackAvatar = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">?</text></svg>`;

		// Store this fallback
		profilePic = fallbackAvatar;

		return fallbackAvatar;
	}

	// Keep the getProfileDisplay function for backward compatibility
	function getProfileDisplay() {
		return currentProfileDisplay;
	}

	function handleProfilePicError() {
		profilePicError = true;

		// Generate a fallback immediately
		if (username) {
			const initial = username.charAt(0).toUpperCase();
			profilePic = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">${initial}</text></svg>`;
		} else {
			profilePic = `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="14" r="14" fill="%232a2e36"/><text x="14" y="19" font-family="Arial" font-size="14" fill="white" text-anchor="middle" font-weight="bold">?</text></svg>`;
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
		document.addEventListener('mousemove', handleSidebarResize);
		document.addEventListener('mouseup', stopSidebarResize);
		document.addEventListener('touchmove', handleSidebarResize);
		document.addEventListener('touchend', stopSidebarResize);
	}

	function handleSidebarResize(event: MouseEvent | TouchEvent) {
		if (!sidebarResizing) return;

		let currentY;
		if (event instanceof MouseEvent) {
			currentY = event.clientY;
		} else {
			currentY = event.touches[0].clientY;
		}

		// Account for the bottom bar height (40px) and adjust for the drag handle position
		const bottomBarHeight = 40;
		const newHeight = window.innerHeight - currentY - bottomBarHeight;

		// Clamp the height between min and max values
		tickerHeight = Math.min(Math.max(newHeight, MIN_TICKER_HEIGHT), MAX_TICKER_HEIGHT);

		// Update the CSS variable
		document.documentElement.style.setProperty('--ticker-height', `${tickerHeight}px`);
	}

	function stopSidebarResize() {
		if (!browser) return;

		sidebarResizing = false;
		document.body.style.cursor = '';
		document.removeEventListener('mousemove', handleSidebarResize);
		document.removeEventListener('mouseup', stopSidebarResize);
		document.removeEventListener('touchmove', handleSidebarResize);
		document.removeEventListener('touchend', stopSidebarResize);
	}

	// Add function after other function declarations
	function resetInactivityTimer() {
		if (inactivityTimer) {
			clearTimeout(inactivityTimer);
		}
		if (!screensaverActive) {
			inactivityTimer = setTimeout(() => {
				// Only activate screensaver, don't hide the chart
				screensaverActive = true;
			}, INACTIVITY_TIMEOUT);
		}
	}

	function toggleScreensaver() {
		screensaverActive = !screensaverActive;
		// If turning off screensaver, reset the inactivity timer
		if (!screensaverActive) {
			resetInactivityTimer();
		}
	}

	// Add reactive statements to update the profile icon when data changes
	$: if (profilePic || username) {
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
			startResize(new MouseEvent('mousedown'));
		}
	}

	function handleKeyboardLeftResize(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ' ') {
			e.preventDefault();
			startLeftResize(new MouseEvent('mousedown'));
		}
	}

	// Left sidebar resizing
	function startLeftResize(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		leftResizing = true;
		document.addEventListener('mousemove', resizeLeft);
		document.addEventListener('mouseup', stopLeftResize);
		document.addEventListener('touchmove', resizeLeft);
		document.addEventListener('touchend', stopLeftResize);
		document.body.style.cursor = 'ew-resize';
	}

	function resizeLeft(event: MouseEvent | TouchEvent) {
		if (!leftResizing) return;

		let clientX = 0;
		if (event instanceof MouseEvent) {
			clientX = event.clientX;
		} else {
			clientX = event.touches[0].clientX;
		}

		// Calculate width from left edge of window
		let newWidth = clientX;
		const maxLeftSidebarWidth = Math.min(800, window.innerWidth - 45);

		// Manage resize
		if (newWidth < minWidth) {
			leftMenuWidth = 0;
		} else {
			leftMenuWidth = Math.min(newWidth, maxLeftSidebarWidth);
		}

		updateChartWidth();
	}

	function stopLeftResize() {
		leftResizing = false;
		document.removeEventListener('mousemove', resizeLeft);
		document.removeEventListener('mouseup', stopLeftResize);
		document.removeEventListener('touchmove', resizeLeft);
		document.removeEventListener('touchend', stopLeftResize);
		document.body.style.cursor = 'default';
	}

	// Toggle left pane for Query
	function toggleLeftPane() {
		if (leftMenuWidth > 0) {
			leftMenuWidth = 0;
		} else {
			leftMenuWidth = 300;
		}
		updateChartWidth();
	}

	// Subscribe to the requestChatOpen store
	$: if ($requestChatOpen && browser) {
		if (leftMenuWidth === 0) {
			toggleLeftPane(); // Open the left pane if closed
		}
		// Reset the trigger after handling
		// Use setTimeout to ensure the pane opens before resetting,
		// although direct reset might work fine with Svelte's reactivity.
		setTimeout(() => {
			requestChatOpen.set(false);
		}, 0);
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
	<!-- Global Popups -->
	<Input />
	<RightClick />
	<StrategiesPopup />
	<Calendar 
		bind:visible={calendarVisible} 
		initialTimestamp={$streamInfo.timestamp}
	/>
	<!--<Algo />-->
	<!-- Main area wrapper -->
	<div class="app-container">
		<div class="content-wrapper">
			<!-- Left sidebar for Query -->
			{#if leftMenuWidth > 0}
				<div class="left-sidebar" style="width: {leftMenuWidth}px;">
					<div class="sidebar-content">
						<div class="main-sidebar-content">
							<Query />
						</div>
					</div>
					<div
						class="resize-handle right"
						role="separator"
						aria-orientation="vertical"
						on:mousedown={startLeftResize}
						on:touchstart={startLeftResize}
						on:keydown={handleKeyboardLeftResize}
						tabindex="0"
					/>
				</div>
			{/if}

			<!-- Main content area -->
			<div class="main-content">
				<!-- Chart area -->
				<div class="chart-wrapper">
					<ChartContainer width={chartWidth} />
					{#if screensaverActive}
						<Screensaver on:exit={() => (screensaverActive = false)} />
					{/if}
				</div>

				<!-- Bottom windows container -->
				<div class="bottom-windows-container" style="--bottom-height: {bottomWindowsHeight}px">
					{#each bottomWindows as w}
						<div class="bottom-window">
							<div class="window-content">
								{#if w.type === 'screener'}
									<Screener />
								{:else if w.type === 'strategies'}
									<Strategies />
								<!-- {:else if w.type === 'account'}
									<Account /> -->
								{:else if w.type === 'settings'}
									<Settings />
								{:else if w.type === 'backtest'}
                                <Backtest/>
								{:else if w.type === 'deploy'}
                                <Deploy/>
								{/if}
							</div>
						</div>
					{/each}
					{#if bottomWindows.length > 0}
						<div
							class="bottom-resize-handle"
							role="separator"
							aria-orientation="horizontal"
							on:mousedown={startBottomResize}
							on:keydown={handleKeyboardBottomResize}
							tabindex="0"
						></div>
					{/if}
				</div>
			</div>

			<!-- Sidebar -->
			{#if $menuWidth > 0}
				<div class="sidebar" style="width: {$menuWidth}px;">
					<div
						class="resize-handle"
						role="separator"
						aria-orientation="vertical"
						on:mousedown={startResize}
						on:touchstart={startResize}
						on:keydown={handleKeyboardResize}
						tabindex="0"
					/>
					<div class="sidebar-content">
						<!-- Main sidebar content -->
						<div class="main-sidebar-content">
							{#if $activeMenu === 'watchlist'}
								<Watchlist />
							{:else if $activeMenu === 'alerts'}
								<Alerts />
							<!--{:else if $activeMenu === 'study'}
								<Study />-->
							{:else if $activeMenu === 'news'}
								<News />
							{/if}
						</div>

						<div
							class="sidebar-resize-handle"
							role="separator"
							aria-orientation="horizontal"
							on:mousedown={startSidebarResize}
							on:touchstart|preventDefault={startSidebarResize}
							on:keydown={handleKeyboardSidebarResize}
							tabindex="0"
						></div>

						<div class="ticker-info-container" style="height: {tickerHeight}px">
							<Quote />
						</div>
					</div>
				</div>
			{/if}
		</div>

		<!-- Sidebar toggle buttons -->
		<div class="sidebar-buttons">
			{#each sidebarMenus as menu}
				<button
					class="toggle-button side-btn {$activeMenu === menu ? 'active' : ''}"
					on:click={() => toggleMenu(menu)}
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
				class="toggle-button query-feature {leftMenuWidth > 0 ? 'active' : ''}"
				on:click={toggleLeftPane}
				title="AI Query"
			>
				<svg class="chat-icon" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M8 12H8.01M12 12H12.01M16 12H16.01M21 12C21 16.418 16.97 20 12 20C10.89 20 9.84 19.8 8.87 19.42L3 21L4.58 15.13C4.2 14.16 4 13.11 4 12C4 7.582 8.03 4 12 4C16.97 4 21 7.582 21 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'strategies') ? 'active' : ''}"
				on:click={() => openBottomWindow('strategies')}
			>
				Strategies
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'backtest') ? 'active' : ''}"
				on:click={() => openBottomWindow('backtest')}
			>
				Backtest
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'screener') ? 'active' : ''}"
				on:click={() => openBottomWindow('screener')}
			>
				Screener
			</button>	<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'deploy') ? 'active' : ''}"
				on:click={() => openBottomWindow('deploy')}
			>
				Deploy
			</button>
			<!-- <button
				class="toggle-button {bottomWindows.some((w) => w.type === 'account') ? 'active' : ''}"
				on:click={() => openBottomWindow('account')}
			>
				Account
			</button> -->

		</div>

		<div class="bottom-bar-right">
			<!-- Calendar button for timestamp selection -->
			<button
				class="toggle-button calendar-button"
				on:click={handleCalendar}
				title="Go to Date"
			>
				<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M19 3H18V1H16V3H8V1H6V3H5C3.89 3 3 3.9 3 5V19C3 20.1 3.89 21 5 21H19C20.11 21 21 20.1 21 19V5C21 3.9 20.11 3 19 3ZM19 19H5V8H19V19ZM7 10H12V15H7V10Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</button>

			<!-- Combined replay button -->
			<!---- <button
				class="toggle-button replay-button { !$streamInfo.replayActive || $streamInfo.replayPaused ? 'play' : 'pause' }"
				on:click={() => {
					if (!$streamInfo.replayActive) {
						handlePlay();
					} else if ($streamInfo.replayPaused) {
						handlePlay();
					} else {
						handlePause();
					}
				}}
				title={$streamInfo.replayActive && !$streamInfo.replayPaused ? 'Pause Replay' : 'Start/Resume Replay'}
			>
				{#if !$streamInfo.replayActive}
					<svg viewBox="0 0 24 24"><path d="M8,5.14V19.14L19,12.14L8,5.14Z" /></svg> <span>Replay</span>
				{:else if $streamInfo.replayPaused}
					<svg viewBox="0 0 24 24"><path d="M8,5.14V19.14L19,12.14L8,5.14Z" /></svg> <span>Play</span>
				{:else}
					<svg viewBox="0 0 24 24"><path d="M14,19H18V5H14M6,19H10V5H6V19Z" /></svg> <span>Pause</span>
				{/if}
			</button>

			{#if $streamInfo.replayActive}
				<button class="toggle-button replay-button stop" on:click={handleStop} title="Stop Replay">
					<svg viewBox="0 0 24 24"><path d="M18,18H6V6H18V18Z" /></svg> 
				</button>
				<button class="toggle-button replay-button reset" on:click={handleReset} title="Reset Replay">
					<svg viewBox="0 0 24 24"><path d="M12,5V1L7,6L12,11V8C15.31,8 18,10.69 18,14C18,17.31 15.31,20 12,20C8.69,20 6,17.31 6,14H4C4,18.42 7.58,22 12,22C16.42,22 20,18.42 20,14C20,9.58 16.42,6 12,6V5Z" /></svg> 
				</button>
				<button class="toggle-button replay-button next-day" on:click={handleNextDay} title="Next Day">
					<svg viewBox="0 0 24 24"><path d="M14,19.14V4.86L11,7.86L9.59,6.45L15.14,0.89L20.7,6.45L19.29,7.86L16,4.86V19.14H14M5,19.14V4.86H3V19.14H5Z" /></svg>
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

			<span class="value">
				{#if $streamInfo.timestamp !== undefined}
					{formatTimestamp($streamInfo.timestamp)}
				{:else}
					Loading Time...
				{/if}
			</span>
			-->
			<button class="profile-button" on:click={toggleSettings} aria-label="Toggle Settings">
				<!-- Add key to force re-render when the profile changes -->
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
					<Settings />
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.page {
		width: 100vw;
		height: 100vh;
		position: relative;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		background-color: var(--c1);
		margin: 0;
		padding: 0;
	}

	.content-wrapper {
		flex: 1;
		display: flex;
		height: 100%;
		min-height: 0;
		position: relative;
		margin-right: 45px;
	}

	.main-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		position: relative;
		overflow: hidden;
		min-height: 0;
	}

	.bottom-bar {
		height: 40px;
		min-height: 40px;
		background-color: var(--c2);
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0 10px;
		gap: 10px;
		flex-shrink: 0;
		width: 100%;
		z-index: 3;
		border-top: 1px solid var(--c1);
	}

	.chart-wrapper {
		flex: 1;
		position: relative;
		overflow: hidden;
		min-height: 0;
	}

	.sidebar {
		height: 100%;
		background-color: var(--ui-bg-primary);
		display: flex;
		flex-direction: column;
		position: relative;
		flex-shrink: 0;
		border-left: 1px solid var(--ui-border);
		max-width: min(600px, calc(100vw - 45px)); /* 600px max sidebar with 45px button bar */
	}

	.sidebar-buttons {
		position: fixed;
		top: 0;
		right: 0;
		height: 100vh;
		width: 45px;
		display: flex;
		flex-direction: column;
		background-color: var(--c2);
		z-index: 2;
		flex-shrink: 0;
		border-right: 1px solid var(--ui-border);
		max-width: min(600px, calc(100vw - 45px)); /* 600px max with 45px button bar */
	}

	.resize-handle {
		width: 4px;
		height: 100%;
		cursor: ew-resize;
		background-color: var(--c4);
		flex-shrink: 0;
		transition: background-color 0.2s;
		position: absolute;
		left: -4px;
		top: 0;
		z-index: 100;
		overflow: hidden;
		font-size: 0;
		line-height: 0;
		text-indent: -9999px;
	}

	.resize-handle:hover {
		background-color: var(--c3);
	}

	.side-btn {
		flex: 0 0 45px;
	}

	.menu-icon {
		filter: brightness(0) invert(1);
		width: 24px;
		height: 24px;
		object-fit: contain;
	}

	.bottom-bar-left {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.bottom-bar-right {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-left: auto;
	}

	.bottom-bar .pfp {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		cursor: pointer;
		margin-left: 8px;
		background-color: var(--c3);
		border: 1px solid var(--c4);
		overflow: hidden;
		display: block;
	}

	.speed-label {
		display: flex;
		align-items: center;
		color: #fff;
		font-size: 0.9em;
	}

	.speed-input {
		width: 50px;
		margin-left: 5px;
		height: 24px;
		background: var(--c1);
		border: 1px solid var(--c3);
		color: #fff;
		border-radius: 3px;
		padding: 0 4px;
	}

	.close-btn {
		background: transparent;
		border: none;
		color: #fff;
		cursor: pointer;
	}

	.window-content {
		padding: 10px;
		background-color: var(--ui-bg-primary);
		height: calc(100% - 30px);
		overflow-y: auto;
		scrollbar-width: none;
		-ms-overflow-style: none;
	}
	.window-content::-webkit-scrollbar {
		display: none;
	}

	:global(body) {
		margin: 0;
		padding: 0;
		overflow: hidden;
	}

	:global(*) {
		box-sizing: border-box;
	}

	.bottom-windows-container {
		position: relative;
		height: var(--bottom-height);
		background: var(--c1);
		border-top: 1px solid var(--c4);
		overflow: hidden;
		display: flex;
		border-top: none;
		max-width: 100%; /* Ensure container doesn't overflow */
	}

	.bottom-window {
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: column;
		background: var(--ui-bg-primary);
		min-width: 0; /* Allow window to shrink */
	}

	.window-content {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden; /* Prevent horizontal overflow */
		padding: 8px;
		scrollbar-width: none;
		height: 100%;
		background: var(--ui-bg-primary);
		min-width: 0; /* Allow content to shrink */
	}

	.bottom-resize-handle {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 4px;
		background: var(--c4);
		cursor: ns-resize;
		z-index: 100;
		transition: background-color 0.2s;
	}

	.bottom-resize-handle:hover {
		background: var(--c3);
	}

	.sidebar-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		height: 100%;
	}

	.main-sidebar-content {
		flex: 1;
		overflow-y: auto;
		scrollbar-width: none;
		min-height: 0;
	}

	.ticker-info-container {
		flex-shrink: 0;
		border-top: 1px solid var(--c3);
		background: var(--c2);
		height: var(--ticker-height);
		overflow-y: auto;
		overflow-x: hidden;
	}

	.main-sidebar-content::-webkit-scrollbar,
	.sidebar-content::-webkit-scrollbar {
		display: none;
	}

	.settings-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background-color: rgba(0, 0, 0, 0.7);
		display: flex;
		justify-content: center;
		align-items: center;
		z-index: 1000;
	}

	.settings-modal {
		width: 85%;
		height: 90%;
		min-width: 800px;
		min-height: 600px;
		background-color: var(--c1);
		border-radius: 8px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.settings-header {
		background-color: var(--c2);
		padding: 16px 24px;
		display: flex;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid var(--c3);
	}

	.settings-header h2 {
		margin: 0;
		color: var(--f1);
		font-size: 1.25rem;
		font-weight: 600;
	}

	.settings-header .close-btn {
		background: none;
		border: none;
		color: var(--f1);
		font-size: 1.75rem;
		cursor: pointer;
		padding: 0 8px;
		line-height: 1;
	}

	.settings-header .close-btn:hover {
		color: var(--f2);
	}

	.settings-content {
		flex: 1;
		overflow: hidden;
	}

	/* Prevent text-selection while dragging */
	.bottom-bar,
	.bottom-bar button,
	.side-btn,
	.menu-icon,
	.pfp,
	.close-btn,
	.speed-label {
		-webkit-user-select: none;
		-moz-user-select: none;
		-ms-user-select: none;
		user-select: none;
	}

	.sidebar-resize-handle {
		height: 4px;
		background: var(--c4);
		cursor: ns-resize;
		margin: -2px 0;
		z-index: 10;
		position: relative;
		transition: background-color 0.2s;
	}

	.sidebar-resize-handle:hover {
		background: var(--c3);
	}

	.profile-button {
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* Add styles for left sidebar */
	.left-sidebar {
		height: 100%;
		background-color: var(--ui-bg-primary);
		display: flex;
		flex-direction: column;
		position: relative;
		flex-shrink: 0;
		border-right: 1px solid var(--ui-border);
		max-width: min(800px, calc(100vw - 60px));
	}

	/* Add right position for left sidebar handle */
	.resize-handle.right {
		left: auto;
		right: -4px;
	}

	/* Query feature button styling */
	.query-feature {
		/* Remove special styling - use default button styles */
	}

	.chat-icon {
		width: 20px;
		height: 20px;
		color: var(--f1);
	}

	/* Calendar and Replay button styles */
	.calendar-button,
	.replay-button {
		padding: 0.3rem 0.8rem; /* Adjust padding slightly for text */
		min-width: auto; /* Remove fixed min-width */
		min-height: 32px;
		display: inline-flex; /* Use inline-flex */
		justify-content: center;
		align-items: center;
		gap: 0.4rem; /* Add gap between icon and text */
	}

	.calendar-button svg,
	.replay-button svg {
		width: 16px; /* Adjust icon size */
		height: 16px;
		fill: currentColor; /* Use button text color for icon */
	}

	.replay-button.play {
		/* Optional: Style play button differently, maybe green? */
		/* background-color: var(--success-color-faded); */
		/* border-color: var(--success-color); */
		/* color: var(--success-color); */
	}

	.replay-button.pause {
		/* Optional: Style pause button differently, maybe orange? */
		/* background-color: var(--warning-color-faded); */
		/* border-color: var(--warning-color); */
		/* color: var(--warning-color); */
	}

	.replay-button.stop {
		/* Optional: Style stop button differently, maybe red? */
		/* background-color: var(--error-color-faded); */
		/* border-color: var(--error-color); */
		/* color: var(--error-color); */
	}

	.calendar-button:hover,
	.replay-button:hover {
		background-color: var(--ui-bg-hover);
		border-color: var(--ui-border-hover);
	}

	.calendar-button:active,
	.replay-button:active {
		background-color: var(--ui-bg-active);
		transform: translateY(1px);
	}
</style>
