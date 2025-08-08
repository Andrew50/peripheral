<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import Query from '$lib/features/chat/chat.svelte';
	import ChartContainer from '$lib/features/chart/chartContainer.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import Watchlist from '$lib/features/watchlist/watchlist.svelte';
	import WatchlistTabs from '$lib/features/watchlist/watchlistTabs.svelte';
	import Alerts from '$lib/features/alerts/alert.svelte';
	import Quote from '$lib/features/quotes/quote.svelte';
	import MobileBanner from '$lib/components/mobileBanner.svelte';
	import { activeMenu, changeMenu, activeChartInstance } from '$lib/utils/stores/stores';
	import {
		activeMobileTab,
		showMobileBanner as showMobileBannerStore,
		initializeMobileBanner,
		dismissMobileBanner,
		switchMobileTab,
		type MobileTab
	} from '$lib/stores/mobileStore';

	// Props
	export let data: any;
	export let sharedConversationId: string = '';
	export let isPublicViewing: boolean = false;

	// Alert tabs
	const alertTabs = ['price', 'strategy', 'logs'] as const;
	type AlertView = (typeof alertTabs)[number];
	let alertView: AlertView = 'price';

	// Component references
	let alertsComponent: any;

	// Initialize on mount
	onMount(() => {
		initializeMobileBanner();
	});

	// Mobile tab click handler with sidebar menu logic
	function handleSidebarTabClick() {
		switchMobileTab('sidebar');
		// Default to watchlist if no menu is active
		if (get(activeMenu) === 'none') {
			changeMenu('watchlist');
		}
	}

	// Alert creation methods
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
</script>

