import { writable } from 'svelte/store';

export interface AuthModalState {
	visible: boolean;
	mode: 'login' | 'signup';
	requiredFeature: string;
}

const defaultState: AuthModalState = {
	visible: false,
	mode: 'login',
	requiredFeature: 'this feature'
};

export const authModalStore = writable<AuthModalState>(defaultState);

// Helper functions to trigger the modal
export function showAuthModal(requiredFeature: string, mode: 'login' | 'signup' = 'login') {
	authModalStore.set({
		visible: true,
		mode,
		requiredFeature
	});
}

export function hideAuthModal() {
	authModalStore.update(state => ({
		...state,
		visible: false
	}));
}

// Function to check if user is authenticated
export function requireAuth(featureName: string): boolean {
	const token = typeof window !== 'undefined' ? sessionStorage.getItem('authToken') : null;
	
	if (!token) {
		showAuthModal(featureName);
		return false;
	}
	
	return true;
} 