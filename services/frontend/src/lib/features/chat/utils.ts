import { publicRequest } from '$lib/utils/helpers/backend';
import type { Instance } from '$lib/utils/types/types';
import { marked } from 'marked';
import { queryChart } from '$lib/features/chart/interface';
import { queryInstanceRightClick } from '$lib/components/rightClick.svelte';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore – types provided by the package at runtime; this line quiets TS until it is installed
import DOMPurify from 'isomorphic-dompurify';

// ---------------------------------------------------------------------------
// DOMPurify global configuration
// We want to allow inline hover effects on <a> tags added by parseMarkdown.
// These use `onmouseenter` / `onmouseleave` attributes to tweak colours.
// By default DOMPurify strips any `on*` attribute, so we re-allow ONLY
// these two attributes and ONLY on <a> elements.
// ---------------------------------------------------------------------------

const ALLOWED_EVENT_ATTRS = new Set(['onmouseenter', 'onmouseleave']);

// Register the hook exactly once (module scope). The hook is idempotent; if
// the file is re-imported HMR will register multiple hooks, so guard it.
// `DOMPurify.version` is constant, we can attach a flag to it.
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore attach flag on the DOMPurify constructor
if (!DOMPurify.__peripheralHookAdded) {
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	DOMPurify.addHook('uponSanitizeAttribute', (_node: any, data: any) => {
		if (data.attrName && ALLOWED_EVENT_ATTRS.has(data.attrName.toLowerCase())) {
			// Allow only on <a> elements to reduce surface area
			if (data && data.attrName && (_node as HTMLElement).nodeName === 'A') {
				// Keep the attribute as is (inline JS). Note: still reliant on our
				// own generated markup; user-supplied `onmouseenter` will be removed
				// because we never expose <a> tags with those attrs in user input.
				data.keepAttr = true;
			}
		}
	});
	// eslint-disable-next-line @typescript-eslint/ban-ts-comment
	// @ts-ignore
	DOMPurify.__peripheralHookAdded = true;
}

// Centralized ticker formatting regex pattern
const TICKER_FORMAT_REGEX = /\$\$([A-Z0-9.]{1,6})-(\d+)\$\$/g;

// Function to parse markdown content and make links open in new tabs
export function parseMarkdown(content: string): string {
	try {
		// Format ISO 8601 timestamps like 2025-04-08T21:36:28Z to a more readable format
		const isoTimestampRegex = /\b(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z)\b/g;
		const processedContent = content.replace(isoTimestampRegex, (match) => {
			try {
				const date = new Date(match);
				if (!isNaN(date.getTime())) {
					return date.toLocaleString();
				}
				return match;
			} catch {
				return match;
			}
		});

		// 3. Handle the Promise case by converting immediately to string after markdown parsing
		const parsed = marked.parse(processedContent); // marked.parse will treat our buttons as HTML
		const parsedString = typeof parsed === 'string' ? parsed : String(parsed);

		// 4. Regex to find $$TICKER-TIMESTAMPINMS$$ patterns
		// Captures TICKER (1), TIMESTAMPINMS (2)
		// This runs *after* marked.parse and after simple tickers are converted.
		const contentWithTickerButtons = parsedString.replace(
			TICKER_FORMAT_REGEX,
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

		// Strip ticker buttons from headers - replace with plain ticker text
		const withCleanHeaders = withExternalLinks.replace(
			/<(h[1-6][^>]*)>(.*?)<\/h[1-6]>/gi,
			(match, openingTag, content) => {
				// Remove ticker buttons from header content and replace with just ticker text
				const cleanContent = content.replace(
					/<button[^>]*data-ticker="([^"]*)"[^>]*>.*?<\/button>/gi,
					'$1'
				);
				return `<${openingTag}>${cleanContent}</h${openingTag.match(/h([1-6])/i)?.[1] || '1'}>`;
			}
		);

		// Strip strikethrough formatting - replace <del> and <s> tags with plain text
		const withoutStrikethrough = withCleanHeaders.replace(/<(del|s)[^>]*>(.*?)<\/(del|s)>/gi, '$2');

		// Sanitize the final HTML to mitigate XSS while preserving the custom
		// elements and attributes we purposely inject (e.g., ticker buttons).
		const cleanHtml = DOMPurify.sanitize(withoutStrikethrough, {
			// Allow standard safe-HTML elements and attributes
			USE_PROFILES: { html: true },
			// Permit our custom <button> element produced by ticker formatting
			'ADD_TAGS': ['button'],
			// Allow styling hooks and custom data attributes used throughout chat
			'ADD_ATTR': [
				'class',
				'style',
				'data-ticker',
				'data-timestamp-ms',
				'target',
				'rel',
				'onmouseenter',
				'onmouseleave'
			]
		});
		return cleanHtml;
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

		// Format the date in EST timezone
		const estDate = new Intl.DateTimeFormat('en-US', {
			timeZone: 'America/New_York',
			year: 'numeric',
			month: '2-digit',
			day: '2-digit'
		}).formatToParts(date);

		// Extract parts and format as YYYY-MM-DD
		const year = estDate.find(part => part.type === 'year')?.value || '';
		const month = estDate.find(part => part.type === 'month')?.value || '';
		const day = estDate.find(part => part.type === 'day')?.value || '';

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

	// First, handle the original $$ ticker patterns before they're converted to buttons
	// Pattern: $$TICKER-TIMESTAMPINMS$$
	const processedContent = htmlContent.replace(TICKER_FORMAT_REGEX, '$1');

	// Create a temporary DOM element to parse HTML
	const tempDiv = document.createElement('div');
	tempDiv.innerHTML = processedContent;

	// Find all ticker buttons and replace them with just the ticker symbol
	const tickerButtons = tempDiv.querySelectorAll('button.ticker-button[data-ticker]');
	tickerButtons.forEach((button) => {
		const ticker = button.getAttribute('data-ticker');
		if (ticker) {
			button.replaceWith(document.createTextNode(ticker));
		}
	});

	// Return the text content (automatically strips all HTML tags)
	return tempDiv.textContent || '';
}
// Function to clean ticker formatting from strings
export function cleanTickerFormatting(text: string): string {
	if (!text) return text;

	// Pattern: $$TICKER-TIMESTAMPINMS$$
	return text.replace(TICKER_FORMAT_REGEX, '$1');
}
// Generic function to recursively clean ticker formatting from any data structure
function cleanDataRecursively(data: unknown): unknown {
	if (!data) return data;

	if (typeof data === 'string') {
		return cleanTickerFormatting(data);
	}

	if (Array.isArray(data)) {
		return data.map((item) => cleanDataRecursively(item));
	}

	if (typeof data === 'object' && data !== null) {
		const cleaned: Record<string, unknown> = {};
		for (const [key, value] of Object.entries(data)) {
			cleaned[key] = cleanDataRecursively(value);
		}
		return cleaned;
	}

	return data;
}

