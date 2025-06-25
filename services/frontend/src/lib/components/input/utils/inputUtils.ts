import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
import { allKeys, type InstanceAttributes } from './inputTypes';
export { capitalize, formatTimeframe, detectInputTypeSync };
import { type Instance } from '$lib/utils/types/types';
import { parse } from 'date-fns';
let isLoadingSecurities = false;

function capitalize(str: string, lower = false): string {
	return (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, (match: string) =>
		match.toUpperCase()
	);
}

function formatTimeframe(timeframe: string): string {
	const match = timeframe.match(/^(\d+)([dwmsh]?)$/i) ?? null;
	let result = timeframe;
	if (match) {
		switch (match[2]) {
			case 'd':
				result = `${match[1]} days`;
				break;
			case 'w':
				result = `${match[1]} weeks`;
				break;
			case 'm':
				result = `${match[1]} months`;
				break;
			case 'h':
				result = `${match[1]} hours`;
				break;
			case 's':
				result = `${match[1]} seconds`;
				break;
			default:
				result = `${match[1]} minutes`;
				break;
		}
		if (match[1] === '1') {
			result = result.slice(0, -1);
		}
	}
	return result;
}
// Add this new helper function above the existing determineInputType function
function detectInputTypeSync(
	inputString: string,
	possibleKeysArg: InstanceAttributes[] | 'any'
): string {
	// Make sure we have a valid array of possible keys
	const possibleKeys = Array.isArray(possibleKeysArg) ? possibleKeysArg : [...allKeys];

	if (!inputString || inputString === '') {
		return '';
	}

	// Test for timeframe first - if it starts with a number, it's likely a timeframe
	if (possibleKeys.includes('timeframe') && /^\d/.test(inputString)) {
		return 'timeframe';
	} else if (possibleKeys.includes('ticker') && /^[A-Z]+$/.test(inputString)) {
		return 'ticker';
	} else if (possibleKeys.includes('ticker') && /^[a-zA-Z]+$/.test(inputString)) {
		// Default to ticker for any alphabetic input if ticker is possible
		return 'ticker';
	}

	return '';
}

export async function validateInput(
	inputString: string,
	inputType: string
): Promise<{
	inputValid: boolean;
	securities: Instance[];
}> {
	if (inputType === 'ticker') {
		isLoadingSecurities = true;

		try {
			// Add a small delay to avoid too many rapid requests during typing
			await new Promise((resolve) => setTimeout(resolve, 10));

			let securities: Instance[];

			// If input string is empty, get popular tickers
			if (!inputString || inputString.trim() === '') {
				securities = await publicRequest<Instance[]>('getPopularTickers', {});
			} else {
				// Otherwise, search for tickers matching the input
				securities = await publicRequest<Instance[]>('getSecuritiesFromTicker', {
					ticker: inputString
				});
			}

			if (Array.isArray(securities) && securities.length > 0) {
				return {
					inputValid: true,
					securities: securities
				};
			}
			return { inputValid: inputString.length === 0, securities: [] }; // Empty input is valid
		} catch (error) {
			console.error('Error fetching securities:', error);
			// Return empty results but mark as valid if we have some input or empty input
			// This allows the UI to stay responsive even if backend request fails
			return {
				inputValid: true, // Consider both empty and non-empty input as valid when there's an error
				securities: []
			};
		} finally {
			isLoadingSecurities = false;
		}
	} else if (inputType === 'timeframe') {
		const regex = /^\d{1,3}[yqmwhds]?$/i;
		return { inputValid: regex.test(inputString), securities: [] };
	}
}
