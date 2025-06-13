	import { privateRequest } from "$lib/utils/helpers/backend";
	import type { Instance } from "$lib/utils/types/types";
	import { marked } from "marked";
	import { queryChart } from "$lib/features/chart/interface";


	// Function to parse markdown content and make links open in new tabs
	export function parseMarkdown(content: string): string {
		try {
			// Format ISO 8601 timestamps like 2025-04-08T21:36:28Z to a more readable format
			const isoTimestampRegex = /\b(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z)\b/g;
			let processedContent = content.replace(isoTimestampRegex, (match) => {
				try {
					const date = new Date(match);
					if (!isNaN(date.getTime())) {
						return date.toLocaleString();
					}
					return match;
				} catch (e) {
					return match;
				}
			});

			// 3. Handle the Promise case by converting immediately to string after markdown parsing
			const parsed = marked.parse(processedContent); // marked.parse will treat our buttons as HTML
			const parsedString = typeof parsed === 'string' ? parsed : String(parsed);

			// 4. Regex to find $$$TICKER-TIMESTAMPINMS$$$ patterns
			// Captures TICKER (1), TIMESTAMPINMS (2)
			// This runs *after* marked.parse and after simple tickers are converted.
			const tickerRegex = /\$\$\$([A-Z]{1,5})-(\d+)\$\$\$/g;

			const contentWithTickerButtons = parsedString.replace(
				tickerRegex,
				(match, ticker, timestampMs) => {
					const formattedDate = formatChipDate(parseInt(timestampMs, 10));
					const buttonText = `${ticker}${formattedDate}`;
					return `<button class="ticker-button glass glass--small glass--responsive" data-ticker="${ticker}" data-timestamp-ms="${timestampMs}">${buttonText}</button>`;
				}
			);

			// Add target="_blank" and rel="noopener noreferrer" to all standard links, plus white color styling and hover events
			// Ensure this doesn't interfere with the buttons (it shouldn't as buttons aren't <a> tags)
			const withExternalLinks = contentWithTickerButtons.replace(
				/<a\s+(?:[^>]*?\s+)?href="([^"]*)"(?:\s+[^>]*?)?>/g,
				'<a href="$1" target="_blank" rel="noopener noreferrer" style="color: white !important; text-decoration: none; transition: all 0.2s ease;" onmouseenter="this.style.color=\'#3b82f6\'; this.style.backgroundColor=\'rgba(59, 130, 246, 0.1)\'; this.style.borderRadius=\'4px\';" onmouseleave="this.style.color=\'white\'; this.style.backgroundColor=\'transparent\'; this.style.borderRadius=\'0\';">'
			);

			return withExternalLinks;
		} catch (error) {
			console.error('Error parsing markdown:', error);
			return content; // Fallback to plain text if parsing fails
		}
	}
	// Format timestamp for context chip (matches parseMarkdown format)
	export function formatChipDate(timestampMs?: number): string {
		if (!timestampMs || timestampMs === 0) {
			return '';
		}
		try {
			const date = new Date(timestampMs);
			const year = date.getFullYear();
			const month = (date.getMonth() + 1).toString().padStart(2, '0');
			const day = date.getDate().toString().padStart(2, '0');
			return ` (${year}-${month}-${day})`; // Add leading space
		} catch (e) {
			console.error('Error formatting chip date:', e);
			return '';
		}
	}
	// Runtime calculation function
	export function formatRuntime(startTime: Date, endTime: Date): string {
		const diffMs = endTime.getTime() - startTime.getTime();
		const seconds = Math.floor(diffMs / 1000);
		
		if (seconds < 60) {
			return `Thought for ${seconds}s`;
		} else {
			const minutes = Math.floor(seconds / 60);
			const remainingSeconds = seconds % 60;
			return `Thought for ${minutes}m ${remainingSeconds}s`;
		}
	}
	// Function
	export function cleanHtmlContent(htmlContent: string): string {
		if (!htmlContent) return '';
		
		// First, handle the original $$$ ticker patterns before they're converted to buttons
		// Pattern: $$$TICKER-TIMESTAMPINMS$$$
		const dollarTickerRegex = /\$\$\$([A-Z]{1,5})-(\d+)\$\$\$/g;
		let processedContent = htmlContent.replace(dollarTickerRegex, '$1');
		
		// Create a temporary DOM element to parse HTML
		const tempDiv = document.createElement('div');
		tempDiv.innerHTML = processedContent;
		
		// Find all ticker buttons and replace them with just the ticker symbol
		const tickerButtons = tempDiv.querySelectorAll('button.ticker-button[data-ticker]');
		tickerButtons.forEach(button => {
			const ticker = button.getAttribute('data-ticker');
			if (ticker) {
				button.replaceWith(document.createTextNode(ticker));
			}
		});
		
		// Return the text content (automatically strips all HTML tags)
		return tempDiv.textContent || '';
	}
	// Function to handle clicks on ticker buttons
	export async function handleTickerButtonClick(event: MouseEvent) {
		const target = event.target as HTMLButtonElement; // Assert as Button Element
		if (target && target.classList.contains('ticker-button')) {
			const ticker = target.dataset.ticker;
			const timestampMsStr = target.dataset.timestampMs; // Get the timestamp string

			if (ticker && timestampMsStr) {
				const timestampMs = parseInt(timestampMsStr, 10); // Parse the timestamp

				if (isNaN(timestampMs)) {
					console.error('Invalid timestampMs on ticker button');
					return; // Don't proceed if timestamp is invalid
				}

				try {
					target.disabled = true;

					// Call the new backend function to get the securityId
					// Define expected response shape
					type SecurityIdResponse = { securityId?: number };
					console.log('ticker', ticker);
					console.log('timestampMs', timestampMs);
					const response = await privateRequest<SecurityIdResponse>('getSecurityIDFromTickerTimestamp', {
						ticker: ticker,
						timestampMs: timestampMs // Pass timestamp as number
					});

					// Safely access the securityId
					const securityId = response?.securityId;

					if (securityId && !isNaN(securityId)) {
						// If securityId is valid, query the chart
						queryChart({
							ticker: ticker,
							securityId: securityId,
							timestamp: timestampMs // Pass timestamp as number (milliseconds)
						} as Instance);
					} else {
						console.error('Failed to retrieve a valid securityId from backend:', response);
						// Handle error visually if needed (e.g., show error message)
						target.textContent = 'Error'; // Revert button text or indicate error
						await new Promise(resolve => setTimeout(resolve, 2000)); // Wait 2 seconds
					}

				} catch (error) {
					console.error('Error fetching securityId:', error);
					await new Promise(resolve => setTimeout(resolve, 2000)); // Wait 2 seconds
				} finally {
					target.disabled = false;
				}

			} else {
				console.error('Missing data attributes on ticker button');
			}
		}
	}
