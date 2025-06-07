<script lang="ts">
import { onMount } from 'svelte';
import { writable, get, derived } from 'svelte/store';
import { strategies } from '$lib/utils/stores/stores';
import { privateRequest } from '$lib/utils/helpers/backend';
import '$lib/styles/global.css';

// --- Interfaces ---
type StrategyId = number | 'new' | null;
interface UniverseFilterSpec { securityFeature: "SecurityId" | "Ticker" | "Locale" | "Market" | "PrimaryExchange" | "Active" | "Sector" | "Industry"; include: string[]; exclude: string[]; }
interface UniverseSpec { filters: UniverseFilterSpec[]; timeframe: "1" | "1h" | "1d" | "1w"; extendedHours: boolean; startTime: string | null; endTime: string | null; }
interface SourceSpec { field: "SecurityId" | "Ticker" | "Locale" | "Market" | "PrimaryExchange" | "Active" | "Sector" | "Industry"; value: string; }
// MODIFIED: Added offset to ExprElement
interface ExprElement { type: "column" | "operator"; value: string; offset: number; } // offset added
interface FeatureSpec {
	name: string;
	featureId: number;
	source: SourceSpec;
	output: "raw" | "rankn" | "rankp";
	expr: ExprElement[]; // RPN representation for backend/internal use
	infixExpr: string; // User-facing infix expression for editing
	exprError?: string; // To store parsing errors during edit
	window: number;
}
interface RhsSpec { featureId: number; const: number; scale: number; }
interface FilterSpec { name: string; lhs: number; operator: "<" | "<=" | ">=" | ">" | "!=" | "=="; rhs: RhsSpec; }
interface SortBySpec { feature: number; direction: "asc" | "desc"; }
interface NewStrategySpec { universe: UniverseSpec; features: FeatureSpec[]; filters: FilterSpec[]; sortBy: SortBySpec; }

// Modified Strategy interface: spec is now optional in the base list
interface Strategy {
	strategyId: number;
	name: string;
	spec?: NewStrategySpec; // Spec is optional now for list items
	score?: number;
	version?: string | number;
	createdAt?: string;
	isAlertActive?: boolean;
}
// EditableStrategy still requires the spec
interface EditableStrategy extends Omit<Strategy, 'spec'> {
	strategyId: StrategyId;
	spec: NewStrategySpec; // Spec is required for editing/creating
}

// --- Stores ---
const loading = writable(false); // General loading state
const loadingSpec = writable(false); // Specific loading state for fetching spec details
const selectedStrategyId = writable<StrategyId>(null);
const editedStrategy = writable<EditableStrategy | null>(null);
const viewedStrategyId = writable<number | null>(null);
const detailViewError = writable<string | null>(null); // For errors loading spec

// Derived store to get the currently viewed strategy object from the main list
// Note: This might initially lack the 'spec' until it's fetched.
const viewedStrategyBase = derived(
	[viewedStrategyId, strategies],
	([$viewedStrategyId, $strategies]) => {
		if ($viewedStrategyId === null) return null;
		return $strategies.find(s => s.strategyId === $viewedStrategyId) || null;
	}
);

// --- Constants & Options ---
const timeframeOptions = [ { value: '1', label: '1 Minute' }, { value: '1h', label: '1 Hour' }, { value: '1d', label: '1 Day' }, { value: '1w', label: '1 Week' } ];
const securityFeatureOptions = ["SecurityId", "Ticker", "Locale", "Market", "PrimaryExchange", "Active", "Sector", "Industry"];
const operatorOptions = ["<", "<=", ">=", ">", "!=", "=="];
const outputOptions = ["raw", "rankn", "rankp"];
const sortDirectionOptions = ["asc", "desc"];
const baseColumnOptions = ["open", "high", "low", "close", "volume", "market_cap", "shares_outstanding", "eps", "revenue", "dividend", "social_sentiment", "fear_greed", "short_interest", "borrow_fee"];
const operatorChars = ["+", "-", "*", "/", "^"];
const operatorPrecedence: { [key: string]: number } = { '+': 1, '-': 1, '*': 2, '/': 2, '^': 3 };


// --- Help Text ---
// Updated help text for expression input to mention offset syntax
const helpText = { universeTimeframe: "Select the primary timeframe resolution for the strategy (e.g., 1d for daily).", universeExtendedHours: "Include pre-market and after-hours data? (Only applicable for 1-minute timeframe).", universeFilters: "Define the initial pool of securities using filters (e.g., Sector = Technology, Ticker includes AAPL).", universeTickerInclude: "Enter specific tickers to include (e.g. AAPL, MSFT). Press Enter after each ticker.", universeTickerExclude: "Enter specific tickers to exclude (e.g. TSLA, META). Press Enter after each ticker.", features: "Define calculated values (Features) used in your strategy's filters or sorting. Features use standard infix notation (parentheses) for expressions.", featureExpr: "Expression using standard math notation. Use parentheses () for grouping, valid columns (e.g., 'close', 'volume') and operators (+, -, *, /, ^). Use column[offset] for previous values (e.g., 'close[1]' for previous close). Example: '(high - low[1]) / 2'", featureWindow: "Smoothing window for the expression (e.g., 14 for a 14-period average). 1 means no smoothing.", filters: "Define the conditions your strategy uses to select securities, comparing Features against constants or other Features.", sortBy: "Select a Feature to sort the final results by, and the direction (ascending/descending)." };

// --- Formatting/Helper Functions ---
function formatName(name: string | undefined): string { if (!name) return 'Unnamed'; return name.replace(/_/g, ' ').toLowerCase().split(' ').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' '); }

// MODIFIED: RPN to Infix formatter to include offset syntax
function formatExprInfix(expr: ExprElement[] | undefined): string {
	if (!expr || expr.length === 0) return '(Empty Expression)';
	const stack: { value: string, precedence: number }[] = [];
	for (const element of expr) {
		if (element.type !== 'operator') { // Treat as operand (column)
			let valueStr = formatName(element.value);
			// Append offset if > 0
			if (element.offset && element.offset > 0) {
				valueStr += `[${element.offset}]`;
			}
			stack.push({ value: valueStr, precedence: Infinity });
		} else if (element.type === 'operator') {
			const opPrecedence = operatorPrecedence[element.value] ?? 0;
			if (stack.length < 2) return `(Error: Invalid RPN near '${element.value}')`;
			const operand2 = stack.pop()!;
			const operand1 = stack.pop()!;
			// Add parentheses if operand precedence is lower, or equal for right operand (for left-associativity)
			const val1 = operand1.precedence < opPrecedence ? `(${operand1.value})` : operand1.value;
			const val2 = operand2.precedence <= opPrecedence ? `(${operand2.value})` : operand2.value;
			stack.push({ value: `${val1} ${element.value} ${val2}`, precedence: opPrecedence });
		}
	}
	if (stack.length !== 1) return '(Error: Invalid RPN expression)';
	return stack[0].value;
}

function formatTimeframe(tf: string | undefined): string { return timeframeOptions.find(opt => opt.value === tf)?.label ?? tf ?? 'N/A'; }
function formatFilterCondition(filter: FilterSpec, features: FeatureSpec[]): string { const lhsFeature = features.find(f => f.featureId === filter.lhs); const lhsName = formatName(lhsFeature?.name); let rhsDesc: string; if (filter.rhs.featureId !== 0) { const rhsFeature = features.find(f => f.featureId === filter.rhs.featureId); rhsDesc = formatName(rhsFeature?.name); if (filter.rhs.scale !== 1.0) { rhsDesc += ` * ${filter.rhs.scale}`; } } else { rhsDesc = `${filter.rhs.const}`; if (filter.rhs.scale !== 1.0) { rhsDesc = `${filter.rhs.const * filter.rhs.scale} (${filter.rhs.const} * ${filter.rhs.scale})`; } } return `${lhsName} ${filter.operator} ${rhsDesc}`; }
function formatUniverseFilter(uFilter: UniverseFilterSpec): string { let desc = `${uFilter.securityFeature}`; if (uFilter.include.length > 0) desc += ` includes [${uFilter.include.join(', ')}]`; if (uFilter.exclude.length > 0) desc += `${uFilter.include.length > 0 ? ' and' : ''} excludes [${uFilter.exclude.join(', ')}]`; return desc; }


