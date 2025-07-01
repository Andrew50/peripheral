import { writable } from 'svelte/store';

interface MobileBannerState {
	visible: boolean;
}

export const mobileBannerStore = writable<MobileBannerState>({
	visible: false
});

// Helper functions to control the banner
export function showMobileBanner() {
	mobileBannerStore.set({ visible: true });
}

export function hideMobileBanner() {
	mobileBannerStore.set({ visible: false });
}