// Function to clean ticker formatting from plot data
export function cleanPlotData(plotData: unknown): unknown {
	return cleanDataRecursively(plotData);
}

// Function to clean ticker formatting from content chunks (only plots)
export function cleanContentChunk(chunk: unknown): unknown {
	if (!chunk || (chunk as { type: string }).type !== 'plot') {
		return chunk;
	}

	const cleanedContent = cleanPlotData((chunk as { content: unknown }).content);

	const result = {
		...chunk,
		content: cleanedContent
	};

	return result;
}

// Helper function to create text content for copying from a content chunk
export function getContentChunkTextForCopy(
	chunk: { type: string; content: unknown },
	isTableData: (content: unknown) => boolean,
	plotDataToText: (data: unknown) => string
): string {
	if (chunk.type === 'text') {
		const content = typeof chunk.content === 'string' ? chunk.content : String(chunk.content);
		return cleanHtmlContent(content);
	} else if (chunk.type === 'table' && isTableData(chunk.content)) {
		// For tables, create a simple text representation
		const tableData = chunk.content as { caption?: string; headers: unknown[]; rows: unknown[] };
		let tableText = '';
		if (tableData.caption) {
			const cleanCaption = cleanHtmlContent(tableData.caption);
			tableText += cleanCaption + '\n\n';
		}
		// Add headers (clean ticker formatting from headers too)
		tableText +=
			tableData.headers.map((header: unknown) => cleanHtmlContent(String(header))).join('\t') + '\n';
		// Add rows
		tableText += tableData.rows
			.map((row: unknown) => {
				if (Array.isArray(row)) {
					return row.map((cell: unknown) => cleanHtmlContent(String(cell))).join('\t');
				} else {
					return cleanHtmlContent(String(row));
				}
			})
			.join('\n');
		return tableText;
	} else if (chunk.type === 'plot') {
		// For plots, clean ticker formatting and create a text representation
		const cleanedChunk = cleanContentChunk(chunk);
		return plotDataToText((cleanedChunk as { content: unknown }).content);
	}
	return '';
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
				const response = await publicRequest<SecurityIdResponse>(
					'getSecurityIDFromTickerTimestamp',
					{
						ticker: ticker,
						timestampMs: timestampMs // Pass timestamp as number
					}
				);

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
					await new Promise((resolve) => setTimeout(resolve, 2000)); // Wait 2 seconds
				}
			} catch (error) {
				console.error('Error fetching securityId:', error);
				await new Promise((resolve) => setTimeout(resolve, 2000)); // Wait 2 seconds
			} finally {
				target.disabled = false;
			}
		} else {
			console.error('Missing data attributes on ticker button');
		}
	}
}

export async function handleTickerButtonRightClick(event: MouseEvent) {
	const target = event.target as HTMLButtonElement; // Assert as Button Element
	if (target && target.classList.contains('ticker-button')) {
		event.preventDefault();
		event.stopPropagation();
		const ticker = target.dataset.ticker;
		const timestampMsStr = target.dataset.timestampMs; // Get the timestamp string

		if (ticker && timestampMsStr) {
			const timestampMs = parseInt(timestampMsStr, 10); // Parse the timestamp

			if (isNaN(timestampMs)) {
				console.error('Invalid timestampMs on ticker button for right-click:', timestampMsStr);
				return; // Don't proceed if timestamp is invalid
			}

			console.log(`Right-click on ticker button: ${ticker} at ${timestampMs}`);

			try {
				target.disabled = true;
				// We need securityId to create a complete instance for the right-click menu.
				type SecurityIdResponse = { securityId?: number };
				const response = await publicRequest<SecurityIdResponse>(
					'getSecurityIDFromTickerTimestamp',
					{
						ticker: ticker,
						timestampMs: timestampMs // Pass timestamp as number
					}
				);

				const securityId = response?.securityId;

				if (securityId && !isNaN(securityId)) {
					const instance: Instance = {
						ticker: ticker,
						securityId: securityId,
						timestamp: timestampMs // Pass timestamp as number (milliseconds)
					};

					console.log('Constructed instance for right-click:', instance);
					await queryInstanceRightClick(event, instance, 'embedded');
				} else {
					console.error(
						'Failed to retrieve a valid securityId from backend for right-click:',
						response
					);
				}
			} catch (error) {
				console.error('Error handling ticker button right-click:', error);
			} finally {
				target.disabled = false;
			}
		} else {
			console.error('Missing data attributes on ticker button for right-click');
		}
	}
}
