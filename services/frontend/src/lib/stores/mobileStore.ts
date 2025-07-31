import { writable, derived } from 'svelte/store';
import { browser } from '$app/environment';

// Types
export type MobileTab = 'agent' | 'chart' | 'sidebar';

// Constants
const MOBILE_BANNER_STORAGE_KEY = 'atlantis-mobile-banner-dismissed';

// Stores
export const activeMobileTab = writable<MobileTab>('agent');
export const showMobileBanner = writable(false);

// Initialize mobile banner based on dismissal state
export function initializeMobileBanner() {
    if (browser) {
        const dismissed = localStorage.getItem(MOBILE_BANNER_STORAGE_KEY);
        showMobileBanner.set(!dismissed);
    }
}

// Actions
export function dismissMobileBanner() {
    showMobileBanner.set(false);
    if (browser) {
        localStorage.setItem(MOBILE_BANNER_STORAGE_KEY, 'true');
    }
}

export function switchMobileTab(tab: MobileTab) {
    activeMobileTab.set(tab);
}

// Derived stores if needed
export const isMobileChartActive = derived(
    activeMobileTab,
    $tab => $tab === 'chart'
);

export const isMobileSidebarActive = derived(
    activeMobileTab,
    $tab => $tab === 'sidebar'
); 