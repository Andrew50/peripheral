import { browser } from '$app/environment';
import { goto } from '$app/navigation';

// ============================================================================
// AUTH TYPES
// ============================================================================

export interface User {
	authToken: string;
	profilePic: string;
}

export interface AuthLayoutData {
	isAuthenticated: boolean;
	isPublicViewing: boolean;
	sharedConversationId: string | null;
	user: User | null;
}

export interface LoginResponse {
	token: string;
	profilePic: string;
}

export interface GoogleCallbackResponse {
	token: string;
	profilePic: string;
}

// ============================================================================
// AUTH UTILITIES
// ============================================================================

/**
 * Centralized logout function that clears all auth data
 * and redirects to the specified path
 */
export function logout(redirectPath: string = '/login') {
	if (!browser) return;

	// Clear all session data
	sessionStorage.removeItem('authToken');
	sessionStorage.removeItem('profilePic');

	// Redirect to specified path
	window.location.href = "/";
}

/**
 * Set auth cookies with consistent options
 */
export function setAuthCookies(token: string, profilePic: string) {
	const domain = window.location.hostname;
	const isSecure = window.location.protocol === 'https:';
	const sameSite = 'lax';
	const cookieOptions = `path=/; domain=${domain}; max-age=21600; samesite=${sameSite}${isSecure ? '; secure' : ''}`;

	document.cookie = `authToken=${encodeURIComponent(token)}; ${cookieOptions}`;
	document.cookie = `profilePic=${encodeURIComponent(profilePic || '')}; ${cookieOptions}`;
}

/**
 * Set sessionStorage with auth data
 */
export function setAuthSessionStorage(token: string, profilePic: string) {
	if (typeof sessionStorage !== 'undefined') {
		sessionStorage.setItem('authToken', token);
		sessionStorage.setItem('profilePic', profilePic || '');
	}
}

/**
 * Get a specific cookie value
 */
export function getCookie(name: string): string | null {
	if (!browser) return null;

	const value = `; ${document.cookie}`;
	const parts = value.split(`; ${name}=`);
	if (parts.length === 2) {
		const cookieValue = parts.pop()?.split(';').shift();
		return cookieValue ? decodeURIComponent(cookieValue) : null;
	}
	return null;
}

/**
 * Check if user is authenticated by checking both sessionStorage and cookies
 * Uses sessionStorage first (faster), falls back to cookies (for initial loads)
 */
export function getAuthState(): boolean {
	if (!browser) return false;

	const sessionToken = sessionStorage.getItem('authToken');
	const cookieToken = getCookie('authToken');

	return !!(sessionToken || cookieToken);
}

export function clearAuth() {
	if (typeof sessionStorage !== 'undefined') {
		sessionStorage.removeItem('authToken');
		sessionStorage.removeItem('profilePic');
	}
}
