<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
	import { marked } from 'marked'; // Import the markdown parser
	import { queryChart } from '$lib/features/chart/interface'; // Import queryChart
	import type { Instance } from '$lib/utils/types/types';
	import { browser } from '$app/environment'; // Import browser
	import { derived, writable, get } from 'svelte/store';
	import {
		inputValue,
		contextItems,
		addInstanceToChat,
		removeInstanceFromChat,
		removeFilingFromChat,
		type FilingContext, // Import the new type
		pendingChatQuery,
	} from './interface'
	import type { Message, ConversationData, QueryResponse, TableData, ContentChunk } from './interface';
	import { parseMarkdown, formatChipDate, formatRuntime, cleanHtmlContent, handleTickerButtonClick } from './utils';
	import { activeChartInstance } from '$lib/features/chart/interface';
	import { functionStatusStore, type FunctionStatusUpdate } from '$lib/utils/stream/socket'; // <-- Import the status store and FunctionStatusUpdate type
	import './chat.css'; // Import the CSS file
	import { generateSharedConversationLink } from './chatHelpers';
	import { showAuthModal } from '$lib/stores/authModal';
	import type { ConversationSummary } from './interface';

	export let sharedConversationId: string = '';
	export let isPublicViewing: boolean;

	// Conversation management state
	let conversations: ConversationSummary[] = [];
	let currentConversationId = '';
	let currentConversationTitle = 'Chat';
	let showConversationDropdown = false;
	let conversationDropdown: HTMLDivElement;
	let loadingConversations = false;
	let conversationToDelete = ''; // Add state to track which conversation is being deleted
	
	import ConversationHeader from './components/ConversationHeader.svelte';
	
	// Share modal reference
	let shareModalRef: HTMLDivElement;

	// Configure marked to make links open in a new tab
	const renderer = new marked.Renderer();
	
	marked.setOptions({
		renderer,
		breaks: true,
		gfm: true
	});



	let queryInput: HTMLTextAreaElement;
	let isLoading = false;
	let messagesContainer: HTMLDivElement;
	let initialSuggestions: string[] = [];
	let pollInterval: ReturnType<typeof setInterval> | null = null;
	let pollAttempts = 0;
	let maxPollAttempts = 3;

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

	// Derived store to control initial chip visibility
	const showChips = derived([inputValue, messagesStore], ([$val, $msgs]) => $msgs.length === 0 && $val.trim() === '');

	// Reactive variable for top 3 suggestions
	$: topChips = initialSuggestions.slice(0, 3);

	// State for showing all initial suggestions
	let showAllInitialSuggestions = false;

	// Add abort controller for cancelling requests
	let currentAbortController: AbortController | null = null;
	let requestCancelled = false;
	let currentProcessingQuery = ''; // Track the query currently being processed

	// Message editing state
	let editingMessageId = '';
	let editingContent = '';

	// Copy feedback state
	let copiedMessageId = '';
	let copyTimeout: ReturnType<typeof setTimeout> | null = null;

	// Share modal state
	let showShareModal = false;
	let shareLink = '';
	let shareLoading = false;
	let shareCopied = false;
	let shareCopyTimeout: ReturnType<typeof setTimeout> | null = null;

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
		if (isPublicViewing) return; // Don't load conversations in public viewing mode
		
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
		
		// Clear current chat and context items
		messagesStore.set([]);
		contextItems.set([]); // Clear context items when creating new conversation
		
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
				
				// Clear current messages and context items to prevent leakage
				messagesStore.set([]);
				contextItems.set([]); // Clear context items when switching conversations
				
				await loadConversationHistory();
				
				showConversationDropdown = false;
			}
		} catch (error) {
			console.error('Error switching conversation:', error);
		}
	}

	async function deleteConversation(conversationId: string, event: MouseEvent) {
		event.stopPropagation(); // Prevent switching to the conversation
		
		// Instead of confirm dialog, set the conversation to delete mode
		conversationToDelete = conversationId;
	}

	async function confirmDeleteConversation(conversationId: string) {
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
		} finally {
			conversationToDelete = ''; // Clear delete mode
		}
	}

	function cancelDeleteConversation() {
		conversationToDelete = ''; // Clear delete mode
	}

	function toggleConversationDropdown() {
		// Close share modal if it's open
		if (showShareModal) {
			closeShareModal();
		}
		
		showConversationDropdown = !showConversationDropdown;
		if (showConversationDropdown) {
			loadConversations();
		}
	}

	// Close dropdown when clicking outside
	function handleClickOutside(event: MouseEvent) {
		// Don't handle clicks on the share button itself
		const target = event.target as HTMLElement;
		if (target && target.closest('.share-btn')) {
			return;
		}
		
		if (showConversationDropdown && conversationDropdown && !conversationDropdown.contains(event.target as Node)) {
			showConversationDropdown = false;
		}
		// Close share modal when clicking outside
		if (showShareModal && shareModalRef && !shareModalRef.contains(event.target as Node)) {
			closeShareModal();
		}
	}

	// Load any existing conversation history from the server
	async function loadConversationHistory(shouldAutoScroll: boolean = true) {
		try {
			isLoading = true;
			let response;
			console.log("isPublicViewing", isPublicViewing)
			console.log("sharedConversationId", sharedConversationId)
			if (isPublicViewing && sharedConversationId) {
				// For public viewing, use publicRequest to get shared conversation
				response = await publicRequest('getPublicConversation', {
					conversation_id: sharedConversationId
				});
			} else {
				// For authenticated users, use privateRequest 
				response = await privateRequest('getUserConversation', {});
			}
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

					// Only process messages that have backend message IDs
					if (!msg.message_id) {
						console.warn('Skipping message without backend message_id:', msg);
						return;
					}

					// Now we know msg.message_id is defined (TypeScript will recognize this)
					const messageId = msg.message_id;
					messagesStore.update(current => [...current, {
						message_id: messageId, // Use backend ID 
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
						// Assistant messages use a different ID pattern since they're not editable
						const assistantMessageId = messageId + '_response';
						messagesStore.update(current => [...current, {
							message_id: assistantMessageId,
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
						const loadingMessageId = messageId + '_loading';
						messagesStore.update(current => [...current, {
							message_id: loadingMessageId,
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

				// Update conversation details from backend response
				// Only update conversation ID if we don't already have one or if it matches
				if (conversation.conversation_id) {
					if (!currentConversationId || currentConversationId === conversation.conversation_id) {
						currentConversationId = conversation.conversation_id;
					} else {
						// Conversation ID mismatch - log warning but don't switch
						console.warn('Conversation ID mismatch detected, keeping current conversation');
					}
				}
				if (conversation.title) {
					currentConversationTitle = conversation.title;
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
				// No conversation history, clear messages and reset state
				messagesStore.set([]);
				currentConversationId = '';
				currentConversationTitle = 'Chat';
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
		if (isPublicViewing) return; // Don't poll for public shared conversations
		
		try {
			const response = await privateRequest('getUserConversation', {});
			const conversation = response as ConversationData;
			
			if (conversation && conversation.messages && conversation.messages.length > 0) {
				// Validate that the polled conversation matches our current conversation
				// If conversation IDs don't match, don't process updates to prevent switching to wrong conversation
				if (currentConversationId && conversation.conversation_id && 
					conversation.conversation_id !== currentConversationId) {
					console.log('Polling returned different conversation ID, skipping update to prevent chat switching');
					return;
				}
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
					// Use message_id for proper identification instead of content matching
					const existingMessage = $messagesStore.find(m => m.message_id === msg.message_id && m.sender === 'user');
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
		if (queryInput && !isPublicViewing) {
			setTimeout(() => queryInput.focus(), 100);
		}
		loadConversationHistory();
		loadConversations(); // Load conversations on mount (will be skipped for public viewing)

		// Set up periodic polling for updates (every 10 seconds) - only for authenticated users
		if (!isPublicViewing) {
			pollInterval = setInterval(checkForUpdates, 10000);
		}

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
		
		// Clean up copy timeout
		if (copyTimeout) {
			clearTimeout(copyTimeout);
		}

		// Clean up share copy timeout
		if (shareCopyTimeout) {
			clearTimeout(shareCopyTimeout);
		}
	});

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
		if(isPublicViewing) {
			showAuthModal('conversations', 'signup');
			return;
		}
		inputValue.set(query)
		handleSubmit();
	}

	async function handleSubmit() {
		if (!$inputValue.trim() || isLoading) return;
		
		isLoading = true;
		let loadingMessage: Message | null = null;
		let backendConversationId = ''; // Track the conversation ID from backend
		
		// Create new abort controller for this request
		currentAbortController = new AbortController();
		
		try { 
			// Note: Create a temporary user message for immediate UI feedback
			// The backend will provide the actual message ID, but we need something for the UI
			const userMessage: Message = {
				message_id: 'temp_' + Date.now(), // Temporary ID for UI only
				content: $inputValue,
				sender: 'user',
				timestamp: new Date(),
				contextItems: [...$contextItems]
			};

			messagesStore.update(current => [...current, userMessage]);

			// Create loading message placeholder
			loadingMessage = {
				message_id: 'temp_loading_' + Date.now(),
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

			// Remove backtest mode logic - use query directly without prepending
			const finalQuery = currentProcessingQuery;
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
					// Load conversations to get the title (only for authenticated users)
					if (!isPublicViewing) {
						await loadConversations();
						// Update the title immediately if we have one in the response
						const newConversation = conversations.find(c => c.conversation_id === currentConversationId);
						if (newConversation) {
							currentConversationTitle = newConversation.title;
						}
					}
				}

				// Update the temporary user message with the real backend message ID
				if (typedResponse.message_id) {
					messagesStore.update(current => 
						current.map(msg => 
							msg.message_id === userMessage.message_id 
								? { ...msg, message_id: typedResponse.message_id! }
								: msg
						)
					);
				}

				// Use timestamps from backend response if available, otherwise use current time
				const messageTimestamp = typedResponse.timestamp ? new Date(typedResponse.timestamp) : new Date();
				const messageCompletedAt = typedResponse.completed_at ? new Date(typedResponse.completed_at) : new Date();

				const assistantMessage: Message = {
					message_id: typedResponse.message_id  + '_response' || '',
					content: typedResponse.text || "Error processing request.",
					sender: 'assistant',
					timestamp: messageTimestamp,
					contentChunks: typedResponse.content_chunks,
					suggestedQueries: typedResponse.suggestions || [],
					status: 'completed',
					completedAt: messageCompletedAt
				};

				messagesStore.update(current => [...current, assistantMessage]);
				
				// Update last seen timestamp since we just saw this response
				const lastSeenKey = 'chat_last_seen_timestamp';
				localStorage.setItem(lastSeenKey, messageCompletedAt.toISOString());
				
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
					message_id: 'temp_error_' + Date.now(),
					content: `Error: ${error.message || 'Failed to get response'}`,
					sender: 'assistant',
					timestamp: new Date(),
					status: 'error'
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
					message_id: 'temp_error_' + Date.now(),
					content: `Error: ${error.message || 'An unexpected error occurred'}`,
					sender: 'assistant',
					timestamp: new Date(),
					status: 'error'
				};

				messagesStore.update(current => [...current, errorMessage]);
			}
		} finally {
			// Always clean up loading message and reset state
			if (loadingMessage) {
				messagesStore.update(current => current.filter(m => m.message_id !== loadingMessage!.message_id));
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
			Array.isArray(content.rows) &&
			content.rows.every((row: any) => Array.isArray(row)); // Ensure each row is also an array
	}
	
	// Function to get table data safely
	function getTableData(content: any): TableData | null {
		if (isTableData(content)) {
			return content;
		}
		return null;
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
			// Skip sorting if rows are not arrays
			if (!Array.isArray(a) || !Array.isArray(b)) {
				return 0;
			}
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
					const currentTableKey = msg.message_id + '-' + idx;
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

	// Message editing functions
	function startEditing(message: Message) {
		if (message.sender !== 'user') return; // Only allow editing user messages
		editingMessageId = message.message_id;
		editingContent = message.content;
	}

	function cancelEditing() {
		editingMessageId = '';
		editingContent = '';
	}

	// Function to copy message content to clipboard
	async function copyMessageToClipboard(message: Message) {
		try {
			let textToCopy = '';
			
			if (message.contentChunks && message.contentChunks.length > 0) {
				// For messages with content chunks, extract text from each chunk
				textToCopy = message.contentChunks.map(chunk => {
					if (chunk.type === 'text') {
						const content = typeof chunk.content === 'string' ? chunk.content : String(chunk.content);
						return cleanHtmlContent(content);
					} else if (chunk.type === 'table' && isTableData(chunk.content)) {
						// For tables, create a simple text representation
						const tableData = chunk.content;
						let tableText = '';
						if (tableData.caption) {
							const cleanCaption = cleanHtmlContent(tableData.caption);
							tableText += cleanCaption + '\n\n';
						}
						// Add headers
						tableText += tableData.headers.join('\t') + '\n';
						// Add rows (also clean ticker formatting from table cells)
						tableText += tableData.rows.map(row => {
							if (Array.isArray(row)) {
								return row.map(cell => cleanHtmlContent(String(cell))).join('\t');
							} else {
								return cleanHtmlContent(String(row));
							}
						}).join('\n');
						return tableText;
					}
					return '';
				}).join('\n\n');
			} else {
				// For simple text messages
				textToCopy = cleanHtmlContent(message.content);
			}
			
			await navigator.clipboard.writeText(textToCopy);
			
			// Show success feedback
			copiedMessageId = message.message_id;
			if (copyTimeout) {
				clearTimeout(copyTimeout);
			}
			copyTimeout = setTimeout(() => {
				copiedMessageId = '';
				copyTimeout = null;
			}, 1000); // Show success state for 2 seconds
			
		} catch (error) {
			console.error('Failed to copy message to clipboard:', error);
			// Fallback for older browsers
			try {
				const textArea = document.createElement('textarea');
				const fallbackText = cleanHtmlContent(message.content);
				textArea.value = fallbackText;
				document.body.appendChild(textArea);
				textArea.select();
				document.execCommand('copy');
				document.body.removeChild(textArea);
				
				// Show success feedback for fallback too
				copiedMessageId = message.message_id;
				if (copyTimeout) {
					clearTimeout(copyTimeout);
				}
				copyTimeout = setTimeout(() => {
					copiedMessageId = '';
					copyTimeout = null;
				}, 1000);
			} catch (fallbackError) {
				console.error('Fallback copy method also failed:', fallbackError);
			}
		}
	}

	async function saveMessageEdit() {
		if (!editingMessageId || !editingContent.trim()) {
			return;
		}
		
		try {
			// Find the message being edited
			const editingMessage = $messagesStore.find(msg => msg.message_id === editingMessageId);
			
			if (!editingMessage) {
				console.error('Message to edit not found');
				return;
			}

			// Use the message ID directly (it should be the backend message ID)
			const backendMessageId = editingMessage.message_id;
			
			if (!backendMessageId) {
				console.error('No backend message ID found for editing');
				return;
			}

			// Store the new content before clearing editing state
			const newContent = editingContent.trim();

			// Only proceed if the content actually changed
			if (editingMessage.content === newContent) {
				// No changes, just exit editing mode
				editingMessageId = '';
				editingContent = '';
				return;
			}

			// Clear editing state early
			editingMessageId = '';
			editingContent = '';

			// Call backend to edit the message using the backend message ID
			const requestPayload = {
				conversation_id: currentConversationId,
				message_id: backendMessageId,
				new_query: newContent
			};

			const response = await privateRequest('editMessage', requestPayload);

			if (response && (response as any).success) {
				console.log('Edit response received:', response);
				
				// Update conversation ID first before any other operations
				if ((response as any).conversation_id) {
					currentConversationId = (response as any).conversation_id;
				}
				
				console.log('Current messages before reload:', $messagesStore.length);
				
				// Reload conversation history to reflect the archived state
				await loadConversationHistory(false);
				
				console.log('Current messages after reload:', $messagesStore.length);
				
				// Wait a moment for the UI to fully update
				await tick();

				// Now automatically send the edited message as a new chat request
				// Set the input value and context from the edit response
				inputValue.set(newContent);
				
				// Always clear existing context and set only the ones from the edited message
				contextItems.update(currentItems => {
					// Set context items from the edit response, or empty array if none
					return (response as any).context_items || [];
				});

				// Use tick to ensure input value is updated before submitting
				await tick();
				
				console.log('About to submit edited message:', newContent);
				
				// Submit the message using existing logic
				handleSubmit();
			} else {
				console.error('Backend edit failed or success=false');
				// Reload conversation to get current state
				await loadConversationHistory(false);
			}
		} catch (error) {
			console.error('Error saving message edit:', error);
			
			// Clear editing state on error
			editingMessageId = '';
			editingContent = '';
			
			// Reload conversation to get current state
			await loadConversationHistory(false);
		}
	}

	// Share conversation functions
	async function handleShareConversation() {
		if (!currentConversationId) {
			console.error('No active conversation to share');
			return;
		}

		// Close conversation dropdown if it's open
		if (showConversationDropdown) {
			showConversationDropdown = false;
		}

		showShareModal = true;
		shareLoading = true;
		shareLink = '';
		shareCopied = false;

		try {
			const link = await generateSharedConversationLink(currentConversationId);
			if (link) {
				shareLink = link;
			} else {
				console.error('Failed to generate share link');
			}
		} catch (error) {
			console.error('Error generating share link:', error);
		} finally {
			shareLoading = false;
		}
	}

	function closeShareModal() {
		showShareModal = false;
		shareLink = '';
		shareLoading = false;
		shareCopied = false;
		if (shareCopyTimeout) {
			clearTimeout(shareCopyTimeout);
			shareCopyTimeout = null;
		}
	}

	async function copyShareLink() {
		if (!shareLink) return;
		
		try {
			await navigator.clipboard.writeText(shareLink);
			shareCopied = true;
			
			if (shareCopyTimeout) {
				clearTimeout(shareCopyTimeout);
			}
			shareCopyTimeout = setTimeout(() => {
				shareCopied = false;
				shareCopyTimeout = null;
			}, 2000);
		} catch (error) {
			console.error('Failed to copy share link:', error);
		}
	}
</script>

<div class="chat-container">
	{#if !isPublicViewing}
		<ConversationHeader 
			bind:conversationDropdown
			{showConversationDropdown}
			{conversations}
			{currentConversationId}
			{currentConversationTitle}
			{loadingConversations}
			{conversationToDelete}
			{messagesStore}
			{isLoading}
			{toggleConversationDropdown}
			{createNewConversation}
			{switchToConversation}
			{deleteConversation}
			{confirmDeleteConversation}
			{cancelDeleteConversation}
			{handleShareConversation}
			{clearConversation}
		/>
	{:else}
		<!-- Simple header for public viewing -->
		<div class="public-conversation-header">
			<h3>{currentConversationTitle || 'Shared Conversation'}</h3>
		</div>
	{/if}

	<div class="chat-messages" bind:this={messagesContainer}>
		{#if $messagesStore.length === 0}
			<!-- Only show the container and header when chat is empty -->
			<div class="initial-container">
				<!-- Capabilities text merged here -->
				<p class="capabilities-text">Chat is a powerful interface for analyzing market data, filings, news, backtesting strategies, and more. It can answer questions and perform tasks.</p>
				<p class="suggestions-header">Ask Atlantis a question or to perform a task to get started.</p>


			</div>
		{:else}
			{#each $messagesStore as message (message.message_id)}
				<div class="message-wrapper {message.sender}">
					<div
						class="message {message.sender} {message.status === 'error' || message.content.includes('Error:')
							? 'error'
							: ''} {message.isNewResponse ? 'new-response' : ''} {editingMessageId === message.message_id ? 'editing' : ''} {message.sender === 'user' ? 'glass glass--pill glass--responsive' : ''}"
					>
						{#if message.isLoading}
							<!-- Always display status text when loading, as we set an initial one -->
							<p class="loading-status">{$functionStatusStore?.userMessage || 'Processing...'}</p> 
						{:else if editingMessageId === message.message_id}
							<!-- Editing interface - using CSS classes -->
							<div class="edit-container">
								<textarea
									class="edit-textarea"
									bind:value={editingContent}
									placeholder="Edit your message..."
									on:keydown={(e) => {
										if (e.key === 'Enter' && e.ctrlKey) {
											e.preventDefault();
											saveMessageEdit();
										} else if (e.key === 'Escape') {
											e.preventDefault();
											cancelEditing();
										}
									}}
								></textarea>
								<div class="edit-actions">
									<button class="edit-cancel-btn glass glass--small glass--responsive" on:click={cancelEditing}>
										Cancel
									</button>
									<button class="edit-save-btn glass glass--small glass--responsive" on:click={saveMessageEdit}>
										Send
									</button>
								</div>
							</div>
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
								{#if message.sender === 'assistant' && message.completedAt && message.timestamp}
									<div class="message-runtime">
										{formatRuntime(message.timestamp, message.completedAt)}
									</div>
								{/if}
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
													{@const tableKey = message.message_id + '-' + index}
													{@const isLongTable = tableData && tableData.rows.length > 5}
													{@const isExpanded = tableExpansionStates[tableKey] === true}
													{@const currentSort = tableSortStates[tableKey] || { columnIndex: null, direction: null }}

													{#if tableData}
														<div class="chunk-table-wrapper glass glass--rounded glass--responsive">
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
																				{#if Array.isArray(row)}
																					{#each row as cell}
																					<td>{@html parseMarkdown(typeof cell === 'string' ? cell : String(cell))}</td>
																					{/each}
																				{:else}
																					<td colspan="{tableData.headers.length}">Invalid row data: {typeof row === 'string' ? row : String(row)}</td>
																				{/if}
																			</tr>
																			{/if}
																		{/each}
																	</tbody>
																</table>
															</div>
															{#if isLongTable}
																<button class="table-toggle-btn glass glass--small glass--responsive" on:click={() => toggleTableExpansion(tableKey)}>
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
							{#if message.sender === 'assistant'}
								<div class="message-actions">
									<button 
										class="copy-btn glass glass--small glass--responsive {copiedMessageId === message.message_id ? 'copied' : ''}" 
										on:click={() => copyMessageToClipboard(message)}
									>
										{#if copiedMessageId === message.message_id}
											<svg viewBox="0 0 24 24" width="14" height="14">
												<path d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z" fill="currentColor" />
											</svg>
										{:else}
											<svg viewBox="0 0 24 24" width="14" height="14">
												<path d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z" fill="currentColor" />
											</svg>
										{/if}
									</button>
								</div>
							{/if}
							{#if message.suggestedQueries && message.suggestedQueries.length > 0}
								<div class="suggested-queries">
									{#each message.suggestedQueries as query}
																		<button 
									class="suggested-query-btn glass glass--rounded glass--responsive" 
									on:click={() => handleSuggestedQueryClick(query)}
								>
											{query}
										</button>
									{/each}
								</div>
							{/if}
						{/if}
					</div>

					<!-- Edit button for user messages - outside the message div -->
					{#if message.sender === 'user' && editingMessageId !== message.message_id}
						<div class="message-actions">
							<button 
								class="copy-btn glass glass--small glass--responsive {copiedMessageId === message.message_id ? 'copied' : ''}" 
								on:click={() => copyMessageToClipboard(message)}
							>
								{#if copiedMessageId === message.message_id}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z" fill="currentColor" />
									</svg>
								{:else}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z" fill="currentColor" />
									</svg>
								{/if}
							</button>
							<button class="edit-btn glass glass--small glass--responsive" on:click={() => startEditing(message)}>
								<svg viewBox="0 0 24 24" width="14" height="14">
									<path d="M20.71,7.04C21.1,6.65 21.1,6 20.71,5.63L18.37,3.29C18,2.9 17.35,2.9 16.96,3.29L15.12,5.12L18.87,8.87M3,17.25V21H6.75L17.81,9.93L14.06,6.18L3,17.25Z" fill="currentColor" />
								</svg>
							</button>
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	{#if !isPublicViewing}
		<div class="chat-input-wrapper">
			{#if $showChips && topChips.length}
			  <div class="chip-row">
			    {#each initialSuggestions as q, i}
			      {#if i < 3 || showAllInitialSuggestions}
			      					<button class="chip suggestion-chip glass glass--pill glass--responsive" on:click={() => handleSuggestedQueryClick(q)}>
			        <kbd>{i + 1}</kbd> {q}
			      </button>
			      {/if}
			    {/each}
			    {#if initialSuggestions.length > 3 && !showAllInitialSuggestions}
			      <button class="chip suggestion-chip glass glass--pill glass--responsive more" on:click={() => showAllInitialSuggestions = true}>⋯ More</button>
			    {/if}
			  </div>
			{/if}

		<div class="input-area-wrapper">
			{#if $contextItems.length > 0}
				<div class="context-chips">
					{#each $contextItems as item (
						item.securityId + '-' + ('filingType' in item ? item.link : item.timestamp)
					)}
						{@const isFiling = 'filingType' in item}
						<button
							type="button"
							class="chip glass glass--pill glass--responsive"
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

			<div class="input-field-container glass glass--rounded glass--responsive">
				<textarea
					class="chat-input"
					placeholder="Ask anything..."
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
						<svg viewBox="0 0 18 18" class="send-icon">
							<path d="M7.99992 14.9993V5.41334L4.70696 8.70631C4.31643 9.09683 3.68342 9.09683 3.29289 8.70631C2.90237 8.31578 2.90237 7.68277 3.29289 7.29225L8.29289 2.29225L8.36906 2.22389C8.76184 1.90354 9.34084 1.92613 9.70696 2.29225L14.707 7.29225L14.7753 7.36842C15.0957 7.76119 15.0731 8.34019 14.707 8.70631C14.3408 9.07242 13.7618 9.09502 13.3691 8.77467L13.2929 8.70631L9.99992 5.41334V14.9993C9.99992 15.5516 9.55221 15.9993 8.99992 15.9993C8.44764 15.9993 7.99993 15.5516 7.99992 14.9993Z" />
						</svg>
					{/if}
				</button>
			</div>
		</div>
		</div>
	{:else}
		<!-- Public viewing message -->
		<div class="public-viewing-notice">
			<p>You are viewing a shared conversation. <a href="/app">Sign in</a> to start your own chat.</p>
		</div>
	{/if}

	<!-- Share Modal -->
	{#if showShareModal}
		<div class="share-modal-popup glass glass--rounded glass--responsive" bind:this={shareModalRef}>
			<div class="share-modal-header">
				<h4>Share Conversation</h4>
				<button class="close-btn" on:click={closeShareModal} aria-label="Close">
					<svg viewBox="0 0 24 24" width="16" height="16">
						<path d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z" fill="currentColor" />
					</svg>
				</button>
			</div>
			
			<div class="share-modal-content">
				{#if shareLoading}
					<div class="share-loading">
						<p>Generating share link...</p>
					</div>
				{:else if shareLink}
					<div class="share-link-container">
						<p class="share-description">
							Anyone with this link can view this conversation.
						</p>
						
						<div class="share-link-field">
							<input 
								type="text" 
								value={shareLink} 
								readonly 
								class="share-link-input"
							/>
							<button 
								class="copy-link-btn glass glass--small glass--responsive {shareCopied ? 'copied' : ''}"
								on:click={copyShareLink}
							>
								{#if shareCopied}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z" fill="currentColor" />
									</svg>
									Copied!
								{:else}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z" fill="currentColor" />
									</svg>
									Copy
								{/if}
							</button>
						</div>
					</div>
				{:else}
					<div class="share-error">
						<p>Failed to generate share link. Please try again.</p>
						<button class="retry-btn glass glass--small glass--responsive" on:click={handleShareConversation}>
							Retry
						</button>
					</div>
				{/if}
			</div>
		</div>
	{/if	}
</div>