// --- Expression Parsing (Infix to RPN) ---
// MODIFIED: Basic Shunting-yard to handle column[offset] syntax
function parseInfixToRPN(infix: string): { success: boolean; rpn?: ExprElement[]; error?: string } {
	if (!infix?.trim()) {
		return { success: true, rpn: [] }; // Empty infix is valid, results in empty RPN
	}

	const outputQueue: ExprElement[] = [];
	const operatorStack: string[] = [];
	// Regex to find: words optionally followed by [digits], or operators/parentheses/brackets/digits
	const tokens = infix.match(/(\b\w+\b(?:\[\d+\])?|[+\-*/^()\[\]]|\d+)/g);

	if (!tokens) {
		return { success: false, error: "Could not tokenize expression." };
	}

	const columnOffsetRegex = /^(\b\w+\b)\[(\d+)\]$/; // Matches word[digits]

	for (const token of tokens) {
		// Check if token is column with offset like "close[1]"
		const offsetMatch = token.match(columnOffsetRegex);
		if (offsetMatch) {
			const columnName = offsetMatch[1];
			const offset = parseInt(offsetMatch[2], 10);
			if (baseColumnOptions.includes(columnName.toLowerCase())) {
				if (offset < 0) {
					 return { success: false, error: `Offset cannot be negative: ${token}` };
				}
				outputQueue.push({ type: "column", value: columnName, offset: offset });
			} else {
				return { success: false, error: `Unrecognized column name: ${columnName} in ${token}` };
			}
		}
		// Check if token is simple column name like "close"
		else if (baseColumnOptions.includes(token.toLowerCase())) {
			outputQueue.push({ type: "column", value: token, offset: 0 }); // Default offset 0
		}
		// Check if token is an operator
		else if (operatorChars.includes(token)) {
			while (
				operatorStack.length > 0 &&
				operatorStack[operatorStack.length - 1] !== '(' &&
				(operatorPrecedence[operatorStack[operatorStack.length - 1]] ?? 0) >= (operatorPrecedence[token] ?? 0)
			) {
				outputQueue.push({ type: "operator", value: operatorStack.pop()!, offset: 0 }); // Operators always have offset 0
			}
			operatorStack.push(token);
		} else if (token === '(') {
			operatorStack.push(token);
		} else if (token === ')') {
			while (operatorStack.length > 0 && operatorStack[operatorStack.length - 1] !== '(') {
				outputQueue.push({ type: "operator", value: operatorStack.pop()!, offset: 0 }); // Operators always have offset 0
			}
			if (operatorStack.length === 0 || operatorStack[operatorStack.length - 1] !== '(') {
				return { success: false, error: "Mismatched parentheses." };
			}
			operatorStack.pop(); // Discard the '('
		}
		// Ignore bracket/digit tokens if they weren't part of column[offset] pattern
		// This prevents standalone '[' or digits from causing errors if used incorrectly.
		// A more robust parser might error here.
		else if (token === '[' || token === ']' || /^\d+$/.test(token)) {
			// Ignored for now, unless part of column[offset] handled above
			console.warn(`Ignoring standalone token: ${token}. Use syntax like column[offset].`);
		}
		else {
			// Handle numeric literals if needed in the future, but spec says no constants in expr
			// For now, consider any other token an error
			 return { success: false, error: `Unrecognized token: ${token}` };
		}
	}

	// Pop remaining operators from the stack to the output queue
	while (operatorStack.length > 0) {
		const op = operatorStack.pop()!;
		if (op === '(') {
			return { success: false, error: "Mismatched parentheses." };
		}
		outputQueue.push({ type: "operator", value: op, offset: 0 }); // Operators always have offset 0
	}

	// Validation: RPN structure check
	let stackHeight = 0;
	for (const el of outputQueue) {
		if (el.type === 'column') {
			stackHeight++;
		} else if (el.type === 'operator') {
			stackHeight--; // Assumes binary ops
		}
		// Check for errors during evaluation (e.g., operator without enough operands)
		if (stackHeight < 0) {
			return { success: false, error: "Invalid RPN sequence (operator before required operands)." };
		}
	}

	// Final stack height should be 1 for a valid expression, unless the queue is empty
	if (outputQueue.length > 0 && stackHeight !== 1) {
		return { success: false, error: `Invalid RPN structure (final stack height ${stackHeight}, expected 1).` };
	}
	// Allow empty RPN for empty input
	if (outputQueue.length === 0 && infix.trim() !== "") {
		return { success: false, error: "Parsing resulted in empty RPN for non-empty input." };
	}


	return { success: true, rpn: outputQueue };
}

// --- Utility Functions ---
function getNextFeatureId(features: FeatureSpec[]): number { if (!features || features.length === 0) return 0; return Math.max(...features.map(f => f.featureId)) + 1; }

// MODIFIED: Ensure default feature has offset 0 in its ExprElement
function blankSpec(): NewStrategySpec {
	const defaultFeatureId = 0;
	const defaultInfix = "close"; // Default is current close
	const parseResult = parseInfixToRPN(defaultInfix); // Parses to { type: "column", value: "close", offset: 0 }
	return {
		universe: { filters: [], timeframe: '1d', extendedHours: false, startTime: null, endTime: null },
		features: [ {
			name: "close_price",
			featureId: defaultFeatureId,
			source: { field: "SecurityId", value: "relative" },
			output: "raw",
			infixExpr: defaultInfix, // Store default infix
			expr: parseResult.rpn ?? [], // Use parsed RPN (includes offset: 0)
			window: 1
		} ],
		filters: [],
		sortBy: { feature: defaultFeatureId, direction: 'desc' }
	};
}

// MODIFIED: Ensure offsets are handled correctly during validation/fallback
function ensureValidSpec(spec: any): NewStrategySpec {
	const validSpec = blankSpec(); // Start with a blank, valid structure
	if (!spec) return validSpec;

	// Universe validation (unchanged)
	if (spec.universe) {
		if (Array.isArray(spec.universe.filters)) validSpec.universe.filters = spec.universe.filters;
		if (typeof spec.universe.timeframe === 'string' && ['1', '1h', '1d', '1w'].includes(spec.universe.timeframe)) {
			validSpec.universe.timeframe = spec.universe.timeframe as "1" | "1h" | "1d" | "1w";
		}
		if (typeof spec.universe.extendedHours === 'boolean') validSpec.universe.extendedHours = spec.universe.extendedHours;
		validSpec.universe.startTime = typeof spec.universe.startTime === 'string' && spec.universe.startTime !== "" ? spec.universe.startTime : null;
		validSpec.universe.endTime = typeof spec.universe.endTime === 'string' && spec.universe.endTime !== "" ? spec.universe.endTime : null;
	}

	// Features validation (modified for infix/RPN with offset)
	const tempFeatures: FeatureSpec[] = [];
	if (Array.isArray(spec.features) && spec.features.length > 0) {
		spec.features.forEach((f: any) => {
			const nextId = getNextFeatureId(tempFeatures);
			const featureId = typeof f.featureId === 'number' ? f.featureId : nextId;
			const name = typeof f.name === 'string' ? f.name : `feature_${featureId}`;
			const source = f.source && typeof f.source.field === 'string' ? f.source : { field: "SecurityId", value: "relative" };
			const output = f.output && ["raw", "rankn", "rankp"].includes(f.output) ? f.output : "raw";
			const window = typeof f.window === 'number' && f.window >= 1 ? f.window : 1;

			let infixExpr = "";
			let expr: ExprElement[] = [];
			let exprError: string | undefined = undefined;

			if (typeof f.infixExpr === 'string' && f.infixExpr.trim()) {
				// Priority: Use infixExpr if provided
				infixExpr = f.infixExpr;
				const parseResult = parseInfixToRPN(infixExpr);
				if (parseResult.success) {
					expr = parseResult.rpn!; // Parser now includes offset
				} else {
					expr = []; // Set RPN to empty on parse failure
					exprError = parseResult.error || "Failed to parse expression.";
					console.warn(`Feature '${name}' (ID: ${featureId}) infix expression parsing failed: ${exprError}`);
				}
			} else if (Array.isArray(f.expr) && f.expr.length > 0) {
				// Fallback: Use RPN expr if provided and infixExpr is missing
				// Ensure incoming RPN has offset (default to 0 if missing) and validate
				expr = f.expr.map((el: any) => {
					const validEl: ExprElement = {
						type: el.type === 'column' || el.type === 'operator' ? el.type : 'column', // Default to column if invalid type
						value: typeof el.value === 'string' ? el.value : '',
						offset: typeof el.offset === 'number' && el.offset >= 0 ? el.offset : 0
					};
					// Validate offset constraints specifically for this path
					if (validEl.type === 'operator' && validEl.offset !== 0) {
						console.warn(`Invalid RPN data for Feature '${name}' (ID: ${featureId}): Operator '${validEl.value}' had non-zero offset ${el.offset}. Resetting to 0.`);
						validEl.offset = 0;
						exprError = exprError || "Invalid operator offset found in RPN data.";
					}
                    if (validEl.type === 'column' && validEl.offset < 0) {
                         console.warn(`Invalid RPN data for Feature '${name}' (ID: ${featureId}): Column '${validEl.value}' had negative offset ${el.offset}. Resetting to 0.`);
                         validEl.offset = 0;
                         exprError = exprError || "Negative column offset found in RPN data.";
                    }
					return validEl;
				});
				// Attempt to generate a displayable infix string from the RPN
				infixExpr = formatExprInfix(expr); // Formatter now includes offset
				// Check if formatting resulted in an error message
				if (infixExpr.startsWith('(Error:')) {
						 console.warn(`Feature '${name}' (ID: ${featureId}) RPN could not be formatted to infix: ${infixExpr}`);
						 exprError = exprError || "Invalid RPN data could not be formatted.";
				}
			} else {
				 // No expression provided
				 infixExpr = "";
				 expr = [];
				 exprError = "Expression is missing.";
				 console.warn(`Feature '${name}' (ID: ${featureId}) has no expression defined.`);
			}

			tempFeatures.push({
				name, featureId, source, output, window,
				infixExpr, expr, exprError // Store all derived values
			});
		});
		validSpec.features = tempFeatures;
	} else {
		// No features provided, use the default from blankSpec
		validSpec.features = blankSpec().features;
	}

	// Filter and SortBy validation (mostly unchanged, relies on validFeatureIds)
	const validFeatureIds = new Set(validSpec.features.map(f => f.featureId));
	if (Array.isArray(spec.filters)) {
		validSpec.filters = spec.filters
			.filter((f: any) => f && typeof f.lhs === 'number' && validFeatureIds.has(f.lhs)) // Ensure LHS feature exists
			.map((f: any) => ({
				name: typeof f.name === 'string' ? f.name : `filter_${f.lhs}`,
				lhs: f.lhs,
				operator: f.operator && operatorOptions.includes(f.operator) ? f.operator : ">",
				rhs: {
					featureId: typeof f.rhs?.featureId === 'number' && (f.rhs.featureId === 0 || validFeatureIds.has(f.rhs.featureId)) ? f.rhs.featureId : 0, // Ensure RHS feature exists if not const
					const: typeof f.rhs?.const === 'number' ? f.rhs.const : 0.0,
					scale: typeof f.rhs?.scale === 'number' ? f.rhs.scale : 1.0
				}
			}));
	}

	if (spec.sortBy && typeof spec.sortBy.feature === 'number' && validFeatureIds.has(spec.sortBy.feature)) {
		validSpec.sortBy.feature = spec.sortBy.feature;
		if (typeof spec.sortBy.direction === 'string' && sortDirectionOptions.includes(spec.sortBy.direction)) {
			validSpec.sortBy.direction = spec.sortBy.direction as "asc" | "desc";
		}
	} else if (validSpec.features.length > 0) {
		// Fallback sort: use the first valid feature if the original sort feature is invalid or missing
		validSpec.sortBy = { feature: validSpec.features[0].featureId, direction: 'desc' };
	} else {
		// No features, use the default sort from blankSpec (though it might point to a non-existent feature)
		validSpec.sortBy = blankSpec().sortBy;
	}

	return validSpec;
}


