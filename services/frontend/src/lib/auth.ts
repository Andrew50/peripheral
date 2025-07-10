import { browser } from '$app/environment';
import { goto } from '$app/navigation';

// ============================================================================
// AUTH TYPES
// ============================================================================

export interface AuthUser {
	profilePic: string;
	username: string;
	authToken: string;
}

export interface AuthLayoutData {
	isPublicViewing: boolean;
	sharedConversationId: string | null;
	isAuthenticated: boolean;
	user: AuthUser | null;
}

export interface LoginResponse {
	token: string;
	profilePic: string;
	username: string;
}

export interface GoogleCallbackResponse {
	token: string;
	profilePic: string;
	username: string;
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
	sessionStorage.removeItem('username');
	sessionStorage.removeItem('profilePic');

	// Redirect to specified path
	window.location.href = "/";
}

/**
 * Set auth cookies with consistent options
 */
export function setAuthCookies(token: string, profilePic: string, username: string) {
	if (!browser) return;

	const cookieOptions = 'path=/; max-age=21600; SameSite=Lax'; // 6 hours

	document.cookie = `authToken=${token}; ${cookieOptions}`;
	document.cookie = `profilePic=${encodeURIComponent(profilePic || '')}; ${cookieOptions}`;
	document.cookie = `username=${encodeURIComponent(username || '')}; ${cookieOptions}`;
}

/**
 * Set sessionStorage with auth data
 */
export function setAuthSessionStorage(token: string, profilePic: string, username: string) {
	if (!browser) return;

	sessionStorage.setItem('authToken', token);
	sessionStorage.setItem('profilePic', profilePic || '');
	sessionStorage.setItem('username', username || '');
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
