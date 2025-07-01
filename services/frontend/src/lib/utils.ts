/**
 * Simple utility functions for testing
 */

/**
 * Adds two numbers together
 */
export function add(a: number, b: number): number {
	return a + b;
}

/**
 * Formats a number as currency
 */
export function formatCurrency(amount: number): string {
	return new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: 'USD'
	}).format(amount);
}