// --- Data Loading & CRUD Actions ---

async function loadStrategies() {
	loading.set(true);
	viewedStrategyId.set(null);
	selectedStrategyId.set(null);
	detailViewError.set(null);
	try {
		// Fetch basic strategy info (without spec)
		const data = await privateRequest<Omit<Strategy, 'spec'>[]>('getStrategies', {}); // Expect data without spec
		const initialStrategies: Strategy[] = (data || []).map((d: any) => ({
			...d,
			// Initialize other fields if needed, spec will be fetched on demand
			version: d.version ?? '1.0',
			createdAt: d.createdAt ?? new Date(Date.now() - Math.random() * 1e10).toISOString(),
			isAlertActive: d.isAlertActive ?? (Math.random() > 0.7),
			spec: undefined // Explicitly set spec to undefined initially
		}));
		strategies.set(initialStrategies);
	} catch (error) {
		console.error("Error loading strategies:", error);
		strategies.set([]);
		detailViewError.set("Failed to load strategy list.");
	} finally {
		loading.set(false);
	}
}

onMount(loadStrategies);

// Function to fetch spec for a given strategy ID and update the store
async function fetchAndStoreSpec(id: number) {
	const currentStrategies = get(strategies);
	const targetStrategy = currentStrategies.find(s => s.strategyId === id);

	// Only fetch if the strategy exists and doesn't already have a spec
	if (targetStrategy && !targetStrategy.spec) {
		loadingSpec.set(true);
		detailViewError.set(null);
		try {
			const specData = await privateRequest<NewStrategySpec>('getStrategySpec', { strategyId: id });
			// Validate fetched spec, including parsing infix if needed and handling offsets
			const validSpec = ensureValidSpec(specData);

			// Update the strategy in the main store
			strategies.update(current =>
				current.map(s =>
					s.strategyId === id ? { ...s, spec: validSpec } : s
				)
			);
			// Check if validation found errors during spec processing
			if (validSpec.features.some(f => f.exprError)) {
				 detailViewError.set(`Strategy ${id} loaded, but some feature expressions have errors. Please edit to fix.`);
			}

		} catch (error: any) {
			console.error(`Error fetching spec for strategy ${id}:`, error);
			detailViewError.set(`Failed to load details for strategy ${id}: ${error.message || 'Unknown error'}`);
			// Optionally remove the strategy from view if spec fails? Or keep showing base info?
			// For now, we just show the error.
		} finally {
			loadingSpec.set(false);
		}
	} else if (!targetStrategy) {
		 detailViewError.set(`Strategy with ID ${id} not found in the list.`);
	} else if (targetStrategy.spec && targetStrategy.spec.features.some(f => f.exprError)){
		// Spec already loaded, but has errors from initial load/validation
		detailViewError.set(`Displaying strategy ${id}, but some feature expressions have errors. Please edit to fix.`);
	}
	 else {
		// Spec already loaded and seems okay, clear any previous error for this ID
		 detailViewError.set(null);
	}
}

// --- Reactive Statement ---
// When viewedStrategyId changes, fetch the spec if needed
$: if ($viewedStrategyId !== null) {
	fetchAndStoreSpec($viewedStrategyId);
}


function viewStrategy(id: number) {
	selectedStrategyId.set(null);
	editedStrategy.set(null);
	detailViewError.set(null); // Clear previous errors
	viewedStrategyId.set(id); // This will trigger the reactive statement above
}

// MODIFIED: Ensure blank spec includes offset 0 in default feature RPN
function startCreate() {
	const newStrategy: EditableStrategy = {
		strategyId: 'new',
		name: '',
		spec: blankSpec(), // Create uses a blank spec (with default 'close[0]' feature)
		version: '1.0',
		createdAt: new Date().toISOString(),
		isAlertActive: false,
	};
	viewedStrategyId.set(null);
	detailViewError.set(null);
	editedStrategy.set(newStrategy);
	selectedStrategyId.set('new');
}

// Modified startEdit to fetch spec if needed
async function startEdit(id: number | null) {
	if (id === null || typeof id !== 'number') return;

	loading.set(true); // Use general loading indicator for edit prep
	detailViewError.set(null);
	selectedStrategyId.set(null); // Reset selection first
	editedStrategy.set(null);

	try {
		let strategyToEdit = get(strategies).find(s => s.strategyId === id);
		let specToUse: NewStrategySpec;

		if (!strategyToEdit) {
			throw new Error(`Strategy with ID ${id} not found.`);
		}

		// Check if spec needs fetching or if it exists but failed validation previously
		if (!strategyToEdit.spec || strategyToEdit.spec.features.some(f=>f.exprError)) {
			console.log(`Spec for strategy ${id} not found locally or has errors, fetching/re-validating...`);
			loadingSpec.set(true); // Show spec loading indicator briefly if needed
			const specData = await privateRequest<NewStrategySpec>('getStrategySpec', { strategyId: id });
			specToUse = ensureValidSpec(specData); // Re-validate fetched/existing data, handles offsets
			 loadingSpec.set(false);
			 // Update the store immediately so the base object has the potentially corrected spec for cloning
			 strategies.update(current =>
				current.map(s =>
					s.strategyId === id ? { ...s, spec: specToUse } : s
				)
			);
			 // Re-fetch the strategy object now that it includes the updated spec
			 strategyToEdit = get(strategies).find(s => s.strategyId === id);
			 if (!strategyToEdit || !strategyToEdit.spec) {
				  throw new Error(`Failed to retrieve or update strategy ${id} after fetching/validating spec.`);
			 }
			 // Check again for validation errors after fetching
			 if (specToUse.features.some(f => f.exprError)) {
				  console.warn(`Strategy ${id} spec loaded for editing, but validation found expression errors.`);
				  // We still allow editing, errors will be shown in the form.
			 }
		} else {
			// Spec already exists and seems valid, ensure it's freshly validated before editing
			// (ensureValidSpec is idempotent, so this is safe)
			specToUse = ensureValidSpec(strategyToEdit.spec); // Handles offsets
		}


		// Clone the strategy *with* the validated spec for editing
		// Make sure the cloned object matches EditableStrategy structure
		const clonedStrategy: EditableStrategy = {
			 strategyId: strategyToEdit.strategyId,
			 name: strategyToEdit.name,
			 spec: JSON.parse(JSON.stringify(specToUse)), // Deep clone the validated spec (includes offsets)
			 score: strategyToEdit.score,
			 version: strategyToEdit.version,
			 createdAt: strategyToEdit.createdAt,
			 isAlertActive: strategyToEdit.isAlertActive,
		};

		// Clear any exprError flags on the cloned spec for the edit session
		// Errors will be re-evaluated on blur/save
		clonedStrategy.spec.features.forEach(f => f.exprError = undefined);

		editedStrategy.set(clonedStrategy);
		selectedStrategyId.set(id);
		viewedStrategyId.set(null); // Exit detail view

	} catch (error: any) {
		 console.error(`Error preparing strategy ${id} for editing:`, error);
		 detailViewError.set(`Failed to prepare strategy for editing: ${error.message || 'Unknown error'}`);
		 // Reset states if edit fails
		 selectedStrategyId.set(null);
		 editedStrategy.set(null);
	} finally {
		loading.set(false);
		loadingSpec.set(false);
	}
}


