import { preloadPricingConfiguration } from '$lib/config/pricing';

// Global pricing loader that can be called early in the app lifecycle
let pricingPreloaded = false;

export async function initializePricing(): Promise<void> {
    if (pricingPreloaded) return;

    try {
        await preloadPricingConfiguration();
        pricingPreloaded = true;
        console.log('✅ Pricing configuration preloaded successfully');
    } catch (error) {
        console.warn('⚠️ Failed to preload pricing configuration:', error);
        // Don't throw - this is a background operation
    }
}

// Call this function early in the app lifecycle (e.g., in app.html or main layout)
export function startPricingPreload(): void {
    // Start preloading in the background without blocking
    initializePricing().catch(error => {
        console.warn('Background pricing preload failed:', error);
    });
} 