import { add, formatCurrency } from './utils';

describe('Utility functions', () => {
	describe('add', () => {
		it('adds two positive numbers correctly', () => {
			expect(add(1, 2)).toBe(3);
		});

		it('handles negative numbers', () => {
			expect(add(-1, 1)).toBe(0);
			expect(add(-1, -1)).toBe(-2);
		});
	});

	describe('formatCurrency', () => {
		it('formats positive numbers as USD currency', () => {
			expect(formatCurrency(1234.56)).toBe('$1,234.56');
		});

		it('formats negative numbers as USD currency', () => {
			expect(formatCurrency(-1234.56)).toBe('-$1,234.56');
		});
	});
});
