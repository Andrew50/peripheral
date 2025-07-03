// Centralized pricing configuration
export const PRICING_CONFIG = {
    // Stripe Price IDs - Update these with your actual Stripe price IDs
    PRICE_IDS: {
        starter: 'price_starter_monthly', // Replace with actual Stripe price ID
        plus: 'price_plus_monthly',       // Replace with actual Stripe price ID
        pro: 'price_pro_monthly'          // Replace with actual Stripe price ID
    },

    // Plan configurations with pricing and features
    PLANS: {
        free: {
            name: 'Free',
            price: 0,
            period: '/month',
            description: 'Basic access to get started',
            features: [
                'Delayed charting',
                '5 queries',
                'Watchlists'
            ],
            cta: 'Current Plan',
            disabled: true
        },
        plus: {
            name: 'Plus',
            price: 99,
            period: '/month',
            description: 'Perfect for active traders',
            features: [
                'Realtime charting',
                '250 queries',
                '5 strategy alerts',
                'Single strategy screening',
                '100 news or price alerts'
            ],
            cta: 'Choose Plus',
            priceId: 'plus' // References PRICE_IDS.plus
        },
        pro: {
            name: 'Pro',
            price: 199,
            period: '/month',
            description: 'Advanced features for professional traders',
            features: [
                'Sub 1 minute charting',
                'Multi chart',
                '1000 queries',
                '20 strategy alerts',
                'Multi strategy screening',
                '400 alerts',
                'Watchlist alerts'
            ],
            cta: 'Choose Pro',
            priceId: 'pro', // References PRICE_IDS.pro
            popular: true
        }
    }
} as const;

// Helper function to get plan by key
export function getPlan(planKey: keyof typeof PRICING_CONFIG.PLANS) {
    return PRICING_CONFIG.PLANS[planKey];
}

// Helper function to get Stripe price ID
export function getStripePrice(planKey: 'starter' | 'plus' | 'pro') {
    return PRICING_CONFIG.PRICE_IDS[planKey];
}

// Helper function to format price
export function formatPrice(price: number): string {
    return `$${price}`;
} 