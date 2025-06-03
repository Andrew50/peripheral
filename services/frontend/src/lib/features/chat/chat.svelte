<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { marked } from 'marked'; // Import the markdown parser
	import { queryChart } from '$lib/features/chart/interface'; // Import queryChart
	import type { Instance } from '$lib/utils/types/types';
	import { browser } from '$app/environment'; // Import browser
	import { derived, writable } from 'svelte/store';
	import {
		inputValue,
		contextItems,
		addInstanceToChat,
		removeInstanceFromChat,
		removeFilingFromChat,
		type FilingContext, // Import the new type
		pendingChatQuery // Import the new store
	} from './interface'
	import { activeChartInstance } from '$lib/features/chart/interface';
	import { functionStatusStore, type FunctionStatusUpdate } from '$lib/utils/stream/socket'; // <-- Import the status store and FunctionStatusUpdate type

	// Conversation management types
	type ConversationSummary = {
		conversation_id: string;
		title: string;
		created_at: string;
		updated_at: string;
		last_message_query?: string;
	};

	type ConversationCreateResponse = {
		conversation_id: string;
		title: string;
	};

	// Conversation management state
	let conversations: ConversationSummary[] = [];
	let currentConversationId = '';
	let currentConversationTitle = 'Chat';
	let showConversationDropdown = false;
	let conversationDropdown: HTMLDivElement;
	let loadingConversations = false;

	// Set default options for the markdown parser (optional)
	marked.setOptions({
		breaks: true, // Adds support for GitHub-flavored markdown line breaks
		gfm: true     // GitHub-flavored markdown
	});

	// Configure marked to make links open in a new tab
	const renderer = new marked.Renderer();
	
	
	marked.setOptions({
		renderer,
		breaks: true,
		gfm: true
	});

	// Function to parse markdown content and make links open in new tabs
	function parseMarkdown(content: string): string {
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
					return `<button class="ticker-button" data-ticker="${ticker}" data-timestamp-ms="${timestampMs}">${buttonText}</button>`;
				}
			);

			// Add target="_blank" and rel="noopener noreferrer" to all standard links
			// Ensure this doesn't interfere with the buttons (it shouldn't as buttons aren't <a> tags)
			const withExternalLinks = contentWithTickerButtons.replace(
				/<a\s+(?:[^>]*?\s+)?href="([^"]*)"(?:\s+[^>]*?)?>/g,
				'<a href="$1" target="_blank" rel="noopener noreferrer">'
			);

			return withExternalLinks;
		} catch (error) {
			console.error('Error parsing markdown:', error);
			return content; // Fallback to plain text if parsing fails
		}
	}
	// Define the ContentChunk and TableData types to match the backend
	type TableData = {
		caption?: string;
		headers: string[];
		rows: any[][];
	};

	type ContentChunk = {
		type: 'text' | 'table';
		content: string | TableData;
	};

	type QueryResponse = {
		type: 'text' | 'mixed_content';
		text?: string;
		content_chunks?: ContentChunk[];
		suggestions?: string[];
		conversation_id?: string;
	};

	// Conversation history type
	type ConversationData = {
		messages: Array<{
			query: string;
			content_chunks?: ContentChunk[];
			response_text: string;
			timestamp: string | Date;
			context_items?: (Instance | FilingContext)[];
			suggested_queries?: string[];
			completed_at?: string | Date;
			status?: string;
		}>;
		timestamp: string | Date;
	};

	// Message type for chat history
	type Message = {
		id: string;
		content: string;
		sender: 'user' | 'assistant' | 'system';
		timestamp: Date;
		contentChunks?: ContentChunk[];
		responseType?: string;
		isLoading?: boolean;
		suggestedQueries?: string[];
		contextItems?: (Instance | FilingContext)[];
		status?: string;        // "pending", "completed", "error"
		completedAt?: Date;     // When the response was completed
		isNewResponse?: boolean; // Flag to indicate if this is a new unseen response
	};

	// Type for suggested queries response
	type SuggestedQueriesResponse = {
		suggestions: string[];
	};

	let queryInput: HTMLTextAreaElement;
	let isLoading = false;
	let messagesContainer: HTMLDivElement;
	let initialSuggestions: string[] = [];
	let pollInterval: ReturnType<typeof setInterval> | null = null;
	let pollAttempts = 0;
	let maxPollAttempts = 3;

	// Backtest mode state
	let isBacktestMode = false;

	// State for table expansion
	let tableExpansionStates: { [key: string]: boolean } = {};

	// State for table sorting
	type SortState = {
		columnIndex: number | null;
		direction: 'asc' | 'desc' | null;
	};
	let tableSortStates: { [key: string]: SortState } = {};

	// Chat history
	let messagesStore = writable<Message[]>([]); // Wrap messages in a writable store
	let historyLoaded = false; // Add state variable
	let isInitialLoad = true; // Track if this is the initial load

	// Derived store to control initial chip visibility
	const showChips = derived([inputValue, messagesStore], ([$val, $msgs]) => $msgs.length === 0 && $val.trim() === '');

	// Reactive variable for top 3 suggestions
	$: topChips = initialSuggestions.slice(0, 3);

	// State for showing all initial suggestions
	let showAllInitialSuggestions = false;

	// Manage active chart context: subscribe to add new and remove old chart contexts
	let previousChartInstance: Instance | null = null;

	// Add abort controller for cancelling requests
	let currentAbortController: AbortController | null = null;
	let requestCancelled = false;
	let currentProcessingQuery = ''; // Track the query currently being processed

	// Function to fetch initial suggestions based on active chart
	async function fetchInitialSuggestions() {
		initialSuggestions = []; // Clear previous suggestions first
		if ($activeChartInstance) { // Only fetch if there's an active instance
			try {
				const response = await privateRequest<{ suggestions: string[] }>('getInitialQuerySuggestions', {
					activeChartInstance: $activeChartInstance
				});
				if (response && response.suggestions) {
					initialSuggestions = response.suggestions;
				} else {
					initialSuggestions = []; // Ensure it's an empty array on bad response
				}
			} catch (error) {
				console.error('Error fetching initial suggestions:', error);
				initialSuggestions = []; // Ensure it's an empty array on error
			}
		}
	}

	// Conversation management functions
	async function loadConversations() {
		try {
			loadingConversations = true;
			const response = await privateRequest<ConversationSummary[]>('getUserConversations', {});
			conversations = response || [];
		} catch (error) {
			console.error('Error loading conversations:', error);
			conversations = [];
		} finally {
			loadingConversations = false;
		}
	}

	async function createNewConversation() {
		// Clear current conversation state - new conversation will be created when first message is sent
		currentConversationId = '';
		currentConversationTitle = 'New Chat';
		
		// Clear current chat
		messagesStore.set([]);
		
		// Clear any pending query that might be set
		pendingChatQuery.set(null);
		
		// Close dropdown
		showConversationDropdown = false;
		
		// Focus on input
		if (queryInput) {
			setTimeout(() => queryInput.focus(), 100);
		}
		
		// Clear initial suggestions and fetch new ones for empty chat
		initialSuggestions = [];
		fetchInitialSuggestions();
	}

	async function switchToConversation(conversationId: string, title: string) {
		if (conversationId === currentConversationId) {
			showConversationDropdown = false;
			return;
		}

		try {
			const response = await privateRequest('switchConversation', {
				conversation_id: conversationId
			});
			
			if (response) {
				currentConversationId = conversationId;
				currentConversationTitle = title;
				
				// Clear current messages and load the selected conversation
				messagesStore.set([]);
				await loadConversationHistory();
				
				showConversationDropdown = false;
			}
		} catch (error) {
			console.error('Error switching conversation:', error);
		}
	}

	async function deleteConversation(conversationId: string, event: MouseEvent) {
		event.stopPropagation(); // Prevent switching to the conversation
		
		if (!confirm('Are you sure you want to delete this conversation?')) {
			return;
		}

		try {
			await privateRequest('deleteConversation', {
				conversation_id: conversationId
			});
			
			// If we deleted the current conversation, start a new one
			if (conversationId === currentConversationId) {
				createNewConversation(); // This will clear the UI state
			} else {
				// Just refresh the conversation list
				await loadConversations();
			}
		} catch (error) {
			console.error('Error deleting conversation:', error);
		}
	}

	function toggleConversationDropdown() {
		showConversationDropdown = !showConversationDropdown;
		if (showConversationDropdown) {
			loadConversations();
		}
	}

	// Close dropdown when clicking outside
	function handleClickOutside(event: MouseEvent) {
		if (showConversationDropdown && conversationDropdown && !conversationDropdown.contains(event.target as Node)) {
			showConversationDropdown = false;
		}
	}

	// Load any existing conversation history from the server
	async function loadConversationHistory(shouldAutoScroll: boolean = true) {
		try {
			isLoading = true;
			const response = await privateRequest('getUserConversation', {});

			// Check if we have a valid conversation history
			const conversation = response as ConversationData;
			if (conversation && conversation.messages && conversation.messages.length > 0) {
				// Clear existing messages to avoid duplicates during polling updates
				messagesStore.set([]);
				
				// Get the last seen timestamp from localStorage
				const lastSeenKey = 'chat_last_seen_timestamp';
				const lastSeenStr = localStorage.getItem(lastSeenKey);
				const lastSeenTimestamp = lastSeenStr ? new Date(lastSeenStr) : null;
				
				// Track if we found any new responses
				let hasNewResponses = false;
				let latestCompletedTimestamp: Date | null = null;

				// Process each message in the conversation history
				conversation.messages.forEach((msg) => {
					const msgTimestamp = new Date(msg.timestamp);
					const msgCompletedAt = msg.completed_at ? new Date(msg.completed_at) : undefined;
					const isCompleted = msg.status === 'completed';
					const isPending = msg.status === 'pending';
					
					// Check if this is a new response (completed after last seen timestamp)
					const isNewResponse = isCompleted && msgCompletedAt && lastSeenTimestamp && msgCompletedAt > lastSeenTimestamp ? true : undefined;
					
					if (isNewResponse) {
						hasNewResponses = true;
					}

					// Track the latest completed timestamp
					if (isCompleted && msgCompletedAt && (!latestCompletedTimestamp || msgCompletedAt > latestCompletedTimestamp)) {
						latestCompletedTimestamp = msgCompletedAt;
					}

					// Add user message
					messagesStore.update(current => [...current, {
						id: generateId(),
						content: msg.query,
						sender: 'user',
						timestamp: msgTimestamp,
						contextItems: msg.context_items || [],
						status: msg.status,
						completedAt: msgCompletedAt,
						isNewResponse: isNewResponse
					}]);

					// Only add assistant message if it's completed (has content)
					if (isCompleted && (msg.content_chunks || msg.response_text)) {
						messagesStore.update(current => [...current, {
							id: generateId(),
							sender: 'assistant',
							content: msg.response_text || '',
							contentChunks: msg.content_chunks || [],
							timestamp: msgTimestamp,
							suggestedQueries: msg.suggested_queries || [],
							status: msg.status,
							completedAt: msgCompletedAt,
							isNewResponse: isNewResponse
						}]);
					} else if (isPending) {
						// Add a loading message for pending requests
						messagesStore.update(current => [...current, {
							id: generateId(),
							sender: 'assistant',
							content: '',
							timestamp: msgTimestamp,
							isLoading: true,
							status: 'pending'
						}]);
					}
				});

				// Update last seen timestamp if we have new responses
				if (hasNewResponses && latestCompletedTimestamp) {
					localStorage.setItem(lastSeenKey, (latestCompletedTimestamp as Date).toISOString());
				} else if (!lastSeenTimestamp && latestCompletedTimestamp) {
					// First time loading, set the timestamp
					localStorage.setItem(lastSeenKey, (latestCompletedTimestamp as Date).toISOString());
				}

				// Show notification for new responses if tab wasn't focused
				if (hasNewResponses && document.hidden) {
					// You could add a notification here
				}

				// If we were in a new conversation state (no conversation ID), 
				// we need to get the conversation details
				if (!currentConversationId && conversation.messages.length > 0) {
					try {
						// Get all conversations to find the active one
						const conversationsResponse = await privateRequest<ConversationSummary[]>('getUserConversations', {});
						const conversations = conversationsResponse || [];
						
						// The most recent conversation should be the active one
						if (conversations.length > 0) {
							const latestConversation = conversations[0]; // They're ordered by updated_at DESC
							currentConversationId = latestConversation.conversation_id;
							currentConversationTitle = latestConversation.title;
						}
					} catch (error) {
						console.error('Error getting conversation details:', error);
						// Fallback to generic title
						currentConversationTitle = 'New Chat';
					}
				}

				// Only auto-scroll if explicitly requested (e.g., on initial load)
				if (shouldAutoScroll) {
					// Position at bottom after DOM is updated
					await tick();
					if (messagesContainer) {
						messagesContainer.style.scrollBehavior = 'auto';
						messagesContainer.scrollTop = messagesContainer.scrollHeight;
						messagesContainer.style.scrollBehavior = 'smooth';
					}
				}
			} else {
				// No conversation history, clear messages
				messagesStore.set([]);
			}
		} catch (error) {
			console.error('Error loading conversation history:', error);
		} finally {
			isLoading = false;
			historyLoaded = true;

			// Clear previous suggestions and fetch if history is empty
			initialSuggestions = [];
			if ($messagesStore.length === 0) {
				fetchInitialSuggestions(); // <-- Call the new helper function
			}
		}
	}

	// Function to check for conversation updates (for polling)
	async function checkForUpdates() {
		if (isLoading) return; // Don't check if we're already loading
		if (document.hidden) return; // Don't poll when tab is not visible
		
		try {
			const response = await privateRequest('getUserConversation', {});
			const conversation = response as ConversationData;
			
			if (conversation && conversation.messages && conversation.messages.length > 0) {
				const lastSeenKey = 'chat_last_seen_timestamp';
				const lastSeenStr = localStorage.getItem(lastSeenKey);
				const lastSeenTimestamp = lastSeenStr ? new Date(lastSeenStr) : null;
				
				let hasUpdates = false;
				
				// Check if there are any new completed messages or status changes
				for (const msg of conversation.messages) {
					const msgCompletedAt = msg.completed_at ? new Date(msg.completed_at) : null;
					const isCompleted = msg.status === 'completed';
					
					if (isCompleted && msgCompletedAt && lastSeenTimestamp && msgCompletedAt > lastSeenTimestamp) {
						hasUpdates = true;
						break;
					}
					
					// Check if we have pending messages that might have been completed
					const existingMessage = $messagesStore.find(m => m.content === msg.query && m.sender === 'user');
					if (existingMessage && isCompleted && (!existingMessage.status || existingMessage.status === 'pending')) {
						hasUpdates = true;
						break;
					}
				}
				
				if (hasUpdates) {
					// Incrementally update instead of full reload when possible
					// Don't auto-scroll during polling updates to preserve user's reading position
					await loadConversationHistory(false);
				}
			}
			
			// Reset poll attempts on success
			pollAttempts = 0;
		} catch (error) {
			console.error('Error checking for updates:', error);
			pollAttempts++;
			
			// Stop polling after max attempts to avoid spamming
			if (pollAttempts >= maxPollAttempts) {
				console.warn('Stopped polling due to repeated failures');
				if (pollInterval) {
					clearInterval(pollInterval);
					pollInterval = null;
				}
			}
		}
	}

	onMount(() => {
		if (queryInput) {
			setTimeout(() => queryInput.focus(), 100);
		}
		loadConversationHistory();
		loadConversations(); // Load conversations on mount

		// Set up periodic polling for updates (every 10 seconds)
		pollInterval = setInterval(checkForUpdates, 10000);

		// Resume polling when tab becomes visible and check for updates immediately
		const handleVisibilityChange = () => {
			if (!document.hidden) {
				// Reset poll attempts when tab becomes visible
				pollAttempts = 0;
				// Restart polling if it was stopped due to errors
				if (!pollInterval) {
					pollInterval = setInterval(checkForUpdates, 10000);
				}
				// Check for updates immediately when tab becomes visible
				checkForUpdates();
			}
		};
		
		document.addEventListener('visibilitychange', handleVisibilityChange);
		document.addEventListener('click', handleClickOutside); // Add click outside listener

		// Add delegated event listener for ticker buttons
		if (messagesContainer) {
			messagesContainer.addEventListener('click', handleTickerButtonClick);
		}

		// Cleanup listener on component destroy
		return () => {
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			document.removeEventListener('click', handleClickOutside); // Clean up click outside listener
			if (messagesContainer) {
				messagesContainer.removeEventListener('click', handleTickerButtonClick);
			}
		};
	});

	onDestroy(() => {
		// Clean up polling interval
		if (pollInterval) {
			clearInterval(pollInterval);
		}
	});

	// Generate unique IDs for messages
	function generateId(): string {
		return Date.now().toString(36) + Math.random().toString(36).substring(2);
	}

	// Scroll to bottom of chat (for user-initiated actions)
	function scrollToBottom() {
		setTimeout(() => {
			if (messagesContainer) {
				messagesContainer.scrollTop = messagesContainer.scrollHeight;
			}
		}, 100);
	}

	// Function to handle clicking on a suggested query
	function handleSuggestedQueryClick(query: string) {
		inputValue.set(query)
		handleSubmit();
	}

	// Function to toggle backtest mode
	function toggleBacktestMode() {
		isBacktestMode = !isBacktestMode;
	}

	async function handleSubmit() {
		if (!$inputValue.trim() || isLoading) return;
		
		isLoading = true;
		let loadingMessage: Message | null = null;
		let backendConversationId = ''; // Track the conversation ID from backend
		
		// Create new abort controller for this request
		currentAbortController = new AbortController();
		
		try { 
			const userMessage: Message = {
				id: generateId(),
				content: $inputValue,
				sender: 'user',
				timestamp: new Date(),
				contextItems: [...$contextItems]
			};

			messagesStore.update(current => [...current, userMessage]);

			// Create loading message placeholder
			loadingMessage = {
				id: generateId(),
				content: '', // Content is now handled by the store
				sender: 'assistant',
				timestamp: new Date(),
				isLoading: true
			};
			messagesStore.update(current => [...current, loadingMessage as Message]);
			
			// <-- Set initial status immediately -->
			functionStatusStore.set({
				type: 'function_status',
				userMessage: 'Processing request...'
			});

			// Scroll to show the user's message and loading state
			scrollToBottom();

			currentProcessingQuery = $inputValue; // Store the query before clearing input
			inputValue.set('');
			// We already set an initial status, no need to clear here
			await tick(); // Wait for DOM update
			adjustTextareaHeight(); // Reset height after clearing input and waiting for tick

			// Prepend if backtest mode is active
			const finalQuery = isBacktestMode ? `[RUN BACKTEST] ${currentProcessingQuery}` : currentProcessingQuery;
			const currentActiveChart = $activeChartInstance; // Get current active chart instance
			
			try {
				const response = await privateRequest('getQuery', {
					query: finalQuery,
					context: $contextItems, // Send only manually added context items
					activeChartContext: currentActiveChart, // Send active chart separately
					conversation_id: currentConversationId || '' // Send empty string for new chats
				}, false, false, currentAbortController?.signal);

				// Check if request was cancelled while awaiting
				if (requestCancelled) {
					functionStatusStore.set(null);
					// Try to clean up pending message on backend
					await cleanupPendingMessage(currentProcessingQuery);
					return;
				}

				// Type assertion to handle the response type
				const typedResponse = response as unknown as QueryResponse;

				// Store the conversation ID for potential cleanup
				if (typedResponse.conversation_id) {
					backendConversationId = typedResponse.conversation_id;
				}

				// Clear status store on success
				functionStatusStore.set(null);

				// Update conversation ID if this was a new chat
				if (typedResponse.conversation_id && !currentConversationId) {
					currentConversationId = typedResponse.conversation_id;
					// Load conversations to get the title
					await loadConversations();
				}

				const now = new Date();

				const assistantMessage: Message = {
					id: generateId(),
					content: typedResponse.text || "Error processing request.",
					sender: 'assistant',
					timestamp: now,
					responseType: typedResponse.type,
					contentChunks: typedResponse.content_chunks,
					suggestedQueries: typedResponse.suggestions || [],
					status: 'completed',
					completedAt: now
				};

				messagesStore.update(current => [...current, assistantMessage]);
				
				// Update last seen timestamp since we just saw this response
				const lastSeenKey = 'chat_last_seen_timestamp';
				localStorage.setItem(lastSeenKey, now.toISOString());
				
				// If we didn't have a conversation ID before, we should have one now
				// Load conversation history to get the new conversation ID
				if (!currentConversationId) {
					await loadConversationHistory(false); // Don't scroll since we just added the message
					await loadConversations(); // Refresh conversation list
				}
				
			} catch (error: any) {
				// Check if the request was cancelled (either by AbortController or by our cancellation response)
				if (requestCancelled || (error.cancelled === true)) {
					functionStatusStore.set(null);
					// Try to clean up pending message on backend
					await cleanupPendingMessage(currentProcessingQuery);
					return;
				}
				
				console.error('Error fetching response:', error);

				// Clear status store on error
				functionStatusStore.set(null);

				// Try to clean up pending message on backend for network errors
				await cleanupPendingMessage(currentProcessingQuery);

				const errorMessage: Message = {
					id: generateId(),
					content: `Error: ${error.message || 'Failed to get response'}`,
					sender: 'assistant',
					timestamp: new Date(),
					responseType: 'error'
				};

				messagesStore.update(current => [...current, errorMessage]);
			}
		} catch (error: any) {
			console.error('Error in handleSubmit:', error);
			
			// Clear status store on any error
			functionStatusStore.set(null);
			
			// Try to clean up pending message on backend
			await cleanupPendingMessage(currentProcessingQuery);
			
			// Add error message if we have a loading message
			if (loadingMessage) {
				const errorMessage: Message = {
					id: generateId(),
					content: `Error: ${error.message || 'An unexpected error occurred'}`,
					sender: 'assistant',
					timestamp: new Date(),
					responseType: 'error'
				};

				messagesStore.update(current => [...current, errorMessage]);
			}
		} finally {
			// Always clean up loading message and reset state
			if (loadingMessage) {
				messagesStore.update(current => current.filter(m => m.id !== loadingMessage!.id));
			}
			isLoading = false;
			currentAbortController = null;
			requestCancelled = false;
			currentProcessingQuery = ''; // Clear the processing query
		}
	}

	// Helper function to clean up pending messages on the backend
	async function cleanupPendingMessage(query: string) {
		try {
			const conversationIdToUse = currentConversationId || '';
			if (conversationIdToUse && query) {
				await privateRequest('cancelPendingMessage', {
					conversation_id: conversationIdToUse,
					query: query
				});
			}
		} catch (cleanupError) {
			console.warn('Failed to cleanup pending message on backend:', cleanupError);
			// Don't throw - this is a cleanup operation that shouldn't affect the main flow
		}
	}

	// Function to cancel/pause the current request
	async function handleCancelRequest() {
		if (isLoading) {
			requestCancelled = true;
			functionStatusStore.set(null);
			
			// Abort the HTTP request - this will cancel the backend processing
			if (currentAbortController) {
				currentAbortController.abort();
			}
			
			// Clean up pending message on backend using the tracked query
			if (currentProcessingQuery) {
				await cleanupPendingMessage(currentProcessingQuery);
			}
			
			// Remove any loading messages
			messagesStore.update(current => current.filter(m => !m.isLoading));
			
			isLoading = false;
			currentAbortController = null;
		}
	}

	// Function to adjust textarea height dynamically
	function adjustTextareaHeight() {
		if (!queryInput) return;
		queryInput.style.height = 'auto'; // Reset height to allow shrinking
		// Force reflow to ensure the 'auto' height takes effect before reading scrollHeight
		queryInput.offsetHeight; 
		queryInput.style.height = `${queryInput.scrollHeight}px`; // Set height to content height
	}

	function formatTimestamp(date: Date): string {
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}





	// Function to clear conversation history
	async function clearConversation() {
		// Instead of clearing the current conversation, create a new one
		createNewConversation();
	}

	// Function to safely access table data properties
	function isTableData(content: any): content is TableData {
		return typeof content === 'object' && 
			content !== null && 
			Array.isArray(content.headers) && 
			Array.isArray(content.rows);
	}
	
	// Function to get table data safely
	function getTableData(content: any): TableData | null {
		if (isTableData(content)) {
			return content;
		}
		return null;
	}

	// Function to handle clicks on ticker buttons
	async function handleTickerButtonClick(event: MouseEvent) {
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

	// Function to toggle table expansion state
	function toggleTableExpansion(tableKey: string) {
		if (!(tableKey in tableExpansionStates)) {
			tableExpansionStates[tableKey] = false; // Default to collapsed if not set
		}
		tableExpansionStates[tableKey] = !tableExpansionStates[tableKey];
		tableExpansionStates = { ...tableExpansionStates }; // Trigger reactivity
	}

	// Function to sort table data
	function sortTable(tableKey: string, columnIndex: number, tableData: TableData) {
		const currentSortState = tableSortStates[tableKey] || { columnIndex: null, direction: null };
		let newDirection: 'asc' | 'desc' | null = 'asc';

		if (currentSortState.columnIndex === columnIndex) {
			// Toggle direction if clicking the same column
			newDirection = currentSortState.direction === 'asc' ? 'desc' : 'asc';
		}

		// Update sort state for this table
		tableSortStates[tableKey] = { columnIndex, direction: newDirection };
		tableSortStates = { ...tableSortStates }; // Trigger reactivity

		// Sort the rows
		tableData.rows.sort((a, b) => {
			const valA = a[columnIndex];
			const valB = b[columnIndex];

			// Basic comparison, can be enhanced for types
			let comparison = 0;
			if (typeof valA === 'number' && typeof valB === 'number') {
				comparison = valA - valB;
			} else {
				// Attempt numeric conversion for strings that look like numbers
				const numA = Number(String(valA).replace(/[^0-9.-]+/g,""));
				const numB = Number(String(valB).replace(/[^0-9.-]+/g,""));

				if (!isNaN(numA) && !isNaN(numB)) {
					comparison = numA - numB;
				} else {
					// Fallback to string comparison
					const stringA = String(valA).toLowerCase();
					const stringB = String(valB).toLowerCase();
					if (stringA < stringB) {
						comparison = -1;
					} else if (stringA > stringB) {
						comparison = 1;
					}
				}
			}


			return newDirection === 'asc' ? comparison : comparison * -1;
		});

		// Find the message containing this table and update its content_chunks
		// This is necessary because tableData is a copy within the #each loop
		messagesStore.update(current => current.map(msg => {
			if (msg.contentChunks) {
				msg.contentChunks = msg.contentChunks.map((chunk, idx) => {
					const currentTableKey = msg.id + '-' + idx;
					if (currentTableKey === tableKey && chunk.type === 'table' && isTableData(chunk.content)) {
						// Return a new chunk object with the sorted rows
						return {
							...chunk,
							content: {
								...chunk.content,
								rows: [...tableData.rows] // Ensure a new array reference
							}
						};
					}
					return chunk;
				});
			}
			return msg;
		}));
	}

	// Format timestamp for context chip (matches parseMarkdown format)
	function formatChipDate(timestampMs?: number): string {
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

	// Reactive block to handle pending query
	$: if ($pendingChatQuery && browser && historyLoaded) {
		const queryData = $pendingChatQuery;
		pendingChatQuery.set(null); // Clear the pending query immediately to prevent re-triggering

		// Add context items (preventing duplicates)
		contextItems.update(currentItems => {
			const newItems = queryData.context.filter((newItem: Instance | FilingContext) =>
				!currentItems.some(existingItem =>
					// Simple comparison logic, adjust if needed for more complex cases
					JSON.stringify(existingItem) === JSON.stringify(newItem)
				)
			);
			return [...currentItems, ...newItems];
		});

		// Set the input value
		inputValue.set(queryData.query);

		// Use tick to ensure input value is updated before submitting
		tick().then(() => {
			handleSubmit();
		});
	}
</script>

<div class="chat-container">
	<div class="chat-header">
		<div class="header-left">
			<div class="conversation-dropdown-container" bind:this={conversationDropdown}>
				<button class="hamburger-button" on:click={toggleConversationDropdown} aria-label="Open conversations menu">
					<svg viewBox="0 0 24 24" width="20" height="20">
						<path d="M3,6H21V8H3V6M3,11H21V13H3V11M3,16H21V18H3V16Z" fill="currentColor" />
					</svg>
				</button>
				
				{#if showConversationDropdown}
					<div class="conversation-dropdown">
						<div class="dropdown-header">
							<h4>Conversations</h4>
							<button class="new-conversation-btn" on:click={createNewConversation}>
								<svg viewBox="0 0 24 24" width="16" height="16">
									<path d="M19,13H13V19H11V13H5V11H11V5H13V11H19V13Z" fill="currentColor" />
								</svg>
								New Chat
							</button>
						</div>
						
						<div class="conversation-list">
							{#if loadingConversations}
								<div class="loading-conversations">Loading...</div>
							{:else if conversations.length === 0}
								<div class="no-conversations">No conversations yet</div>
							{:else}
								{#each conversations as conversation (conversation.conversation_id)}
									<div 
										class="conversation-item {conversation.conversation_id === currentConversationId ? 'active' : ''}"
										on:click={() => switchToConversation(conversation.conversation_id, conversation.title)}
									>
										<div class="conversation-info">
											<div class="conversation-title">{conversation.title}</div>
											<div class="conversation-meta">
												{new Date(conversation.updated_at).toLocaleDateString()}
											</div>
										</div>
										<button 
											class="delete-conversation-btn"
											on:click={(e) => deleteConversation(conversation.conversation_id, e)}
											aria-label="Delete conversation"
										>
											<svg viewBox="0 0 24 24" width="14" height="14">
												<path d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z" fill="currentColor" />
											</svg>
										</button>
									</div>
								{/each}
							{/if}
						</div>
					</div>
				{/if}
			</div>
			
			<h3>{currentConversationTitle}</h3>
		</div>
		
		<div class="header-right">
			{#if $messagesStore.length > 0}
				<button class="clear-button" on:click={clearConversation} disabled={isLoading}>
					<svg viewBox="0 0 24 24" width="16" height="16">
						<path d="M19,13H13V19H11V13H5V11H11V5H13V11H19V13Z" fill="currentColor" />
					</svg>
					New Chat
				</button>
			{/if}
		</div>
	</div>

	<div class="chat-messages" bind:this={messagesContainer}>
		{#if $messagesStore.length === 0}
			<!-- Only show the container and header when chat is empty -->
			<div class="initial-container">
				<!-- Capabilities text merged here -->
				<p class="capabilities-text">Chat is a powerful interface for analyzing market data, filings, news, backtesting strategies, and more. It can answer questions and perform tasks.</p>
				<p class="suggestions-header">Ask Atlantis a question or to perform a task to get started.</p>


			</div>
		{:else}
			{#each $messagesStore as message (message.id)}
				<div class="message-wrapper {message.sender}">
					<div
						class="message {message.sender} {message.responseType === 'error'
							? 'error'
							: ''} {message.isNewResponse ? 'new-response' : ''}"
					>
						{#if message.isLoading}
							<!-- Always display status text when loading, as we set an initial one -->
							<p class="loading-status">{$functionStatusStore?.userMessage || 'Processing...'}</p> 
						{:else}
							<!-- Show new response indicator -->
							{#if message.isNewResponse && message.sender === 'assistant'}
								<div class="new-response-indicator">
									<span class="new-badge">New</span>
								</div>
							{/if}
							
							<!-- Display context chips for user messages -->
							{#if message.sender === 'user' && message.contextItems && message.contextItems.length > 0}
								<div class="message-context-chips">
									{#each message.contextItems as item (item.securityId + '-' + ('filingType' in item ? item.link : item.timestamp))}
										{@const isFiling = 'filingType' in item}
										<span class="message-context-chip">
											{item.ticker?.toUpperCase() || ''}
											{#if isFiling}
												{item.filingType}
											{:else}
												{formatChipDate(item.timestamp)}
											{/if}
										</span>
									{/each}
								</div>
							{/if}
							<div class="message-content">
								{#if message.contentChunks && message.contentChunks.length > 0}
									<div class="content-chunks">
										{#each message.contentChunks as chunk, index}
											{#if chunk.type === 'text'}
												<div class="chunk-text">
													{@html parseMarkdown(typeof chunk.content === 'string' ? chunk.content : String(chunk.content))}
												</div>
											{:else if chunk.type === 'table'}
												{#if isTableData(chunk.content)}
													{@const tableData = getTableData(chunk.content)}
													{@const tableKey = message.id + '-' + index}
													{@const isLongTable = tableData && tableData.rows.length > 5}
													{@const isExpanded = tableExpansionStates[tableKey] === true}
													{@const currentSort = tableSortStates[tableKey] || { columnIndex: null, direction: null }}

													{#if tableData}
														<div class="chunk-table-wrapper">
															{#if tableData.caption}
																<div class="table-caption">
																	{@html parseMarkdown(tableData.caption)}
																</div>
															{/if}
															<div class="chunk-table {isExpanded ? 'expanded' : ''}">
																<table>
																	<thead>
																		<tr>
																			{#each tableData.headers as header, colIndex}
																				<th
																					on:click={() => sortTable(tableKey, colIndex, JSON.parse(JSON.stringify(tableData)))}
																					class:sortable={true}
																					class:sorted={currentSort.columnIndex === colIndex}
																					class:asc={currentSort.columnIndex === colIndex && currentSort.direction === 'asc'}
																					class:desc={currentSort.columnIndex === colIndex && currentSort.direction === 'desc'}
																				>
																					{header}
																					{#if currentSort.columnIndex === colIndex}
																						<span class="sort-indicator">
																							{currentSort.direction === 'asc' ? '▲' : '▼'}
																						</span>
																					{/if}
																				</th>
																			{/each}
																		</tr>
																	</thead>
																	<tbody>
																		{#each tableData.rows as row, rowIndex}
																			{#if rowIndex < 5 || isExpanded}
																			<tr>
																				{#each row as cell}
																				<td>{@html parseMarkdown(typeof cell === 'string' ? cell : String(cell))}</td>
																				{/each}
																			</tr>
																			{/if}
																		{/each}
																	</tbody>
																</table>
															</div>
															{#if isLongTable}
																<button class="table-toggle-btn" on:click={() => toggleTableExpansion(tableKey)}>
																	{isExpanded ? 'Show less' : `Show more (${tableData.rows.length} rows)`}
																</button>
															{/if}
														</div>
													{:else}
														<div class="chunk-error">Invalid table data structure</div>
													{/if}
												{:else}
													<div class="chunk-error">Invalid table data format</div>
												{/if}
											{/if}
										{/each}
									</div>
								{:else}
									<p>{@html parseMarkdown(message.content)}</p>
								{/if}
							</div>

							{#if message.suggestedQueries && message.suggestedQueries.length > 0}
								<div class="suggested-queries">
									{#each message.suggestedQueries as query}
										<button 
											class="suggested-query-btn" 
											on:click={() => handleSuggestedQueryClick(query)}
										>
											{query}
										</button>
									{/each}
								</div>
							{/if}

							<div class="message-footer">
								<div class="message-timestamp">
									{formatTimestamp(message.timestamp)}
								</div>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		{/if}
	</div>

	<div class="chat-input-wrapper">
		<!-- Moved Initial Suggestions Here -->
		{#if $showChips && topChips.length}
		  <div class="chip-row">
		    {#each initialSuggestions as q, i}
		      {#if i < 3 || showAllInitialSuggestions}
		      <button class="chip suggestion-chip" on:click={() => handleSuggestedQueryClick(q)}>
		        <kbd>{i + 1}</kbd> {q}
		      </button>
		      {/if}
		    {/each}
		    {#if initialSuggestions.length > 3 && !showAllInitialSuggestions}
		      <button class="chip suggestion-chip more" on:click={() => showAllInitialSuggestions = true}>⋯ More</button>
		    {/if}
		  </div>
		{/if}

		<div class="input-actions">
			<button
				class="action-toggle-button {isBacktestMode ? 'active' : ''}"
				on:click={toggleBacktestMode}
				aria-label="Toggle Backtest Mode"
				title="Toggle Backtest Mode"
			>
				Backtest
			</button>
		</div>

		<div class="input-area-wrapper">
			{#if $contextItems.length > 0}
				<div class="context-chips">
					{#each $contextItems as item (
						item.securityId + '-' + ('filingType' in item ? item.link : item.timestamp)
					)}
						{@const isFiling = 'filingType' in item}
						<button
							type="button"
							class="chip"
							on:click={() => {
								if (isFiling) {
									removeFilingFromChat(item);
								} else {
									removeInstanceFromChat(item);
								}
							}}
						>
							{item.ticker?.toUpperCase() || ''}
							{#if isFiling}
								{item.filingType}
							{:else}
								{formatChipDate(item.timestamp)}
							{/if}
							×
						</button>
					{/each}
				</div>
			{/if}

			<div class="input-field-container">
				<textarea
					class="chat-input"
					placeholder="Ask about anything..."
					bind:value={$inputValue}
					bind:this={queryInput}
					rows="1"
					on:input={adjustTextareaHeight}
					on:keydown={(event) => {
						// Prevent space key events from propagating to parent elements
						if (event.key === ' ' || event.code === 'Space') {
							event.stopPropagation();
						}

						// Handle keyboard shortcuts for chips
						if (event.key >= '1' && event.key <= '3' && $showChips && topChips[+event.key - 1]) {
							handleSuggestedQueryClick(topChips[+event.key - 1]);
							event.preventDefault();
							return; // Prevent Enter key processing
						}

						// Submit on Enter, allow newline with Shift+Enter
						if (event.key === 'Enter' && !event.shiftKey) {
							event.preventDefault(); // Prevent default newline insertion
							handleSubmit();
						}
					}}
				></textarea>
				<button
					class="send-button {isLoading ? 'loading' : ''}"
					on:click={isLoading ? handleCancelRequest : handleSubmit}
					aria-label={isLoading ? "Cancel request" : "Send message"}
					disabled={!isLoading && !$inputValue.trim()}
				>
					{#if isLoading}
						<svg viewBox="0 0 24 24" class="send-icon pause-icon">
							<path d="M6,6H18V18H6V6Z" />
						</svg>
					{:else}
						<svg viewBox="0 0 24 24" class="send-icon">
							<path d="M2,21L23,12L2,3V10L17,12L2,14V21Z" />
						</svg>
					{/if}
				</button>
			</div>
		</div>
	</div>
</div>

<style>
	.chat-container {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.chat-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem 1.5rem;
		border-bottom: 1px solid var(--ui-border); /* Use theme border */
		background: var(--ui-bg-primary); /* Use theme background */
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 1rem;
	}

	.header-right {
		display: flex;
		align-items: center;
	}

	.conversation-dropdown-container {
		position: relative;
		display: flex;
		align-items: center;
	}

	.hamburger-button {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 0.25rem;
		color: var(--text-secondary, #aaa);
		cursor: pointer;
		transition: all 0.2s;
	}

	.hamburger-button:hover {
		background: var(--ui-bg-hover, rgba(255, 255, 255, 0.1));
		color: var(--text-primary, #fff);
		border-color: var(--ui-border, #444);
	}

	.conversation-dropdown {
		position: absolute;
		top: 100%;
		left: 0;
		width: 350px;
		max-height: 400px;
		background: #333;
		background-color: #333;
		border: 1px solid var(--ui-border, #444);
		border-radius: 0.5rem;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
		z-index: 1000;
		margin-top: 0.5rem;
		overflow: hidden;
	}

	.dropdown-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		border-bottom: 1px solid var(--ui-border, #444);
		background: #2a2a2a;
		background-color: #2a2a2a;
	}

	.dropdown-header h4 {
		margin: 0;
		font-size: 0.9rem;
		font-weight: 500;
		color: var(--text-primary, #fff);
	}

	.new-conversation-btn {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.6rem;
		background: var(--accent-color, #3a8bf7);
		color: white;
		border: none;
		border-radius: 0.25rem;
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
	}

	.new-conversation-btn:hover {
		background: var(--accent-color-hover, #2c7ae0);
		transform: translateY(-1px);
	}

	.conversation-list {
		max-height: 200px; /* Reduced from 300px to fit exactly 4 items */
		overflow-y: auto;
	}

	.conversation-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.5rem 0.75rem; /* Reduced from 0.75rem 1rem */
		cursor: pointer;
		transition: background-color 0.2s;
		border-bottom: 1px solid var(--ui-border-darker, #2a2a2a);
	}

	.conversation-item:hover {
		background: var(--ui-bg-hover, rgba(255, 255, 255, 0.05));
	}

	.conversation-item.active {
		background: var(--accent-color-faded, rgba(58, 139, 247, 0.1));
		border-left: 3px solid var(--accent-color, #3a8bf7);
	}

	.conversation-info {
		flex: 1;
		min-width: 0;
	}

	.conversation-title {
		font-size: 0.8rem; /* Reduced from 0.85rem */
		font-weight: 500;
		color: var(--text-primary, #fff);
		margin-bottom: 0.15rem; /* Reduced from 0.2rem */
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.conversation-meta {
		font-size: 0.65rem; /* Reduced from 0.7rem */
		color: var(--text-secondary, #aaa);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.delete-conversation-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0.3rem;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 0.25rem;
		color: var(--text-secondary, #aaa);
		cursor: pointer;
		transition: all 0.2s;
		opacity: 0;
		margin-left: 0.5rem;
	}

	.conversation-item:hover .delete-conversation-btn {
		opacity: 1;
	}

	.delete-conversation-btn:hover {
		background: var(--error-color-faded, rgba(244, 67, 54, 0.1));
		border-color: var(--error-color, #f44336);
		color: var(--error-color, #f44336);
	}

	.loading-conversations,
	.no-conversations {
		padding: 2rem 1rem;
		text-align: center;
		color: var(--text-secondary, #aaa);
		font-size: 0.8rem;
	}

	/* Custom scrollbar for conversation list */
	.conversation-list::-webkit-scrollbar {
		width: 6px;
	}

	.conversation-list::-webkit-scrollbar-track {
		background: var(--ui-bg-element-darker, #2a2a2a);
	}

	.conversation-list::-webkit-scrollbar-thumb {
		background-color: var(--ui-border, #444);
		border-radius: 3px;
	}

	.conversation-list::-webkit-scrollbar-thumb:hover {
		background-color: var(--ui-accent, #3a8bf7);
	}

	.chat-header h3 {
		margin: 0;
		font-size: 1rem;
		font-weight: 500;
		color: var(--text-primary, #fff);
	}

	.clear-button {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.3rem 0.6rem; /* Slightly smaller padding */
		background: transparent; /* Remove background */
		color: var(--text-secondary, #aaa); /* Use secondary text color */
		border: 1px solid transparent; /* Transparent border */
		border-radius: 0.25rem;
		font-size: 0.75rem;
		cursor: pointer;
		transition: all 0.2s;
	}

	.clear-button:hover:not(:disabled) {
		background: var(--ui-bg-hover, rgba(255, 255, 255, 0.1)); /* Subtle hover */
		color: var(--text-primary, #fff); /* Primary text color on hover */
		border-color: var(--ui-border, #444); /* Show border on hover */
	}

	.clear-button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.clear-button svg {
		width: 1rem;
		height: 1rem;
	}

	.chat-messages {
		flex: 1;
		overflow-y: auto;
		padding: clamp(0.75rem, 2vw, 1.5rem);
		display: flex;
		flex-direction: column;
		gap: 1rem;
		scroll-behavior: smooth;
	}

	.empty-chat {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: var(--text-secondary, #aaa);
		gap: 1rem;
		text-align: center;
		padding: 2rem;
	}

	.empty-chat-icon {
		opacity: 0.5;
	}

	.message-wrapper {
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.message-wrapper.user {
		align-items: flex-end;
	}

	.message-wrapper.assistant {
		align-items: flex-start;
		width: 100%;
		border-bottom: 1px solid var(--ui-border, #333);
		padding-bottom: 0.75rem;
	}

	.message {
		max-width: 80%;
		padding: 0.75rem 1rem;
		border-radius: 1rem;
		position: relative;
		font-size: 0.875rem;
	}

	.message.user {
		background: var(--accent-color, #3a8bf7);
		color: white;
		border-bottom-right-radius: 0.25rem;
	}

	.message.assistant {
		background: transparent;
		border: none;
		color: var(--text-primary, #fff);
		max-width: 100%;
		width: 100%;
		padding: 0.5rem 0;
		border-radius: 0;
	}

	.message.error {
		background: rgba(244, 67, 54, 0.15);
		border-color: var(--error-color, #f44336);
	}

	.message-content {
		margin-bottom: 0.5rem;
	}

	.message-content p {
		margin: 0;
		white-space: pre-wrap;
		line-height: 1.5;
		font-size: 0.875rem;
	}

	.message-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-top: 0.5rem;
		font-size: 0.65rem;
		opacity: 0.7;
	}

	.function-tools {
		margin-top: 0.75rem;
		margin-bottom: 0.5rem;
	}

	.function-tool {
		background: var(--ui-bg-element-darker, #2a2a2a);
		border-radius: 0.5rem;
		overflow: hidden;
		margin-bottom: 0.5rem;
		border: 1px solid var(--ui-border, #444);
	}

	.function-tool.error {
		border-color: var(--error-color, #f44336);
	}

	.function-tool.success {
		border-color: var(--success-color, #4caf50);
	}

	.function-header {
		display: flex;
		align-items: center;
		padding: 0.5rem 0.75rem;
		background: rgba(0, 0, 0, 0.2);
		gap: 0.5rem;
	}

	.function-name {
		font-weight: 500;
		font-size: 0.8rem;
		color: var(--accent-color, #3a8bf7);
	}

	.function-error {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem;
		color: var(--error-color, #f44336);
		font-size: 0.8rem;
	}

	.function-result-data {
		padding: 0.75rem;
		font-size: 0.75rem;
		overflow-x: auto;
	}

	/* Suggested queries styling */
	.suggested-queries {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin: 0.75rem 0; /* Keep vertical margin */
	}

	/* Specific styling for initial suggestions above input */
	.initial-suggestions-input-area {
		width: 100%; /* Ensure container takes full width */
		padding-bottom: 0.5rem; /* Space between suggestions and actions */
		border-bottom: 1px solid var(--ui-border); /* Separator line */
		margin-bottom: 0.5rem; /* Space below the separator */
	}

	/* Adjust margin for suggested queries only when they are in the input area */
	.initial-suggestions-input-area .suggested-queries {
		margin: 0; /* Remove margin when above input */
		display: flex; /* Use flexbox */
		flex-direction: column; /* Stack items vertically */
		align-items: stretch; /* Make items stretch to fill width */
		gap: 0.5rem; /* Keep gap between buttons */
	}

	/* Reset styles specifically for buttons in the initial suggestions area */
	.initial-suggestions-input-area .suggested-query-btn {
		padding: 0.4rem 1rem; /* Use standard button padding */
		height: auto;         /* Reset height */
		flex: none;           /* Reset flex property */
		text-align: center;   /* Center text within the stretched button */
	}

	.suggested-query-btn {
		/* Match the ticker button styles */
		background: var(--ui-bg-element-darker, #2a2a2a);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		padding: 0.25rem 0.8rem; /* Adjusted vertical padding */
		border-radius: 0.25rem; /* Match border-radius */
		font-size: 0.75rem; /* Match font-size */
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
		max-width: 100%; /* Keep max-width */
		line-height: 1.3; /* ADJUST line-height for wrapping */
		text-decoration: none; /* Remove default button underline */
		display: inline-block; /* Ensure proper alignment */
		text-align: left; /* Align wrapped text to the left */
	}

	.suggested-query-btn:hover {
		/* Match the ticker button hover styles */
		background: var(--ui-bg-element, #333);
		border-color: var(--ui-accent, #3a8bf7);
		color: var(--text-primary, #fff);
	}

	.chat-input-wrapper {
		display: flex;
		flex-direction: column; /* Stack actions and input vertically */
		padding: clamp(0.5rem, 1.5vw, 1rem) clamp(0.75rem, 2vw, 1.5rem);
		background: var(--ui-bg-primary); /* Use theme background */
		border-top: 1px solid var(--ui-border); /* Use theme border */
		gap: 0.5rem; /* Gap between actions and input field */
	}

	.input-actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}

	.action-toggle-button {
		padding: 0.4rem 0.8rem;
		font-size: 0.75rem;
		border-radius: 1rem; /* More rounded */
		border: 1px solid var(--ui-border, #444);
		background: var(--ui-bg-element, #333);
		color: var(--text-secondary, #aaa);
		cursor: pointer;
		transition: all 0.2s ease;
		font-weight: 500;
	}

	.action-toggle-button:hover {
		border-color: var(--text-secondary, #aaa);
		color: var(--text-primary, #fff);
	}

	.action-toggle-button.active {
		background: var(--accent-color, #3a8bf7);
		border-color: var(--accent-color, #3a8bf7);
		color: white;
	}

	.input-field-container {
		position: relative; /* Needed for absolute positioning of send button */
		display: flex; /* Keep input and send button together */
		width: 100%;
	}

	.chat-input {
		flex: 1;
		padding: clamp(0.75rem, 1.5vw, 1rem);
		font-size: clamp(0.875rem, 1vw, 1rem);
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		border-radius: 0.25rem;
		padding-right: 3rem; /* Adjusted for the new button size */
		resize: none; /* Disable manual resizing */
		overflow-y: hidden; /* Hide scrollbar during resize and when content fits */
		line-height: 1.4; /* Adjust line height for better readability in textarea */
		height: auto; /* Allow height to adjust */
		box-sizing: border-box; /* Include padding and border in element's total width and height */
		font-family: inherit; /* Ensure font matches the rest of the UI */
		max-height: 200px; /* Add a max height to prevent excessive growth */
		/* overflow-y: auto; */ /* Remove this; scrollbar appears automatically if max-height exceeded and overflow is not hidden */
	}

	.send-button {
		padding: 0; 
		line-height: 0;
		width: 32px;
		height: 32px;
		position: absolute;
		right: 0.5rem;
		bottom: 0.5rem;
		background: white; /* White background */
		border: none;
		border-radius: 50%; /* Perfect circle */
		cursor: pointer;
		min-width: 32px; /* Prevent shrinking */
		min-height: 32px; /* Prevent shrinking */
		max-width: 32px; /* Prevent growing */
		max-height: 32px; /* Prevent growing */
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all 0.2s ease;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		z-index: 100; /* Much higher z-index to ensure it's on top */
		flex-shrink: 0; /* Prevent flexbox from shrinking the button */
		box-sizing: border-box; /* Include border in size calculations */
	}

	.send-button:hover:not(:disabled) {
		background: #f5f5f5; /* Light gray on hover */
		transform: translateY(-1px);
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
	}

	.send-button:hover:not(:disabled):not(.loading) {
		background: #f5f5f5; /* Light gray on hover for send button only */
		transform: translateY(-1px);
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
	}

	.send-button:disabled {
		background: #444; /* Use solid color instead of CSS variable */
		color: #aaa; /* Use solid color instead of CSS variable */
		cursor: not-allowed;
		opacity: 0.6;
		transform: none;
		box-shadow: none;
	}

	.send-button.loading {
		background: white; /* Keep white background when loading */
		color: #000; /* Black icon when loading */
	}

	.send-button.loading:hover {
		background: white; /* No hover effect for pause button */
		transform: none; /* No transform for pause button */
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1); /* Keep original shadow */
	}

	.pause-icon {
		fill: white;
	}

	.send-icon {
		width: 1rem;
		height: 1rem;
		fill: currentColor;
	}

	.send-icon path {
		fill: #000;
	}

	.typing-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 1.5rem;
		gap: 0.25rem;
	}

	.typing-indicator span {
		width: 0.5rem;
		height: 0.5rem;
		background: var(--text-secondary, #aaa);
		border-radius: 50%;
		display: inline-block;
		animation: bounce 1.5s infinite ease-in-out;
	}

	.typing-indicator span:nth-child(1) {
		animation-delay: 0s;
	}

	.typing-indicator span:nth-child(2) {
		animation-delay: 0.2s;
	}

	.typing-indicator span:nth-child(3) {
		animation-delay: 0.4s;
	}

	@keyframes bounce {
		0%,
		80%,
		100% {
			transform: translateY(0);
		}
		40% {
			transform: translateY(-0.5rem);
		}
	}

	.message.expiring {
		border-color: rgba(255, 165, 0, 0.7); /* Orange border for expiring messages */
	}

	/* Add styles for markdown elements */
	.message-content :global(p) {
		margin: 0 0 0.5rem 0;
		font-size: 0.875rem;
	}
	
	.message-content :global(p:last-child) {
		margin-bottom: 0;
	}
	
	.message-content :global(ul), .message-content :global(ol) {
		margin: 0.5rem 0;
		padding-left: 1.5rem;
	}
	
	.message-content :global(blockquote) {
		margin: 0.5rem 0;
		padding-left: 0.75rem;
		border-left: 3px solid var(--text-secondary, #aaa);
		color: var(--text-secondary, #aaa);
	}
	
	.message-content :global(a) {
		color: var(--accent-color, #3a8bf7);
		text-decoration: none;
	}
	
	.message-content :global(a:hover) {
		text-decoration: underline;
	}
	
	.message-content :global(img) {
		max-width: 100%;
		border-radius: 0.25rem;
	}

	/* Add style for system messages */
	.message.system {
		background: var(--ui-bg-element, #333);
		border: 1px dashed var(--ui-border, #444);
		color: var(--text-secondary, #aaa);
		font-style: italic;
	}

	/* Add styles for content chunks */
	.content-chunks {
		margin-top: 1rem;
	}
	
	.chunk-text {
		margin-bottom: 1rem;
	}
	
	.chunk-table {
		margin-bottom: 1rem;
		overflow-x: auto;
	}

	.chunk-table.expanded {
		/* No height constraints when expanded */
	}

	.table-caption {
		font-weight: bold;
		margin-bottom: 0.5rem;
		font-size: 0.8rem;
	}
	
	.content-chunks table {
		width: 100%;
		border-collapse: collapse;
		margin-bottom: 1rem;
		background: rgba(0, 0, 0, 0.2);
		font-size: 0.8rem;
	}
	
	.content-chunks th {
		background: rgba(0, 0, 0, 0.3);
		padding: 0.5rem;
		text-align: left;
		border: 1px solid var(--ui-border, #444);
	}

	.content-chunks th.sortable {
		cursor: pointer;
		position: relative; /* For positioning the indicator */
		user-select: none; /* Prevent text selection on click */
	}

	.content-chunks th.sortable:hover {
		background: rgba(255, 255, 255, 0.05); /* Subtle hover */
	}

	.content-chunks th .sort-indicator {
		font-size: 0.7em; /* Smaller indicator */
		margin-left: 0.4em;
		display: inline-block;
		opacity: 0.7;
		/* Optional: absolute positioning */
		/* position: absolute; */
		/* right: 0.5rem; */
		/* top: 50%; */
		/* transform: translateY(-50%); */
	}

	.content-chunks th.sorted .sort-indicator {
		opacity: 1; /* Make active indicator fully visible */
	}
	
	.content-chunks td {
		padding: 0.5rem;
		border: 1px solid var(--ui-border, #444);
	}
	
	.chunk-error {
		background: rgba(244, 67, 54, 0.1);
		color: var(--error-color, #f44336);
		padding: 0.5rem;
		border-radius: 0.25rem;
		border: 1px solid var(--error-color, #f44336);
		margin-bottom: 1rem;
		font-size: 0.8rem;
	}

	/* Style for the ticker buttons */
	.message-content :global(.ticker-button) {
		background: var(--ui-bg-element-darker, #2a2a2a);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		padding: 0.4rem 1rem;
		border-radius: 0.25rem;
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
		margin: 0 0.2rem;
		vertical-align: middle;
		line-height: 1;
		text-decoration: none;
		display: inline-block;
	}

	.message-content :global(.ticker-button:hover) {
		background: var(--ui-bg-element, #333);
		border-color: var(--ui-accent, #3a8bf7);
		color: var(--text-primary, #fff);
	}

	/* Custom Scrollbar Styles */
	.chat-messages::-webkit-scrollbar {
		width: 8px; /* Width of the scrollbar */
	}

	.chat-messages::-webkit-scrollbar-track {
		background: var(--ui-bg-element, #333); /* Track color, matching element background */
		border-radius: 4px;
	}

	.chat-messages::-webkit-scrollbar-thumb {
		background-color: var(--ui-border-darker, #555); /* Thumb color */
		border-radius: 4px;
		border: 2px solid var(--ui-bg-element, #333); /* Creates padding around thumb */
	}

	.chat-messages::-webkit-scrollbar-thumb:hover {
		background-color: var(--ui-accent, #3a8bf7); /* Thumb color on hover */
	}

	/* Firefox scrollbar styles */
	.chat-messages {
		scrollbar-width: thin; /* "auto" or "thin" */
		scrollbar-color: var(--ui-border-darker, #555) var(--ui-bg-element, #333); /* thumb track */
	}

	.table-toggle-btn {
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-secondary, #aaa);
		padding: 0.3rem 0.8rem;
		font-size: 0.7rem;
		border-radius: 0.25rem;
		cursor: pointer;
		transition: all 0.2s ease;
		margin-top: -0.5rem; /* Pull up slightly below table */
		display: inline-block;
	}

	.table-toggle-btn:hover {
		border-color: var(--accent-color, #3a8bf7);
		color: var(--text-primary, #fff);
	}

	.context-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
	}

	.chip {
		background: var(--ui-bg-element, #333);
		color: var(--text-primary, #fff);
		border: 1px solid var(--ui-border, #444);
		padding: 0.25rem 0.5rem;
		border-radius: 0.25rem;
		font-size: 0.75rem;
		cursor: pointer;
	}

	.chip:hover {
		background: var(--ui-bg-hover, rgba(255,255,255,0.05));
	}

	/* Styles for context chips within user messages */
	.message-context-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		margin-bottom: 0.5rem; /* Space between chips and message text */
	}

	.message-context-chip {
		background: rgba(255, 255, 255, 0.1); /* Slightly different background */
		color: inherit; /* Inherit text color from message bubble */
		border: 1px solid rgba(255, 255, 255, 0.2); /* Subtle border */
		padding: 0.2rem 0.5rem; /* Slightly smaller padding */
		border-radius: 0.25rem;
		font-size: 0.7rem; /* Smaller font size */
		cursor: default; /* Not interactive */
	}

	.initial-container {
		text-align: center;
		color: var(--text-secondary, #aaa);
		display: flex;
		flex-direction: column;
		justify-content: center;
		height: 100%;
		flex: 1;
		padding: 2rem;
	}

	.suggestions-header {
		font-size: 0.9rem;
		margin-bottom: 1rem;
		font-weight: 500;
		color: var(--text-primary, #fff);
	}

	/* Style for the capabilities text within the initial container */
	.capabilities-text {
		font-size: 0.8rem;
		color: var(--text-secondary, #aaa);
		margin-bottom: 1.5rem; /* Space before the 'Ask...' prompt */
		line-height: 1.4;
	}

	.loading-status {
		color: transparent;
		font-size: 0.8rem;
		padding: 0.5rem 0;
		margin: 0;
		text-align: left;
		/* Apply gradient as background - Increased contrast */
		background: linear-gradient(
			90deg,
			var(--text-secondary, #aaa),
			rgba(255, 255, 255, 0.9), /* Brighter middle color */
			var(--text-secondary, #aaa)
		);
		background-size: 200% auto;
		background-clip: text;
		-webkit-background-clip: text;
		/* Animate the background position - Slightly faster animation */
		animation: loading-text-highlight 2.5s infinite linear; /* Changed duration to 2s */
	}

	/* Update keyframes to animate background-position */
	@keyframes loading-text-highlight {
		0% {
			background-position: 200% center;
		}
		100% {
			background-position: -200% center;
		}
	}

	/* Chip row styles */
	.chat-input-wrapper > .chip-row {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 0.35rem;
		animation: fadeIn 0.2s;
		flex-wrap: wrap; /* Allow wrapping if needed */
		width: 100%; /* Ensure it takes full available width */
		min-width: 0; /* Help flexbox wrapping calculations */
	}

	/* Specific styles for suggestion chips to override general .chip */
	.suggestion-chip {
		font-size: 0.75rem;
		padding: 0.3rem 0.75rem;
		border-radius: 9999px; /* Pill shape */
		background: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		color: var(--text-secondary); /* Subtler text */
		transition: .15s ease;
		cursor: pointer;
	}

	.suggestion-chip:hover {
		border-color: var(--ui-accent);
		background: var(--ui-bg-element-hover, #444);
		color: var(--text-primary); /* Highlight text on hover */
	}

	.suggestion-chip kbd {
		background: transparent;
		font: inherit;
		opacity: 0.6;
		margin-right: 0.25rem;
		font-size: 0.7rem; /* Slightly smaller */
		border: 1px solid var(--ui-border-darker); /* Subtle border */
		padding: 0.1rem 0.25rem;
		border-radius: 0.2rem;
		line-height: 1; /* Prevent extra height */
		display: inline-block; /* Ensure proper layout */
	}

	@keyframes fadeIn {
		from { opacity: 0; transform: translateY(-5px); }
		to { opacity: 1; transform: translateY(0); }
	}

	/* New response indicator styles */
	.new-response-indicator {
		display: flex;
		justify-content: flex-end;
		margin-bottom: 0.5rem;
	}

	.new-badge {
		background: var(--accent-color, #3a8bf7);
		color: white;
		padding: 0.2rem 0.5rem;
		border-radius: 1rem;
		font-size: 0.7rem;
		font-weight: 500;
		animation: newResponsePulse 2s ease-in-out;
	}

	.message.new-response {
		border-left: 3px solid var(--accent-color, #3a8bf7);
		background: rgba(58, 139, 247, 0.05);
	}

	@keyframes newResponsePulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.7; }
	}

	/* Pending message styles */
	.message.assistant[data-status="pending"] {
		border-left: 3px solid var(--warning-color, #ff9800);
		background: rgba(255, 152, 0, 0.05);
	}

</style>
