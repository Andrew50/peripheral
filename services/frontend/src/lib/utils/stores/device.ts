import { writable } from 'svelte/store';
import { browser } from '$app/environment';

// Store to track if the current device is a mobile device (phone)
export const isMobileDevice = writable(false);

// Debug override flag - can be set via URL param or localStorage
let debugOverride: boolean | null = null;

// Initialize device detection
if (browser) {

    // Try modern userAgentData API first (Chromium browsers)
    let isMobile = false;

    if ('userAgentData' in navigator && (navigator as any).userAgentData?.mobile !== undefined) {
        isMobile = (navigator as any).userAgentData.mobile;
        console.log('🔍 [device detection] Using userAgentData.mobile:', isMobile);
    } else {
        // Fallback to user agent string detection for phones only
        // Excludes tablets to ensure they get desktop UI
        const mobilePhonePattern = /Android.*Mobile|iPhone|iPod|BlackBerry|IEMobile|Opera Mini/i;
        isMobile = mobilePhonePattern.test(navigator.userAgent);
        console.log('🔍 [device detection] Using UA pattern detection:', isMobile, navigator.userAgent);
    }

    // Additional width-based fallback only for very small screens
    if (!isMobile && window.innerWidth <= 480) {
        isMobile = true;
        console.log('🔍 [device detection] Width-based fallback triggered for very small screen');
    }

    isMobileDevice.set(isMobile);
}

// Export function to manually override detection (for testing)
export function setMobileDeviceOverride(isMobile: boolean | null) {
    if (browser) {
        if (isMobile === null) {
            localStorage.removeItem('forceMobile');
            // Re-run detection
            window.location.reload();
        } else {
            localStorage.setItem('forceMobile', isMobile.toString());
            isMobileDevice.set(isMobile);
        }
    }
} 