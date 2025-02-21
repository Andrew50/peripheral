<script lang="ts">
	import '$lib/core/global.css';
	import ChartContainer from '$lib/features/chart/chartContainer.svelte';
	import Alerts from '$lib/features/alerts/alert.svelte';
	import RightClick from '$lib/utils/popups/rightClick.svelte';
	import Setup from '$lib/utils/popups/setup.svelte';
	import Input from '$lib/utils/popups/input.svelte';
	import Similar from '$lib/features/similar/similar.svelte';
	import Study from '$lib/features/study.svelte';
	import Journal from '$lib/features/journal.svelte';
	import Watchlist from '$lib/features/watchlist.svelte';
	//import TickerInfo from '$lib/features/quotes/tickerInfo.svelte';
	import Quote from '$lib/features/quotes/quote.svelte';
	import Algo from '$lib/utils/popups/algo.svelte';
	import { activeMenu, changeMenu } from '$lib/core/stores';

	// Windows that will be opened in draggable divs
	import Screener from '$lib/features/screen.svelte';
	import Account from '$lib/features/account.svelte';
	import Active from '$lib/features/active.svelte';
	import Setups from '$lib/features/setups/setups.svelte';
	import Options from '$lib/features/options.svelte';
	import Settings from '$lib/features/settings.svelte';

	// Replay logic
	import {
		startReplay,
		stopReplay,
		pauseReplay,
		resumeReplay,
		changeSpeed,
		nextDay
	} from '$lib/utils/stream/interface';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { browser } from '$app/environment';
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { goto } from '$app/navigation';
	import {
		initStores,
		streamInfo,
		formatTimestamp,
		dispatchMenuChange,
		menuWidth
	} from '$lib/core/stores';
	import { writable, type Writable } from 'svelte/store';

	// Add import near the top with other imports
	import Screensaver from '$lib/features/screensaver.svelte';

	type Menu = 'none' | 'watchlist' | 'alerts' | 'study' | 'journal' | 'similar';

	// Initialize all sidebar state variables as closed
	let lastSidebarMenu: Menu | null = null;
	let sidebarWidth = 0;
	const sidebarMenus: Menu[] = ['watchlist', 'alerts', 'study', 'journal', 'similar'];

	// Initialize chartWidth with a default value
	let chartWidth = 0;

	// Bottom windows
	type BottomWindowType = 'screener' | 'account' | 'active' | 'options' | 'setups' | 'settings';
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
	const MIN_BOTTOM_HEIGHT = 250;
	const MAX_BOTTOM_HEIGHT = 500;

	// Add these state variables near the top with other state declarations
	let lastBottomWindow: BottomWindow | null = null;

	let profilePic = '';
	let username = '';

	let sidebarResizing = false;
	let startY = 0;
	let tickerHeight = 300; // Initial height
	const MIN_TICKER_HEIGHT = 100;
	const MAX_TICKER_HEIGHT = 600;

	// Add state variables after other state declarations
	let screensaverActive = false;
	let inactivityTimer: ReturnType<typeof setTimeout> | null = null;
	const INACTIVITY_TIMEOUT = 5 * 60 * 1000; // 5 minutes in milliseconds

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
			console.log('menuWidth', $menuWidth);
			chartWidth = window.innerWidth - $menuWidth - 60; // 60px for sidebar buttons
		}
	}

	onMount(() => {
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
		}
		privateRequest<string>('verifyAuth', {}).catch(() => {
			goto('/login');
		});
		initStores();

		dispatchMenuChange.subscribe((menuName: string) => {
			toggleMenu(menuName as Menu);
		});

		profilePic = sessionStorage.getItem('profilePic') || '';
		username = sessionStorage.getItem('username') || '';

		// Setup activity listeners
		if (browser) {
			const activityEvents = ['mousedown', 'mousemove', 'keypress', 'scroll', 'touchstart'];
			activityEvents.forEach((event) => {
				document.addEventListener(event, resetInactivityTimer);
			});
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
		const activityEvents = ['mousedown', 'mousemove', 'keypress', 'scroll', 'touchstart'];
		if (browser && document) {
			window.removeEventListener('resize', updateChartWidth);
			stopSidebarResize();
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
			menuWidth.set(300); // Or whatever your default width is
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
	let minWidth = 200;
	let maxWidth = 600;

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
		let newWidth = window.innerWidth - clientX - 60; // 60px is the width of sidebar buttons

		if (newWidth > maxWidth) {
			newWidth = maxWidth;
		}

		// Store state before closing
		if (newWidth < minWidth && lastSidebarMenu !== null) {
			lastSidebarMenu = null;
			menuWidth.set(0);
		}
		// Restore state if dragging back
		else if (newWidth >= minWidth && lastSidebarMenu) {
			lastSidebarMenu = lastSidebarMenu;
			menuWidth.set(newWidth);
			lastSidebarMenu = null;
		}
		// Normal resize
		else if (newWidth >= minWidth) {
			menuWidth.set(newWidth);
		}

		updateChartWidth();
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
		// Replace current if a different window is clicked
		bottomWindowsHeight = 200; // default
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
		const containerBottom = window.innerHeight - 40; // 40px is bottom-bar height
		const newHeight = containerBottom - event.clientY;

		if (newHeight < MIN_BOTTOM_HEIGHT && bottomWindows.length > 0) {
			// Store state before closing
			lastBottomWindow = bottomWindows[0];
			bottomWindowsHeight = 0;
			bottomWindows = [];
		}
		// Restore state if dragging back
		else if (newHeight >= MIN_BOTTOM_HEIGHT && lastBottomWindow) {
			bottomWindowsHeight = newHeight;
			bottomWindows = [lastBottomWindow];
			lastBottomWindow = null;
		}
		// Normal resize
		else if (newHeight >= MIN_BOTTOM_HEIGHT && newHeight <= MAX_BOTTOM_HEIGHT) {
			bottomWindowsHeight = newHeight;
		} else if (newHeight > MAX_BOTTOM_HEIGHT) {
			bottomWindowsHeight = MAX_BOTTOM_HEIGHT;
		}

		updateChartWidth();
	}

	function stopBottomResize() {
		bottomResizing = false;
		document.removeEventListener('mousemove', handleBottomResize);
		document.removeEventListener('mouseup', stopBottomResize);
		document.body.style.cursor = 'default';
	}

	function getProfileDisplay() {
		if (profilePic) {
			return profilePic;
		}
		// Generate initial avatar if no profile pic
		if (username) {
			return `data:image/svg+xml,${encodeURIComponent(`<svg width="28" height="28" xmlns="http://www.w3.org/2000/svg"><circle cx="14" cy="14" r="14" fill="#1a1c21"/><text x="14" y="19" font-family="Arial" font-size="14" fill="#e0e0e0" text-anchor="middle" font-weight="bold">${username.charAt(0).toUpperCase()}</text></svg>`)}`;
		}
		// Fallback if no username (shouldn't happen)
		return `data:image/svg+xml,${encodeURIComponent(`<svg width="28" height="28" xmlns="http://www.w3.org/2000/svg"><circle cx="14" cy="14" r="14" fill="#1a1c21"/><text x="14" y="19" font-family="Arial" font-size="14" fill="#e0e0e0" text-anchor="middle" font-weight="bold">?</text></svg>`)}`;
	}

	function startSidebarResize(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		sidebarResizing = true;
		if (event instanceof MouseEvent) {
			startY = event.clientY;
		} else {
			startY = event.touches[0].clientY;
		}
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

		const deltaY = startY - currentY;
		startY = currentY;

		tickerHeight = Math.min(Math.max(tickerHeight + deltaY, MIN_TICKER_HEIGHT), MAX_TICKER_HEIGHT);
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
				screensaverActive = true;
			}, INACTIVITY_TIMEOUT);
		}
	}

	function toggleScreensaver() {
		screensaverActive = !screensaverActive;
	}