function cancelEdit() {
	const editState = get(editedStrategy);
	editedStrategy.set(null);
	selectedStrategyId.set(null);
	detailViewError.set(null);
	// If cancelling an edit, go back to viewing that strategy.
	if (editState && typeof editState.strategyId === 'number') {
	 	viewStrategy(editState.strategyId);
	} else {
	 	// If it was 'new', go back to list view
	 	viewedStrategyId.set(null);
	}
}

async function deleteStrategyConfirm(id: number | null) {
	if (id === null || typeof id !== 'number') return;
	if (!confirm(`Are you sure you want to delete strategy '${get(strategies).find(s=>s.strategyId===id)?.name ?? `ID: ${id}`}'?`)) return;
	loading.set(true);
	try {
		await privateRequest('deleteStrategy', { strategyId: id });
		strategies.update(arr => arr.filter(s => s.strategyId !== id));
		viewedStrategyId.set(null);
		selectedStrategyId.set(null);
		editedStrategy.set(null);
		detailViewError.set(null);
	} catch (error) {
		console.error("Error deleting strategy:", error);
		alert("Failed to delete strategy.");
		detailViewError.set("Failed to delete strategy.");
	} finally {
	 	loading.set(false);
	}
}

async function saveStrategy() {
	const currentStrategy = get(editedStrategy);
	if (!currentStrategy) return;

	// --- Validation ---
	if (!currentStrategy.name.trim()) { alert("Strategy Name cannot be empty."); return; }
	if (!currentStrategy.spec?.features || currentStrategy.spec.features.length === 0) { alert("Strategy must have at least one Feature."); return; }

	// Re-parse all infix expressions to ensure RPN 'expr' is up-to-date and valid before saving
	let hasExprError = false;
	currentStrategy.spec.features.forEach((feature, index) => {
		const result = parseInfixToRPN(feature.infixExpr); // Parser now handles offset syntax
		if (!result.success) {
			// Update the store directly to show the error in the UI immediately if save fails
			updateEditedStrategy(s => { s.spec.features[index].exprError = result.error || "Invalid expression"; });
			hasExprError = true;
			console.error(`Feature '${feature.name}' expression error: ${result.error}`);
		} else {
			// Update the RPN 'expr' field in the strategy being saved (includes offset)
			currentStrategy.spec.features[index].expr = result.rpn!;
			// Clear any previous error for this feature in the edited state
			updateEditedStrategy(s => { s.spec.features[index].exprError = undefined; });
		}
	});

	if (hasExprError) {
		alert("One or more feature expressions are invalid. Please correct the errors shown in the Features section.");
		return; // Stop saving
	}

	// Continue with existing ID and filter validations
	const featureIds = currentStrategy.spec.features.map(f => f.featureId);
	if (new Set(featureIds).size !== featureIds.length) { alert("Feature IDs must be unique."); return; }
	const existingFeatureIds = new Set(featureIds);
	let invalidFilterFound = false;
	currentStrategy.spec.filters.forEach((filter, index) => { if (!existingFeatureIds.has(filter.lhs)) { alert(`Filter ${index + 1} uses non-existent LHS Feature ID ${filter.lhs}.`); invalidFilterFound = true; } if (filter.rhs.featureId !== 0 && !existingFeatureIds.has(filter.rhs.featureId)) { alert(`Filter ${index + 1} uses non-existent RHS Feature ID ${filter.rhs.featureId}.`); invalidFilterFound = true; } });
	if (!existingFeatureIds.has(currentStrategy.spec.sortBy.feature)) { alert(`Sort By uses non-existent Feature ID ${currentStrategy.spec.sortBy.feature}.`); invalidFilterFound = true; }
	if (invalidFilterFound) return;
	// --- End Validation ---

	loading.set(true);
	detailViewError.set(null);

	// Prepare spec for backend: RPN 'expr' now includes offset. Remove UI fields.
	const cleanSpecForBackend: Omit<NewStrategySpec, 'features'> & { features: Omit<FeatureSpec, 'infixExpr' | 'exprError'>[] } = {
		...currentStrategy.spec,
		features: currentStrategy.spec.features.map(({ infixExpr, exprError, ...rest }) => rest) // Remove UI-specific fields
	};

	// Payload only needs name and the cleaned spec
	const payload = {
		name: currentStrategy.name,
		spec: cleanSpecForBackend // Send the spec with RPN including offsets
	};
	console.log("Saving strategy with payload:", JSON.stringify(payload, null, 2));

	try {
		let savedStrategyId: number | null = null;

		if (currentStrategy.strategyId === 'new') {
			// Create new strategy
			const createdResult = await privateRequest<{strategyId: number, name: string}>('newStrategy', payload);
			const created: Strategy = {
				strategyId: createdResult.strategyId,
				name: createdResult.name,
				spec: ensureValidSpec(currentStrategy.spec), // Use validated spec from edit state for local store (includes offsets)
				version: currentStrategy.version ?? '1.0',
				createdAt: currentStrategy.createdAt ?? new Date().toISOString(),
				isAlertActive: currentStrategy.isAlertActive ?? false,
				score: 0
			};
			strategies.update(arr => [...arr, created]);
			savedStrategyId = created.strategyId;

		} else if (typeof currentStrategy.strategyId === 'number') {
			// Update existing strategy
			await privateRequest('setStrategy', {
				strategyId: currentStrategy.strategyId,
				...payload
			});
			savedStrategyId = currentStrategy.strategyId;

			// Update the local store with the edited data (using validated spec)
			const updatedSpec = ensureValidSpec(currentStrategy.spec); // Use the spec from the edit state (includes offsets)
			strategies.update(arr => arr.map(s =>
				s.strategyId === savedStrategyId
				? { ...s,
					name: payload.name,
					spec: updatedSpec, // Store the fully validated spec locally
					version: currentStrategy.version,
					isAlertActive: currentStrategy.isAlertActive
				  }
				: s
			));
		}

		// Success: Clear edit state and view the saved strategy
		editedStrategy.set(null);
		selectedStrategyId.set(null);
		if (savedStrategyId !== null) {
			viewStrategy(savedStrategyId); // View the strategy just saved/created
		} else {
			 viewedStrategyId.set(null); // Fallback to list if ID is somehow null
		}

	} catch (error: any) {
		console.error("Error saving strategy:", error);
		const errorMsg = error?.response?.data?.error || error.message || "An unknown error occurred.";
		alert(`Failed to save strategy: ${errorMsg}`);
		detailViewError.set(`Failed to save strategy: ${errorMsg}`);
	} finally {
		loading.set(false);
	}
}


// --- UI Update Helpers (Edit View) ---
// updateEditedStrategy remains useful for deep updates
function updateEditedStrategy(updater: (strategy: EditableStrategy) => void) { editedStrategy.update(strategy => { if (!strategy) return null; const clone = JSON.parse(JSON.stringify(strategy)); updater(clone); return clone; }); }

// Universe filter functions remain the same
function addUniverseFilter() { updateEditedStrategy(s => { s.spec.universe.filters.push({ securityFeature: 'Ticker', include: [], exclude: [] }); }); }
function removeUniverseFilter(index: number) { updateEditedStrategy(s => { s.spec.universe.filters.splice(index, 1); }); }
function addUniverseInclude(filterIndex: number, value: string) { if (!value.trim()) return; updateEditedStrategy(s => { const filter = s.spec.universe.filters[filterIndex]; const upperVal = value.trim().toUpperCase(); if (filter && !filter.include.includes(upperVal)) { filter.include.push(upperVal); filter.exclude = filter.exclude.filter(ex => ex !== upperVal); } }); }
function removeUniverseInclude(filterIndex: number, valueIndex: number) { updateEditedStrategy(s => { s.spec.universe.filters[filterIndex]?.include.splice(valueIndex, 1); }); }
function addUniverseExclude(filterIndex: number, value: string) { if (!value.trim()) return; updateEditedStrategy(s => { const filter = s.spec.universe.filters[filterIndex]; const upperVal = value.trim().toUpperCase(); if (filter && !filter.exclude.includes(upperVal)) { filter.exclude.push(upperVal); filter.include = filter.include.filter(inc => inc !== upperVal); } }); }
function removeUniverseExclude(filterIndex: number, valueIndex: number) { updateEditedStrategy(s => { s.spec.universe.filters[filterIndex]?.exclude.splice(valueIndex, 1); }); }