<!-- Mobile tabbed interface -->
<div class="mobile-container">
	<!-- Mobile banner at the top -->
	{#if $showMobileBannerStore}
		<MobileBanner on:dismiss={dismissMobileBanner} />
	{/if}

	<!-- Persistent chart container - always rendered -->
	<div
		class="persistent-chart-container"
		style="display: {$activeMobileTab === 'chart'
			? 'flex'
			: 'none'}; flex-direction: column; height: 100%; flex: 1;"
	>
		<TopBar instance={$activeChartInstance || {}} />
		<ChartContainer defaultChartData={data.defaultChartData} />
	</div>

	<!-- Mobile content area for non-chart tabs -->
	<div
		class="mobile-content"
		style="display: {$activeMobileTab !== 'chart' ? 'flex' : 'none'}; flex-direction: column;"
	>
		{#if $activeMobileTab === 'agent'}
			<Query {isPublicViewing} {sharedConversationId} />
		{:else if $activeMobileTab === 'sidebar'}
			<div class="mobile-sidebar-wrapper">
				<!-- Sidebar header with tabs -->
				<div class="mobile-sidebar-header">
					<!-- Mobile sidebar navigation -->
					<!-- <div class="mobile-sidebar-nav">
                        <button
                            class="mobile-sidebar-tab {$activeMenu === 'watchlist' ? 'active' : ''}"
                            on:click={() => changeMenu('watchlist')}
                        >
                            Watchlist
                        </button>
                        <  button
                            class="mobile-sidebar-tab {$activeMenu === 'alerts' ? 'active' : ''}"
                            on:click={() => changeMenu('alerts')}
                        >
                            Alerts
                        </button>
                    </div> -->

					{#if $activeMenu === 'alerts'}
						<!-- Alert Controls -->
						<div class="alert-tab-container mobile-alert-tabs">
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
						<div class="mobile-watchlist-tabs">
							<WatchlistTabs />
						</div>
					{/if}
				</div>

				<!-- Sidebar content -->
				<div class="mobile-sidebar-content">
					<div class="main-sidebar-content">
						{#if $activeMenu === 'watchlist'}
							<Watchlist showTabs={false} />
						{:else if $activeMenu === 'alerts'}
							<Alerts bind:this={alertsComponent} view={alertView} />
						{/if}
					</div>

					<!-- Quote section -->
					<div class="mobile-quote-container">
						<Quote />
					</div>
				</div>
			</div>
		{/if}
	</div>

	<!-- Mobile bottom navigation -->
	<div class="mobile-bottom-nav">
		<button
			class="mobile-nav-tab {$activeMobileTab === 'agent' ? 'active' : ''}"
			on:click={() => switchMobileTab('agent')}
		>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				fill="none"
				viewBox="0 0 24 24"
				stroke-width="1.5"
				stroke="currentColor"
				width="28"
				height="28"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="m21 7.5-2.25-1.313M21 7.5v2.25m0-2.25-2.25 1.313M3 7.5l2.25-1.313M3 7.5l2.25 1.313M3 7.5v2.25m9 3 2.25-1.313M12 12.75l-2.25-1.313M12 12.75V15m0 6.75 2.25-1.313M12 21.75V19.5m0 2.25-2.25-1.313m0-16.875L12 2.25l2.25 1.313M21 14.25v2.25l-2.25 1.313m-13.5 0L3 16.5v-2.25"
				/>
			</svg>
			<span>Agent</span>
		</button>
		<button
			class="mobile-nav-tab {$activeMobileTab === 'sidebar' ? 'active' : ''}"
			on:click={handleSidebarTabClick}
		>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				width="24"
				height="24"
				fill="currentColor"
				viewBox="0 0 16 16"
				class="watchlist-icon"
			>
				<path
					fill-rule="evenodd"
					d="M2.5 12a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5zm0-4a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5zm0-4a.5.5 0 0 1 .5-.5h10a.5.5 0 0 1 0 1H3a.5.5 0 0 1-.5-.5z"
				/>
			</svg>
			<span>Watchlist</span>
		</button>
		<button
			class="mobile-nav-tab {$activeMobileTab === 'chart' ? 'active' : ''}"
			on:click={() => switchMobileTab('chart')}
		>
			<svg
				xmlns="http://www.w3.org/2000/svg"
				viewBox="0 0 28 28"
				width="28"
				height="28"
				fill="currentColor"
				class="chart-icon"
			>
				<path
					d="M17 11v6h3v-6h-3zm-.5-1h4a.5.5 0 0 1 .5.5v7a.5.5 0 0 1-.5.5h-4a.5.5 0 0 1-.5-.5v-7a.5.5 0 0 1 .5-.5z"
				></path>
				<path d="M18 7h1v3.5h-1zm0 10.5h1V21h-1z"></path>
				<path
					d="M9 8v12h3V8H9zm-.5-1h4a.5.5 0 0 1 .5.5v13a.5.5 0 0 1-.5.5h-4a.5.5 0 0 1-.5-.5v-13a.5.5 0 0 1 .5-.5z"
				></path>
				<path d="M10 4h1v3.5h-1zm0 16.5h1V24h-1z"></path>
			</svg>
			<span>Chart</span>
		</button>
	</div>
</div>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Instrument+Sans:ital,wght@0,100..900;1,100..900&display=swap');

	/* Mobile container and navigation */
	.mobile-container {
		position: fixed;
		inset: 0; /* top:0; right:0; bottom:0; left:0; */
		z-index: 9999; /* above everything */
		background: #121212; /* match site background so it feels native */
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	.mobile-content {
		flex: 1;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	.mobile-bottom-nav {
		height: 60px;
		background: #121212;
		border-top: 4px solid var(--c1);
		display: flex;
		font-family: 'Instrument Sans', sans-serif;
		flex-shrink: 0;
	}

	.mobile-nav-tab {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: space-between;
		background: none;
		border: none;
		color: rgb(255 255 255 / 50%);
		cursor: pointer;
		padding: 4px;
		transition: all 0.2s ease;
		font-size: 14px;
		font-family: 'Instrument Sans', sans-serif;
		flex: 1;
		height: 100%;
	}

	.mobile-nav-tab.active {
		color: #fff;
	}

	.mobile-nav-tab svg {
		width: 24px;
		height: 24px;
	}

	.mobile-nav-tab .watchlist-icon {
		width: 24px;
		height: 24px;
	}

	.mobile-nav-tab .chart-icon {
		width: 24px;
		height: 24px;
	}

	/*
    .mobile-chart-wrapper {
        flex: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }
*/

	/* Mobile sidebar wrapper */
	.mobile-sidebar-wrapper {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.mobile-sidebar-header {
		min-height: 40px;
		background-color: #121212;
		display: flex;
		flex-direction: column;
		padding: 0 10px;
		flex-shrink: 0;
		border-bottom: 2px solid var(--c1);
		gap: 8px;
	}

	/*
    .mobile-sidebar-nav {
        display: flex;
        gap: 8px;
        padding: 8px 0;
    }
*/

	/*
    .mobile-sidebar-tab {
        background: rgba(255, 255, 255, 0.1);
        border: none;
        color: rgba(255, 255, 255, 0.7);
        padding: 8px 16px;
        border-radius: 6px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.2s ease;
    }
*/

	/*
    .mobile-sidebar-tab.active {
        background: rgba(255, 255, 255, 0.2);
        color: #ffffff;
        font-weight: 600;
    }
*/

	.mobile-alert-tabs {
		padding: 0 0 8px;
	}

	.mobile-watchlist-tabs {
		padding: 0 0 8px;
	}

	.mobile-sidebar-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.mobile-sidebar-content .main-sidebar-content {
		flex: 1;
		overflow: auto;
	}

	.mobile-quote-container {
		height: 200px;
		min-height: 200px;
		border-top: 2px solid var(--c1);
		overflow: auto;
	}

	/* Import watchlist tab styles from main page */
	.watchlist-tab {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgb(255 255 255 / 90%);
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
		text-shadow: 0 1px 2px rgb(0 0 0 / 60%);
	}

	.watchlist-tab:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgb(255 255 255 / 40%);
	}

	.watchlist-tab.active {
		background: rgb(255 255 255 / 20%);
		border-color: transparent;
		color: #fff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgb(255 255 255 / 20%);
	}

	.alert-tab-container {
		display: flex;
		align-items: center;
		flex-grow: 1;
		min-width: 0;
		gap: 0;
	}

	/* Persistent chart container */
	.persistent-chart-container {
		overflow: hidden;
	}
</style>