</script>

<div
	class="page"
	role="application"
	on:keydown={(e) => {
		if (e.key === 'Escape') {
			minimizeBottomWindow();
		}
	}}
>
	<!-- Global Popups -->
	<Input />
	<RightClick />
	<Setup />
	<Algo />
	<!-- Main area wrapper -->
	<div class="app-container">
		<div class="content-wrapper">
			<!-- Main content area -->
			<div class="main-content">
				<!-- Chart area -->
				<div class="chart-wrapper">
					<ChartContainer width={chartWidth} />
				</div>

				<!-- Bottom windows container -->
				<div class="bottom-windows-container" style="--bottom-height: {bottomWindowsHeight}px">
					{#each bottomWindows as w}
						<div class="bottom-window">
							<div class="window-content">
								{#if w.type === 'screener'}
									<Screener />
								{:else if w.type === 'active'}
									<Active />
								{:else if w.type === 'options'}
									<Options />
								{:else if w.type === 'setups'}
									<Setups />
								{:else if w.type === 'account'}
									<Account />
								{:else if w.type === 'settings'}
									<Settings />
								{/if}
							</div>
						</div>
					{/each}
					{#if bottomWindows.length > 0}
						<div class="bottom-resize-handle" on:mousedown={startBottomResize}></div>
					{/if}
				</div>
			</div>

			<!-- Sidebar -->
			{#if $menuWidth > 0}
				<div class="sidebar" style="width: {$menuWidth}px;">
					<div class="resize-handle" on:mousedown={startResize} on:touchstart={startResize} />
					<div class="sidebar-content">
						<!-- Main sidebar content -->
						<div class="main-sidebar-content">
							{#if $activeMenu === 'watchlist'}
								<Watchlist />
							{:else if $activeMenu === 'alerts'}
								<Alerts />
							{:else if $activeMenu === 'study'}
								<Study />
							{:else if $activeMenu === 'journal'}
								<Journal />
							{:else if $activeMenu === 'similar'}
								<Similar />
							{/if}
						</div>

						<div
							class="sidebar-resize-handle"
							on:mousedown={startSidebarResize}
							on:touchstart|preventDefault={startSidebarResize}
						></div>

						<div class="ticker-info-container">
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
				class="toggle-button {bottomWindows.some((w) => w.type === 'screener') ? 'active' : ''}"
				on:click={() => openBottomWindow('screener')}
			>
				Screener
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'active') ? 'active' : ''}"
				on:click={() => openBottomWindow('active')}
			>
				Active
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'options') ? 'active' : ''}"
				on:click={() => openBottomWindow('options')}
			>
				Options
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'setups') ? 'active' : ''}"
				on:click={() => openBottomWindow('setups')}
			>
				Setups
			</button>
			<button
				class="toggle-button {bottomWindows.some((w) => w.type === 'account') ? 'active' : ''}"
				on:click={() => openBottomWindow('account')}
			>
				Account
			</button>
		</div>

		<div class="bottom-bar-right">
			<!-- Combined replay button -->
			<button
				on:click={() => {
					if (!$streamInfo.replayActive) {
						handlePlay();
					} else if ($streamInfo.replayPaused) {
						handlePlay();
					} else {
						handlePause();
					}
				}}
			>
				{#if !$streamInfo.replayActive}
					Replay
				{:else if $streamInfo.replayPaused}
					Play
				{:else}
					Pause
				{/if}
			</button>

			{#if $streamInfo.replayActive}
				<button on:click={handleStop}>Stop</button>
				<button on:click={handleReset}>Reset</button>
				<button on:click={handleNextDay}>Next Day</button>

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

			<!-- Current timestamp -->
			<span class="value">
				{#if $streamInfo.timestamp !== undefined}
					{formatTimestamp($streamInfo.timestamp)}
				{:else}
					Loading Time...
				{/if}
			</span>

			<button
				class="toggle-button {screensaverActive ? 'active' : ''}"
				on:click={toggleScreensaver}
				title="Screensaver"
			>
				<i class="fas fa-tv"></i>
			</button>

			<img src={getProfileDisplay()} alt="Profile" class="pfp" on:click={toggleSettings} />
		</div>
	</div>

	{#if showSettingsPopup}
		<div class="settings-overlay" on:click|self={toggleSettings}>
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

	{#if screensaverActive}
		<button
			class="screensaver-overlay"
			on:click={() => (screensaverActive = false)}
			on:keydown={(e) => {
				if (e.key === 'Enter' || e.key === 'Space') {
					screensaverActive = false;
				}
			}}
		>
			<Screensaver />
		</button>
	{/if}
</div>

<!--/+page.svelte-->

<!--/+page.svelte-->

<!--/+page.svelte-->

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
		margin-right: 60px;
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
		max-width: calc(100vw - 60px);
	}

	.sidebar-buttons {
		position: fixed;
		top: 0;
		right: 0;
		height: 100vh;
		width: 60px;
		display: flex;
		flex-direction: column;
		background-color: var(--c2);
		z-index: 2;
		flex-shrink: 0;
		border-left: 1px solid var(--c4);
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
	}

	.resize-handle:hover {
		background-color: var(--c3);
	}

	.side-btn {
		flex: 0 0 60px;
	}

	.menu-icon {
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

	.draggable-window {
		position: fixed;
		border: 1px solid var(--c2);
		background-color: var(--c1);
		z-index: 999;
		min-width: 200px;
		min-height: 100px;
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
		overflow: hidden;
	}

	.window-header {
		background-color: var(--c2);
		color: #fff;
		padding: 5px;
		display: flex;
		justify-content: space-between;
		cursor: move;
	}

	.window-title {
		font-weight: bold;
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

	.bottom-window {
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: column;
		background: var(--ui-bg-primary);
	}

	.window-content {
		flex: 1;
		overflow-y: auto;
		padding: 8px;
		scrollbar-width: none;
		height: 100%;
		background: var(--ui-bg-primary);
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
		overflow: hidden;
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
		width: 50%;
		height: 50%;
		background-color: var(--c1);
		border-radius: 8px;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.settings-header {
		background-color: var(--c2);
		padding: 12px 16px;
		display: flex;
		justify-content: space-between;
		align-items: center;
		border-bottom: 1px solid var(--c3);
	}

	.settings-header h2 {
		margin: 0;
		color: var(--f1);
		font-size: 1.2em;
	}

	.settings-header .close-btn {
		background: none;
		border: none;
		color: var(--f1);
		font-size: 1.5em;
		cursor: pointer;
		padding: 0 4px;
		line-height: 1;
	}

	.settings-header .close-btn:hover {
		color: var(--f2);
	}

	.settings-content {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
	}

	/* Prevent text-selection while dragging */
	.bottom-bar,
	.bottom-bar button,
	.side-btn,
	.menu-icon,
	.timestamp,
	.pfp,
	.window-header,
	.window-title,
	.close-btn,
	.minimize-btn,
	.speed-label {
		-webkit-user-select: none;
		-moz-user-select: none;
		-ms-user-select: none;
		user-select: none;
	}

	.bottom-windows-container {
		position: relative;
		height: var(--bottom-height);
		background: var(--c1);
		border-top: 1px solid var(--c4);
		overflow: hidden;
		display: flex;
		border-top: none;
	}

	/* Only show border when windows are open */
	.bottom-windows-container:not(:empty) {
		border-top: 1px solid var(--c4);
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

	.screensaver-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background-color: var(--c1);
		z-index: 1000;
		cursor: pointer;
		border: none;
		padding: 0;
		width: 100%;
		height: 100%;
	}
</style>
