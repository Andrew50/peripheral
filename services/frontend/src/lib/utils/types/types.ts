export interface Instance {
	ticker?: string;
	timestamp?: number;
	securityId?: string | number;
	timeframe?: string;
	extendedHours?: boolean;
	price?: number;
	name?: string;
	market?: string;
	locale?: string;
	primary_exchange?: string;
	active?: boolean;
	market_cap?: number;
	dollar_volume?: number;
	description?: string;
	logo?: string;
	icon?: string;
	share_class_shares_outstanding?: number;
	industry?: string;
	sector?: string;
	totalShares?: number;
	sortOrder?: number;
	flagged?: boolean;
}

// Defines the structure for the criteria JSON object within a Strategy
export interface StrategyCriteria {
	timeframe: string;
	bars: number;
	threshold: number;
	dolvol: number;
	adr: number;
	mcap: number;
	// Note: score and activeScreen might be handled differently in the application logic
	// as they are not direct columns in the strategies table's criteria JSON based on init.sql
	// score?: number;
	// activeScreen?: boolean;
}

// Updated Strategy interface based on the 'strategies' table in init.sql
export interface Strategy {
	strategyId: number;
	userId: number;
	name: string;
	criteria: StrategyCriteria; // Corresponds to the JSON column
}

export interface Watchlist {
	watchlistName: string;
	watchlistId: number;
}
export interface CloseData {
	price: number;
	channel: string;
}
export interface TradeData {
	timestamp: number;
	price: number;
	size: number;
	exchange: number;
	prevClose: number;
	conditions: Array<number>;
	shouldUpdatePrice: boolean;
}
export interface QuoteData {
	timestamp: number;
	bidPrice: number;
	askPrice: number;
	bidSize: number;
	askSize: number;
}
export interface Settings {
	chartColumns: number;
	chartRows: number;
	dolvol: boolean;
	adrPeriod: number;
	divideTaS: boolean;
	filterTaS: boolean;
	showFilings: boolean;
	// DEPRECATED: Screensaver properties
	// enableScreensaver: boolean;
	// screensaverTimeframes: string[];
	// screensaverSpeed: number;
	// screensaverTimeout: number;
	// screensaverDataSource: 'gainers-losers' | 'watchlist' | 'user-defined';
	// screensaverWatchlistId?: number;
	// screensaverTickers?: string[];
	colorScheme: 'default' | 'dark-blue' | 'midnight' | 'forest' | 'sunset';
}
export interface StreamInfo {
	status: 'replay' | 'realtime' | 'paused';
	startTimestamp: number | null;
	replaySpeed: number;
}
export interface AlertData {
	message: string;
	alertId: number;
	timestamp: number;
	securityId: number;
	channel: string;
	tickers: string[];
}
export interface AlertLog {
	alertLogId: number;
	alertId: number; // Refers to either PriceAlert or StrategyAlert ID
	timestamp: string; // Assuming timestamp comes as string, adjust if Date object
	securityId: number;
}

// Generic Alert configuration used in frontend components.
// May represent either a PriceAlert or StrategyAlert configuration.
export interface Alert {
	active?: boolean;
	alertId?: number; // Could be priceAlertId or strategyAlertId
	alertType: 'price' | 'strategy' | string; // Type discriminator
	strategyId?: number; // Replaces setupId, relevant for strategy alerts
	securityId?: string | number; // Can be string (ticker) or number (securityId)
	ticker?: string;
	price?: number; // Target price for price alerts
	direction?: boolean; // Direction for price/strategy alerts
	alertPrice?: number; // Ensure this field is present
}

// Specific type for Price Alerts based on 'priceAlerts' table
export interface PriceAlert {
	priceAlertId: number;
	userId: number;
	active: boolean;
	price: number; // Using number, adjust if DECIMAL needs string representation
	direction: boolean;
	securityID: number;
}

// Specific type for Strategy Alerts based on 'strategyAlerts' table
export interface StrategyAlert {
	strategyAlertId: number;
	userId: number;
	active: boolean;
	strategyId: number;
	direction: boolean;
	securityID: number;
}

// Interface for Studies based on the 'studies' table
export interface Study {
	studyId: number;
	userId: number;
	securityId?: number;
	strategyId?: number;
	timestamp: string; // Assuming timestamp comes as string, adjust if Date object
	tradeId?: number;
	completed: boolean;
	entry: any; // JSON blob, define more strictly if structure is known
}

export interface Trade {
	time: number;
	type: string;
	price: number;
	shares: number;
	tradeId?: number;
	trade_direction?: string;
	status?: string;
	openQuantity?: number;
	closedPnL?: number | null;
}

export type LogEntry = {
	timestamp: string;
	message: string;
	level: string;
};

export type TaskState = 'queued' | 'running' | 'completed' | 'failed';

export type Task = {
	id: string;
	state: TaskState;
	function: string;
	args: any;
	result?: any;
	error?: string;
	logs: LogEntry[];
	createdAt: string;
	updatedAt: string;
};

export type PollResponse = {
	task: Task;
	newLogs: boolean;
	logsLength: number;
};

// Note-related types
export type Note = {
	noteId: number;
	userId: number;
	title: string;
	content: string;
	category: string;
	tags: string[];
	createdAt: string;
	updatedAt: string;
	isPinned: boolean;
	isArchived: boolean;
};

export type NoteFilter = {
	category?: string;
	tags?: string[];
	isPinned?: boolean;
	isArchived?: boolean;
	searchQuery?: string;
};

export type SearchResult = {
	note: Note;
	rank: number;
	titleHighlight: string;
	contentHighlight: string;
};