// MODIFIED: AddFeature ensures default offset 0 in RPN
function addFeature() {
	updateEditedStrategy(s => {
		const newId = getNextFeatureId(s.spec.features);
		s.spec.features.push({
			name: `new_feature_${newId}`,
			featureId: newId,
			source: { field: "SecurityId", value: "relative" },
			output: "raw",
			infixExpr: "", // Start with empty infix
			expr: [],	   // and empty RPN (parser will add offset:0 when populated)
			window: 1,
			exprError: "Expression is required." // Initial error state
		});
		// Update sortby if it was pointing to a removed feature or if this is the first feature
		if (s.spec.features.length === 1 || !s.spec.features.some(f => f.featureId === s.spec.sortBy.feature)) {
			s.spec.sortBy.feature = newId;
		}
	});
}

function removeFeature(index: number) {
	updateEditedStrategy(s => {
		const removedFeatureId = s.spec.features[index]?.featureId;
		s.spec.features.splice(index, 1);
		// Remove filters referencing this feature
		s.spec.filters = s.spec.filters.filter(f =>
			f.lhs !== removedFeatureId && (f.rhs.featureId === 0 || f.rhs.featureId !== removedFeatureId)
		);
		// Update sortby if it was pointing to the removed feature
		if (s.spec.sortBy.feature === removedFeatureId) {
			s.spec.sortBy.feature = s.spec.features[0]?.featureId ?? (blankSpec().sortBy.feature);
		}
	});
}

// MODIFIED: handleInfixParse uses updated parser which handles offset
function handleInfixParse(featureIndex: number) {
	updateEditedStrategy(s => {
		const feature = s.spec.features[featureIndex];
		if (!feature) return;
		const result = parseInfixToRPN(feature.infixExpr); // Parser includes offset handling
		if (result.success) {
			feature.expr = result.rpn!; // RPN includes offset
			feature.exprError = undefined; // Clear error on success
		} else {
			feature.expr = []; // Clear RPN on failure
			feature.exprError = result.error || "Invalid expression";
		}
	});
}

// Filter functions remain the same
function addFilter() { updateEditedStrategy(s => { const defaultLhsFeatureId = s.spec.features[0]?.featureId ?? 0; s.spec.filters.push({ name: `new_filter_${s.spec.filters.length}`, lhs: defaultLhsFeatureId, operator: '>', rhs: { featureId: 0, const: 0, scale: 1.0 } }); }); }
function removeFilter(index: number) { updateEditedStrategy(s => { s.spec.filters.splice(index, 1); }); }

// Derived store for Edit View dropdowns - remains the same
const availableFeatures = derived(editedStrategy, ($editedStrategy) => {
	if (!$editedStrategy || !$editedStrategy.spec?.features) return [];
	// Filter out features with errors? Maybe not, allow fixing them.
	return $editedStrategy.spec.features.map(f => ({ id: f.featureId, name: f.name || `Feature ${f.featureId}` }));
});

</script>

