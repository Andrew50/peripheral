// Frontend Plan Features Configur

export interface PlanFeatures {
	[planName: string]: string[];
}

// Define all available features for easy maintenance
export const AVAILABLE_FEATURES = {
	// Core features
    DELAYED_DATA: "Delayed data",
	REALTIME_DATA: "Realtime data",
	PROMPTS_5: "5 prompts",
	// Prompt/Credit limits
	PROMPTS_250: "250 prompts/mo",
	PROMPTS_1000: "1000 prompts/mo",
	
	// Strategy features
	STRATEGIES_5: "5 active strategies",
	STRATEGIES_20: "20 active strategies",
	SINGLE_STRATEGY_SCREENING: "Single strategy screening",
	MULTI_STRATEGY_SCREENING: "Multi strategy screening",
	
	// Alert features
	ALERTS_100: "100 news or price alerts",
	ALERTS_400: "400 alerts",
	WATCHLIST_ALERTS: "Watchlist alerts",
	
	// Chart features
	MULTI_CHART: "Multi chart layouts",
	

} as const;

// Plan feature definitions
export const PLAN_FEATURES: PlanFeatures = {
	// Free plan
	"Free": [
		AVAILABLE_FEATURES.DELAYED_DATA,
        AVAILABLE_FEATURES.PROMPTS_5,
	],
	
	// Plus Monthly plan
	"Plus": [
		AVAILABLE_FEATURES.REALTIME_DATA,
		AVAILABLE_FEATURES.PROMPTS_250,
		AVAILABLE_FEATURES.STRATEGIES_5,
		AVAILABLE_FEATURES.SINGLE_STRATEGY_SCREENING,
		AVAILABLE_FEATURES.ALERTS_100,
	],
	
	// Plus Yearly plan
	"Plus Yearly": [
		AVAILABLE_FEATURES.REALTIME_DATA,
		AVAILABLE_FEATURES.PROMPTS_250,
		AVAILABLE_FEATURES.STRATEGIES_5,
		AVAILABLE_FEATURES.SINGLE_STRATEGY_SCREENING,
		AVAILABLE_FEATURES.ALERTS_100,
	],
	
	// Pro Monthly plan
	"Pro": [
		AVAILABLE_FEATURES.REALTIME_DATA,
		AVAILABLE_FEATURES.PROMPTS_1000,
		AVAILABLE_FEATURES.STRATEGIES_20,
		AVAILABLE_FEATURES.MULTI_STRATEGY_SCREENING,
		AVAILABLE_FEATURES.ALERTS_400,
		AVAILABLE_FEATURES.WATCHLIST_ALERTS,
		AVAILABLE_FEATURES.MULTI_CHART,
	],
	
	// Pro Yearly plan
	"Pro Yearly": [
		AVAILABLE_FEATURES.REALTIME_DATA,
		AVAILABLE_FEATURES.PROMPTS_1000,
		AVAILABLE_FEATURES.STRATEGIES_20,
		AVAILABLE_FEATURES.MULTI_STRATEGY_SCREENING,
		AVAILABLE_FEATURES.ALERTS_400,
		AVAILABLE_FEATURES.WATCHLIST_ALERTS,
		AVAILABLE_FEATURES.MULTI_CHART
	]
};

// Utility functions for feature management
export function getPlanFeatures(planName: string): string[] {
	return PLAN_FEATURES[planName] || [];
}