{#if $viewedStrategyId === null && $selectedStrategyId === null}
	<div class="toolbar">
		<button on:click={startCreate} disabled={$loading}>＋ New Strategy</button>
		{#if $loading}<span class="loading-indicator"> Loading...</span>{/if}
		{#if $detailViewError}<div class="error-message">Error: {$detailViewError}</div>{/if}
	</div>

	{#if !$loading && (!$strategies || $strategies.length === 0)}
		<p>No strategies found. Click "＋ New Strategy" to create one.</p>
	{:else if !$loading}
		<div class="table-container">
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>Timeframe</th>
						<th>Version</th>
						<th>Created</th>
						<th>Alert Active</th>
						<th>Score</th>
					</tr>
				</thead>
				<tbody>
					{#each $strategies as s (s.strategyId)}
						<tr class="clickable-row" on:click={() => viewStrategy(s.strategyId)} title="Click to view details">
							<td>{s.name}</td>
							<td>{s.spec ? formatTimeframe(s.spec.universe?.timeframe) : '...'}</td>
							<td>{s.version ?? 'N/A'}</td>
							<td>{s.createdAt ? new Date(s.createdAt).toLocaleDateString() : 'N/A'}</td>
							<td class="alert-status">{s.isAlertActive ? '✔️ Active' : '❌ Inactive'}</td>
							<td>{s.score ?? '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

{:else if $viewedStrategyId !== null && $selectedStrategyId === null}
	{@const strat = $viewedStrategyBase}
	{#if $loadingSpec}
		<div class="loading-container">
			<p>Loading strategy details for '{$viewedStrategyBase?.name ?? `ID: ${$viewedStrategyId}`}'...</p>
			<button class="secondary" on:click={() => { viewedStrategyId.set(null); detailViewError.set(null); }}>← Back to List</button>
		</div>
	{:else if $detailViewError && !strat?.spec}
		<div class="error-container">
				<h2>Error Loading Strategy Details</h2>
				<p class="error-message">{$detailViewError}</p>
				{#if strat} <p>Showing partial information:</p> {/if}
				<button class="secondary" on:click={() => { viewedStrategyId.set(null); detailViewError.set(null); }}>← Back to List</button>
				{#if strat}<button on:click={() => fetchAndStoreSpec($viewedStrategyId)}>Retry Load</button>{/if}
			</div>
			{#if strat}
				<div class="detail-view-container partial-info">
					<div class="detail-view-header">
							<div class="detail-view-title">
								<h2>{formatName(strat.name)} (Details Failed to Load)</h2>
								<div class="detail-view-meta">
									<span>Version: {strat.version ?? 'N/A'}</span> |
									<span>Created: {strat.createdAt ? new Date(strat.createdAt).toLocaleString() : 'N/A'}</span> |
									<span class="alert-status">Alert: {strat.isAlertActive ? '✔️ Active' : '❌ Inactive'}</span>
								</div>
							</div>
							<div class="detail-view-actions">
								<button disabled>✏️ Edit (Unavailable)</button>
								<button class="danger" on:click={() => deleteStrategyConfirm(strat.strategyId)}>🗑️ Delete</button>
								<button class="secondary" on:click={() => { viewedStrategyId.set(null); detailViewError.set(null); }}>← Back to List</button>
							</div>
					</div>
				</div>
			{/if}
	{:else if strat && strat.spec}
		<div class="detail-view-container">
			{#if $detailViewError}
				<div class="error-message">Warning: {$detailViewError}</div>
			{/if}

			<div class="detail-view-header">
				<div class="detail-view-title">
					<h2>{formatName(strat.name)}</h2>
					<div class="detail-view-meta">
						<span>Version: {strat.version ?? 'N/A'}</span> |
						<span>Created: {strat.createdAt ? new Date(strat.createdAt).toLocaleString() : 'N/A'}</span> |
						<span class="alert-status">Alert: {strat.isAlertActive ? '✔️ Active' : '❌ Inactive'}</span>
					</div>
				</div>
				<div class="detail-view-actions">
					{#if $loading}
						<button disabled>✏️ Edit (Loading...)</button>
					{:else}
						<button on:click={() => startEdit(strat.strategyId)} disabled={$loadingSpec || !!$detailViewError || strat.spec.features.some(f=>f.exprError)}>✏️ Edit</button>
					{/if}
					<button class="danger" on:click={() => deleteStrategyConfirm(strat.strategyId)} disabled={$loading || $loadingSpec}>🗑️ Delete</button>
					<button class="secondary" on:click={() => { viewedStrategyId.set(null); detailViewError.set(null); }} disabled={$loading || $loadingSpec}>← Back to List</button>
				</div>
			</div>

			<div class="detail-section-card">
				<h3>Universe</h3>
				<dl class="definition-list">
					<dt>Timeframe</dt><dd>{formatTimeframe(strat.spec.universe.timeframe)}</dd>
					<dt>Extended Hours</dt><dd>{strat.spec.universe.extendedHours ? 'Yes' : 'No'}</dd>
					{#if strat.spec.universe.startTime || strat.spec.universe.endTime}
						<dt>Intraday Time</dt>
						<dd>{strat.spec.universe.startTime ?? 'Market Open'} - {strat.spec.universe.endTime ?? 'Market Close'}</dd>
					{/if}
				</dl>
				{#if strat.spec.universe.filters.length > 0}
					<h4 class="subsection-title">Filters:</h4>
					<ul class="detail-list">
						{#each strat.spec.universe.filters as uFilter}
							<li>{formatUniverseFilter(uFilter)}</li>
						{/each}
					</ul>
				{:else}
					<p class="no-items-note"><em>No specific universe filters applied.</em></p>
				{/if}
			</div>

			<div class="detail-section-card">
				<h3>Features <span class="count-badge">{strat.spec.features.length}</span></h3>
				{#if strat.spec.features.length > 0}
					<ul class="detail-list feature-list">
					{#each strat.spec.features as feature (feature.featureId)}
						<li class="detail-list-item">
							<strong class="feature-name">{formatName(feature.name)}</strong> (ID: {feature.featureId})
							<div class="feature-details">
								<span>Output: {feature.output}</span>
								<span>Window: {feature.window}</span>
								<span>Source: {feature.source.field}={feature.source.value}</span>
							</div>
							<div class="feature-expr">Expr: <code>{formatExprInfix(feature.expr)}</code></div>
							{#if feature.exprError}
								<small class="error-text feature-expr-error">Error: {feature.exprError}</small>
							{/if}
						</li>
					{/each}
					</ul>
				{:else}
					<p class="warning-text">No features defined (strategy may not function).</p>
				{/if}
			</div>

			<div class="detail-section-card">
				<h3>Filters <span class="count-badge">{strat.spec.filters.length}</span></h3>
				{#if strat.spec.filters.length > 0}
					<ul class="detail-list filter-list">
					{#each strat.spec.filters as filter, i (i)}
						<li class="detail-list-item">
							<strong class="filter-name">{formatName(filter.name) || 'Unnamed Filter'}</strong>
							<div class="filter-condition">
								<code>{formatFilterCondition(filter, strat.spec.features)}</code>
							</div>
						</li>
					{/each}
					</ul>
				{:else}
					<p class="no-items-note"><em>No filters defined - strategy will likely return all securities from the universe.</em></p>
				{/if}
			</div>

			<div class="detail-section-card">
				<h3>Sort By</h3>
				{#if strat.spec.sortBy && strat.spec.features.some(f => f.featureId === strat.spec.sortBy.feature)}
					{@const sortFeature = strat.spec.features.find(f => f.featureId === strat.spec.sortBy.feature)}
					<p>Feature: <strong>{formatName(sortFeature?.name)}</strong> (ID: {sortFeature?.featureId}) <br/> Direction: <strong>{strat.spec.sortBy.direction.toUpperCase()}</strong></p>
				{:else}
					<p class="warning-text"><em>Sort feature (ID: {strat.spec.sortBy?.feature}) not found or invalid.</em></p>
				{/if}
			</div>

		</div>
	{:else if !strat && !$loadingSpec}
		<div class="error-container">
				<p>Strategy with ID {$viewedStrategyId} not found.</p>
				<button class="secondary" on:click={() => { viewedStrategyId.set(null); detailViewError.set(null); }}>← Back to List</button>
			</div>
	{/if}


{:else if $editedStrategy}
	{#if $loading}<div class="loading-overlay">Preparing editor...</div>{/if}
	{#if $detailViewError}<div class="error-message">Error preparing editor: {$detailViewError}</div>{/if}

	<div class="form-block">
		<label for="strategy-name">Strategy Name</label>
		<input id="strategy-name" type="text" placeholder="e.g., Daily Momentum Breakout" bind:value={$editedStrategy.name} />
	</div>

	<fieldset class="section">
		<legend>Universe Definition</legend>
		<div class="layout-grid cols-3 items-end">
			<label>
				Timeframe <span class="help-icon" title={helpText.universeTimeframe}>?</span>
				<select bind:value={$editedStrategy.spec.universe.timeframe}>
					{#each timeframeOptions as tf} <option value={tf.value}>{tf.label}</option> {/each}
				</select>
			</label>
			<label class="inline-label">
				<input type="checkbox" bind:checked={$editedStrategy.spec.universe.extendedHours} disabled={$editedStrategy.spec.universe.timeframe !== '1'} />
				Extended Hours? <span class="help-icon" title={helpText.universeExtendedHours}>?</span>
			</label>
			{#if $editedStrategy.spec.universe.timeframe !== '1' && $editedStrategy.spec.universe.extendedHours}
				<p class="hint">(Will be ignored unless timeframe is 1-min)</p>
			{/if}
		</div>
		<div class="layout-grid cols-2">
			<label> Intraday Start Time (Optional)
				<input type="time" bind:value={$editedStrategy.spec.universe.startTime} />
			</label>
			<label> Intraday End Time (Optional)
				<input type="time" bind:value={$editedStrategy.spec.universe.endTime} />
			</label>
		</div>
		<div class="subsection">
			<h4>Security Filters <span class="help-icon" title={helpText.universeFilters}>?</span></h4>
			{#each $editedStrategy.spec.universe.filters as uFilter, uIndex (uIndex)}
				<div class="universe-filter-row">
					<div class="universe-filter-header">
						<select bind:value={uFilter.securityFeature}>
							{#each securityFeatureOptions as sf} <option value={sf}>{sf}</option> {/each}
						</select>
						<button type="button" class="danger-text" on:click={() => removeUniverseFilter(uIndex)}>✕ Remove</button>
					</div>
					{#if uFilter.securityFeature === 'Ticker' || uFilter.securityFeature === 'SecurityId'}
						<div class="layout-grid cols-2">
							<div class="pill-group">
								<h5>Include Tickers <span class="help-icon" title={helpText.universeTickerInclude}>?</span></h5>
								{#each uFilter.include as ticker, i (ticker)} <button type="button" class="pill" on:click={() => removeUniverseInclude(uIndex, i)}>{ticker} ✕</button> {/each}
								<input class="small" placeholder="Add Ticker (Enter)" on:keydown={(e) => { if (e.key === 'Enter' && e.currentTarget.value.trim()) { addUniverseInclude(uIndex, e.currentTarget.value); e.currentTarget.value = ''; e.preventDefault(); } }} />
							</div>
							<div class="pill-group">
								<h5>Exclude Tickers <span class="help-icon" title={helpText.universeTickerExclude}>?</span></h5>
								{#each uFilter.exclude as ticker, i (ticker)} <button type="button" class="pill" on:click={() => removeUniverseExclude(uIndex, i)}>{ticker} ✕</button> {/each}
								<input class="small" placeholder="Add Ticker (Enter)" on:keydown={(e) => { if (e.key === 'Enter' && e.currentTarget.value.trim()) { addUniverseExclude(uIndex, e.currentTarget.value); e.currentTarget.value = ''; e.preventDefault(); } }} />
							</div>
						</div>
					{:else}
						<div class="layout-grid cols-2">
							<label>Include Values
								<input type="text" bind:value={uFilter.include[0]} placeholder="Comma-separated values" title="Enter comma-separated values for {uFilter.securityFeature} include" />
							</label>
							<label>Exclude Values
								<input type="text" bind:value={uFilter.exclude[0]} placeholder="Comma-separated values" title="Enter comma-separated values for {uFilter.securityFeature} exclude" />
							</label>
						</div>
						<p class="hint">Enter comma-separated values for Include/Exclude.</p>
					{/if}
				</div>
			{/each}
			<button type="button" on:click={addUniverseFilter}>＋ Add Universe Filter</button>
		</div>
	</fieldset>

	<fieldset class="section">
		<legend>Features <span class="help-icon" title={helpText.features}>?</span></legend>
		<p class="help-text">{helpText.featureExpr}</p>
		{#each $editedStrategy.spec.features as feature, fIndex (feature.featureId)}
			<div class="feature-row">
				<div class="layout-grid cols-3 items-center">
					<label>Name <input type="text" bind:value={feature.name} placeholder="e.g., daily_range" /></label>
					<label>Output Type
						<select bind:value={feature.output}>
							{#each outputOptions as o} <option value={o}>{o}</option> {/each}
						</select>
					</label>
					<div class="feature-id-remove">
						<span class="feature-id-display">ID: {feature.featureId}</span>
						{#if $editedStrategy.spec.features.length > 1}
							<button type="button" class="danger-text" on:click={() => removeFeature(fIndex)}>✕ Remove</button>
						{/if}
					</div>
				</div>
				<div class="layout-grid cols-3">
					<label>Window <span class="help-icon" title={helpText.featureWindow}>?</span>
						<input type="number" min="1" step="1" bind:value={feature.window} />
					</label>
					<label>Source Field
						<select bind:value={feature.source.field}>
							{#each securityFeatureOptions as sf} <option value={sf}>{sf}</option> {/each}
						</select>
					</label>
					<label>Source Value
						<input type="text" bind:value={feature.source.value} placeholder='"relative" or specific value' />
					</label>
				</div>

				<div class="expr-builder">
					<label class="expr-label" for={`infix-expr-${feature.featureId}`}>Expression <span class="help-icon" title={helpText.featureExpr}>?</span></label>
					<input
						type="text"
						id={`infix-expr-${feature.featureId}`}
						class:invalid={feature.exprError}
						placeholder="(high - low[1]) / 2"
						bind:value={feature.infixExpr}
						on:blur={() => handleInfixParse(fIndex)}
						list="expr-suggestions"
					/>
						{#if feature.exprError}
							<small class="error-text expr-error-message">{feature.exprError}</small>
						{/if}
					<datalist id="expr-suggestions">
						{#each baseColumnOptions as col}
							<option value={col}></option>
							<option value={`${col}[1]`}></option> {/each}
						{#each operatorChars as op}<option value={op}></option>{/each}
						<option value="("></option>
						<option value=")"></option>
					</datalist>
					<p class="hint">Available columns: {baseColumnOptions.join(', ')}. Use `col[N]` for offset N. Operators: {operatorChars.join(', ')}. Use parentheses ().</p>
				</div>
			</div>
		{/each}
		<button type="button" on:click={addFeature}>＋ Add Feature</button>
	</fieldset>

	<fieldset class="section">
		<legend>Filters (Conditions) <span class="help-icon" title={helpText.filters}>?</span></legend>
		<p class="help-text">Define conditions comparing Features to constants or other Features.</p>
		{#if $availableFeatures.length === 0}
			<p class="warning-text">You need to define at least one Feature before adding Filters.</p>
		{:else}
			{#each $editedStrategy.spec.filters as filter, fIndex (fIndex)}
				<div class="filter-row">
					<span class="filter-label">IF</span>
					<select bind:value={filter.lhs} title="Left Hand Side Feature">
						{#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each}
					</select>
					<select bind:value={filter.operator} title="Comparison Operator">
						{#each operatorOptions as op} <option value={op}>{op}</option> {/each}
					</select>
					<div class="rhs-group">
						<select bind:value={filter.rhs.featureId} title="Right Hand Side Feature (0 for Constant)">
							<option value={0}>Constant Value</option>
							{#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each}
						</select>
						{#if filter.rhs.featureId === 0}
							<input class="small" type="number" step="any" bind:value={filter.rhs.const} title="Constant Value"/>
						{/if}
						<span class="scale-label">Scale:</span>
						<input class="tiny" type="number" step="any" bind:value={filter.rhs.scale} title="Scale Factor (applied to RHS)" />
					</div>
					<button type="button" class="danger-text" on:click={() => removeFilter(fIndex)}>✕</button>
					<label class="filter-name-label">Name (Optional):
						<input type="text" class="small" bind:value={filter.name} placeholder="e.g., Price above average" />
					</label>
				</div>
			{/each}
			<button type="button" on:click={addFilter} disabled={$availableFeatures.length === 0}>＋ Add Filter</button>
		{/if}
	</fieldset>

	<fieldset class="section">
		<legend>Sort Results By <span class="help-icon" title={helpText.sortBy}>?</span></legend>
		{#if $availableFeatures.length === 0}
			<p class="warning-text">You need to define at least one Feature before setting Sort criteria.</p>
		{:else}
			<div class="layout-grid cols-2">
				<label>Feature to Sort By
					<select bind:value={$editedStrategy.spec.sortBy.feature}>
						{#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each}
					</select>
				</label>
				<label>Direction
					<select bind:value={$editedStrategy.spec.sortBy.direction}>
						{#each sortDirectionOptions as dir} <option value={dir}>{dir.toUpperCase()}</option> {/each}
					</select>
				</label>
			</div>
		{/if}
	</fieldset>

	<div class="actions">
		<button class="primary" on:click={saveStrategy} disabled={$loading || $loadingSpec}>💾 Save Strategy</button>
		<button type="button" on:click={cancelEdit} disabled={$loading || $loadingSpec}>Cancel</button>
		{#if typeof $editedStrategy.strategyId === 'number'}
			<button type="button" class="danger" on:click={() => deleteStrategyConfirm($editedStrategy.strategyId)} disabled={$loading || $loadingSpec}>Delete Strategy</button>
		{/if}
			{#if $loading}<span class="loading-indicator"> Saving...</span>{/if}
	</div>
{/if}


<style>
	/* --- Base & General Styles --- */
	:global(body) { background-color: var(--ui-bg-primary, #f4f7f9); color: var(--text-primary, #333); font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji"; font-size: 15px; line-height: 1.6; }
	input, select { background: var(--ui-bg-element, #fff); color: var(--text-primary, #333); border: 1px solid var(--ui-border, #d1d9e0); padding: 0.5rem 0.75rem; border-radius: 6px; width: 100%; box-sizing: border-box; margin-bottom: 0.5rem; font-size: 0.9rem; transition: border-color 0.2s ease, box-shadow 0.2s ease; }
	input:focus, select:focus { border-color: var(--accent-blue, #007bff); box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.2); outline: none; }
	input[type="checkbox"] { width: auto; margin-right: 0.5rem; vertical-align: middle; }
	input:disabled { background-color: var(--ui-bg-disabled, #e9ecef); cursor: not-allowed; }
	input.small { font-size: 0.8rem; padding: 0.25rem 0.5rem; }
	input.tiny { font-size: 0.75rem; padding: 0.1rem 0.3rem; width: 55px; }
	/* Style for invalid input */
	input.invalid { border-color: var(--color-down, #dc3545); background-color: var(--accent-red-light, #f8d7da); }
	input.invalid:focus { border-color: var(--color-down-dark, #a71d2a); box-shadow: 0 0 0 2px rgba(220, 53, 69, 0.25); }

	label { display: block; font-weight: 500; margin-bottom: 0.25rem; font-size: 0.85rem; color: var(--text-secondary, #555); }
	label.inline-label { display: inline-flex; align-items: center; margin-bottom: 0.5rem; }
	button { background: var(--ui-bg-element, #fff); color: var(--text-primary, #333); border: 1px solid var(--ui-border, #d1d9e0); padding: 0.5rem 1.1rem; border-radius: 6px; cursor: pointer; transition: all 0.2s ease; font-size: 0.9rem; vertical-align: middle; }
	button:hover { background: var(--ui-bg-hover, #f0f3f6); border-color: var(--ui-border-hover, #b8c4cf); }
	button:active { transform: translateY(1px); }
	button:disabled { opacity: 0.6; cursor: not-allowed; }
	button.primary { background-color: var(--accent-blue, #007bff); color: #fff; border-color: var(--accent-blue, #007bff); font-weight: 500; }
	button.primary:hover { background-color: var(--accent-blue-dark, #0056b3); border-color: var(--accent-blue-dark, #0056b3); }
	button.secondary { background-color: var(--ui-bg-secondary, #6c757d); color: #fff; border-color: var(--ui-bg-secondary, #6c757d); }
	button.secondary:hover { background-color: #5a6268; border-color: #545b62; }
	button.danger { color: var(--color-down, #dc3545); border-color: var(--color-down, #dc3545); background-color: transparent; }
	button.danger:hover { background: rgba(220, 53, 69, 0.05); color: var(--color-down-dark, #a71d2a); border-color: var(--color-down-dark, #a71d2a); }
	button.danger-text { background: none; border: none; color: var(--color-down, #dc3545); padding: 0.25rem; margin-left: 0.5rem; font-size: 0.85rem; cursor: pointer; vertical-align: middle; }
	button.danger-text:hover { color: var(--color-down-dark, #a71d2a); text-decoration: underline; }
	code { font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace; background-color: var(--ui-bg-code, #e3eaf0); color: var(--text-code, #212529); padding: 0.1em 0.4em; border-radius: 4px; font-size: 0.9em; } /* Added text color */

	/* --- Layout & Sections --- */
	.layout-grid { display: grid; gap: 1rem; margin-bottom: 1rem; }
	.layout-grid.cols-2 { grid-template-columns: repeat(2, 1fr); }
	.layout-grid.cols-3 { grid-template-columns: repeat(3, 1fr); }
	.layout-grid.items-center { align-items: center; }
	.layout-grid.items-end { align-items: flex-end; }
	fieldset.section { border: 1px solid var(--ui-border, #d1d9e0); background: var(--ui-bg-element, #fff); border-radius: 8px; padding: 1.25rem 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
	legend { font-weight: 600; font-size: 1.1rem; color: var(--text-primary, #333); padding: 0 0.5rem; margin-bottom: 1rem; border-bottom: 1px solid var(--ui-border-light, #e9ecef); display: inline-block; }
	.subsection { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid var(--ui-border-light, #e9ecef); }
	.subsection h4 { font-weight: 600; margin-bottom: 0.75rem; font-size: 1rem; }
	.subsection h5 { font-weight: 500; font-size: 0.8rem; margin-bottom: 0.25rem; color: var(--text-secondary); }
	.form-block { margin-bottom: 1rem; }
	.actions { display: flex; gap: 1rem; margin-top: 2rem; padding-top: 1rem; border-top: 1px solid var(--ui-border-light, #e9ecef); align-items: center;}
	.actions button { padding: 0.7rem 1.4rem; font-weight: 500; }
	.help-icon { display: inline-flex; align-items: center; justify-content: center; width: 15px; height: 15px; border-radius: 50%; background: var(--text-secondary, #aaa); color: #fff; font-size: 10px; font-weight: bold; cursor: help; margin-left: 5px; vertical-align: middle; }
	.help-text { font-size: 0.85rem; color: var(--text-secondary, #666); margin-bottom: 1rem; margin-top: -0.75rem; }
	.hint { font-size: 0.75rem; color: var(--text-secondary, #777); font-style: italic; margin-top: 0.25rem; }
	.warning-text { color: var(--accent-orange, #fd7e14); font-size: 0.85rem; font-weight: 500; }
	.no-items-note { font-style: italic; color: var(--text-secondary); font-size: 0.9rem; margin-top: 0.5rem; }
	.loading-container { padding: 2rem; text-align: center; color: var(--text-secondary); }
	.loading-indicator { font-style: italic; color: var(--text-secondary); margin-left: 1rem; }
	.loading-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(255,255,255,0.7); display: flex; align-items: center; justify-content: center; z-index: 100; font-size: 1.2rem; color: var(--text-primary); }
	/* General error message style */
	.error-message { color: var(--color-down-dark, #721c24); background-color: var(--accent-red-light, #f8d7da); border: 1px solid var(--accent-red, #dc3545); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.9rem; }
	/* Specific error text style (e.g., for inline errors) */
		small.error-text { display: block; color: var(--color-down, #dc3545); font-size: 0.75rem; margin-top: 0.1rem; margin-bottom: 0.25rem; }
		.feature-expr-error { margin-top: 0.25rem; }
		.expr-error-message { margin-top: 0.1rem; font-size: 0.8rem; }

	.error-container { padding: 1rem; border: 1px solid var(--color-down, #dc3545); background: var(--accent-red-light, #f8d7da); border-radius: 8px; margin-bottom: 1rem; }
	.error-container h2 { color: var(--color-down-dark, #721c24); margin-top: 0; }
	.error-container .error-message { /* Use general style */ }
	.error-container button { margin-right: 0.5rem; }


	/* --- List View --- */
	.toolbar { margin-bottom: 1rem; display: flex; align-items: center; gap: 1rem; }
	.table-container { overflow-x: auto; border: 1px solid var(--ui-border, #d1d9e0); border-radius: 8px; background-color: var(--ui-bg-element, #fff); }
	table { width: 100%; border-collapse: collapse; }
	th, td { padding: 0.8rem 1rem; border-bottom: 1px solid var(--ui-border, #d1d9e0); text-align: left; vertical-align: middle; font-size: 0.85rem; }
	th { background-color: var(--ui-bg-secondary, #f8f9fa); font-weight: 600; color: var(--text-secondary, #555); border-top: none; border-bottom-width: 2px; }
	tbody tr { transition: background-color 0.15s ease-in-out; }
	tbody tr:last-child td { border-bottom: none; }
	tbody tr.clickable-row { cursor: pointer; }
	tbody tr.clickable-row:hover { background-color: var(--ui-bg-hover, #eef2f5); }
	.alert-status { font-weight: 500; white-space: nowrap; }

	/* --- Detail View --- */
	.detail-view-container { padding: 1rem; }
	.detail-view-container.partial-info { opacity: 0.7; filter: grayscale(50%); }
	.detail-view-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid var(--ui-border-light, #e9ecef); flex-wrap: wrap; gap: 0.5rem; }
	.detail-view-title h2 { margin: 0 0 0.25rem 0; font-size: 1.8rem; font-weight: 600; line-height: 1.2; color: var(--text-primary); }
	.detail-view-meta { font-size: 0.8rem; color: var(--text-secondary, #666); display: flex; flex-wrap: wrap; gap: 0 0.75rem; }
	.detail-view-meta span { white-space: nowrap; }
	.detail-view-actions { display: flex; gap: 0.5rem; flex-shrink: 0; align-self: flex-start; }
	.detail-section-card { background-color: var(--ui-bg-element, #fff); border: 1px solid var(--ui-border-light, #e3eaf0); border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1.25rem; box-shadow: 0 1px 2px rgba(0,0,0,0.03); }
	.detail-section-card h3 { font-size: 1.15rem; font-weight: 600; margin: 0 0 0.75rem 0; color: var(--text-primary); display: flex; align-items: center; justify-content: space-between; }
	.count-badge { font-size: 0.8rem; font-weight: normal; background-color: var(--ui-bg-secondary, #e9ecef); color: var(--text-secondary, #555); padding: 0.15rem 0.5rem; border-radius: 10px; }
	.definition-list { display: grid; grid-template-columns: auto 1fr; gap: 0.3rem 1rem; font-size: 0.9rem; }
	.definition-list dt { font-weight: 500; color: var(--text-secondary, #555); text-align: right; }
	.definition-list dd { margin: 0; color: var(--text-primary); }
	h4.subsection-title { font-size: 0.9rem; font-weight: 600; color: var(--text-secondary, #555); margin: 1rem 0 0.5rem 0; padding-bottom: 0.25rem; border-bottom: 1px dashed var(--ui-border-light, #e9ecef); }
	.detail-list { list-style: none; padding: 0; margin: 0.5rem 0 0 0; }
	.detail-list-item { padding: 0.75rem 0; border-bottom: 1px solid var(--ui-border-light, #e9ecef); font-size: 0.9rem; }
	.detail-list-item:last-child { border-bottom: none; padding-bottom: 0; }
	.detail-list-item strong { color: var(--text-primary); font-weight: 500; }
	.feature-list .detail-list-item { padding: 0.6rem 0; }
	.feature-name { display: block; margin-bottom: 0.25rem; font-size: 1rem; }
	.feature-details { display: flex; gap: 1rem; font-size: 0.8rem; color: var(--text-secondary); margin-bottom: 0.25rem; flex-wrap: wrap; }
	.feature-expr { font-size: 0.85rem; margin-top: 0.25rem; }
	/* Ensure code block in detail view uses default code style */
	.feature-expr code {
		/* Removed background-color: transparent and padding: 0 */
		/* Inherits background, color, padding from global 'code' style */
		white-space: normal; /* Allow wrapping */
		word-break: break-word; /* Break long words if needed */
	}
	.filter-name { display: block; margin-bottom: 0.25rem; }
	.filter-condition code { display: block; padding: 0.5rem; border-radius: 4px; white-space: normal; } /* Keep this code block styled */

	/* --- Edit View Specific --- */
	.pill-group { margin-bottom: 0.5rem; }
	.pill-group input.small { margin-top: 0.5rem; }
	.pill { background: var(--ui-bg-hover, #e9ecef); color: var(--text-primary, #333); display: inline-block; padding: 0.25rem 0.75rem; border-radius: 16px; margin: 0.25rem 0.25rem 0.25rem 0; cursor: pointer; font-size: 0.8rem; border: 1px solid var(--ui-border-light, #dee2e6); transition: background-color 0.15s ease-in-out; }
	.pill:hover { background: var(--accent-red-light, #f8d7da); border-color: var(--accent-red, #dc3545); color: var(--accent-red-dark, #721c24); }
	.universe-filter-row { border: 1px solid var(--ui-border-light, #e9ecef); padding: 1rem; margin-bottom: 1rem; border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
	.universe-filter-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
	.universe-filter-header select { flex-grow: 1; margin-right: 1rem; margin-bottom: 0; }
	.feature-row { border: 1px solid var(--ui-border-light, #e9ecef); padding: 1rem; margin-bottom: 1rem; border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
	.feature-id-remove { text-align: right; display: flex; align-items: center; justify-content: flex-end; gap: 0.5rem;}
	.feature-id-display { font-size: 0.8rem; color: var(--text-secondary); }
	.expr-builder { margin-top: 1rem; }
	.expr-label { font-weight: 500; display: block; margin-bottom: 0.25rem; }

	.filter-row { display: grid; grid-template-columns: auto minmax(120px, 1fr) auto minmax(180px, 1.5fr) auto; gap: 0.75rem; align-items: center; margin-bottom: 1rem; padding: 0.75rem; border: 1px solid var(--ui-border-light, #e9ecef); border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
	.filter-label { font-size: 0.85rem; font-weight: bold; margin-bottom: 0; }
	.filter-row select, .filter-row .rhs-group { margin-bottom: 0; }
	.rhs-group { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
	.rhs-group select { flex-grow: 1; min-width: 100px; }
	.rhs-group input { flex-shrink: 0; }
	.scale-label { font-size: 0.75rem; color: var(--text-secondary, #666); white-space: nowrap; }
	.filter-name-label { grid-column: 1 / -1; margin-top: 0.5rem; font-size: 0.75rem; font-weight: normal; display: flex; align-items: center; gap: 0.5rem; }
	.filter-name-label input.small { flex-grow: 1; margin-bottom: 0; }

</style>
