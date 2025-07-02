<script lang="ts">
	import { onMount, tick, onDestroy } from 'svelte';
	import { privateRequest, publicRequest, streamingChatRequest } from '$lib/utils/helpers/backend';
	import { marked } from 'marked'; // Import the markdown parser
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
		pendingChatQuery
	} from './interface';
	import type {
		Message,
		ConversationData,
		QueryResponse,
		TableData,
		ContentChunk,
		PlotData,
		TimelineEvent
	} from './interface';
	import {
		parseMarkdown,
		formatChipDate,
		formatRuntime,
		cleanHtmlContent,
		handleTickerButtonClick,
		cleanContentChunk,
		getContentChunkTextForCopy
	} from './utils';
	import { isPlotData, getPlotData, plotDataToText, generatePlotKey } from './plotUtils';
	import { activeChartInstance } from '$lib/features/chart/interface';
	import {
		functionStatusStore,
		titleUpdateStore,
		type FunctionStatusUpdate,
		type TitleUpdate,
		sendChatQuery
	} from '$lib/utils/stream/socket'; // Import both stores and types
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

	// Title typing effect state
	let isTypingTitle = false;
	let typingTitleText = '';
	let typingTitleTarget = '';
	let typingInterval: ReturnType<typeof setInterval> | null = null;

	import ConversationHeader from './components/ConversationHeader.svelte';
	import PlotChunk from './components/PlotChunk.svelte';
	import ShareModal from './components/ShareModal.svelte';
	import MessageTimeline from './components/MessageTimeline.svelte';
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

	// State for table expansion
	let tableExpansionStates: { [key: string]: boolean } = {};

	import type { SortState } from './interface';

	let tableSortStates: { [key: string]: SortState } = {};

	// Chat history
	let messagesStore = writable<Message[]>([]); // Wrap messages in a writable store
	let historyLoaded = false; // Add state variable

	// Derived store to control initial chip visibility
	const showChips = derived(
		[inputValue, messagesStore],
		([$val, $msgs]) => $msgs.length === 0 && $val.trim() === ''
	);

	// Reactive variable for top 3 suggestions
	$: topChips = initialSuggestions.slice(0, 3);

	// State for showing all initial suggestions
	let showAllInitialSuggestions = false;

	// Add abort controller for cancelling requests
	let currentAbortController: AbortController | null = null;
	let currentWebSocketCancel: (() => void) | null = null;
	let requestCancelled = false;
	let currentProcessingQuery = ''; // Track the query currently being processed

	// Message editing state
	let editingMessageId = '';
	let editingContent = '';

	// Copy feedback state
	let copiedMessageId = '';
	let copyTimeout: ReturnType<typeof setTimeout> | null = null;

	// Retry popup state
	let showRetryPopup = '';
	let retryPopupRef: HTMLDivElement;

	// Share modal reference
	let shareModalRef: ShareModal;

	// Processing timeline state
	let processingTimeline: TimelineEvent[] = [];
	let isProcessingMessage = false;
	let lastStatusMessage = '';
	let showTimelineDropdown = false; // State for timeline dropdown visibility

	// Function to fetch initial suggestions based on active chart
	async function fetchInitialSuggestions() {
		initialSuggestions = []; // Clear previous suggestions first
		if ($activeChartInstance) {
			// Only fetch if there's an active instance
			try {
				const response = await privateRequest<{ suggestions: string[] }>(
					'getInitialQuerySuggestions',
					{
						activeChartInstance: $activeChartInstance
					}
				);
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
			setTimeout(() => {
				if (queryInput) {
					queryInput.focus();
				}
			}, 100);
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

		// Immediately update UI for snappy feel
		const previousConversationId = currentConversationId;
		const previousTitle = currentConversationTitle;
		const previousMessages = [...$messagesStore];
		const previousContext = [...$contextItems];

		currentConversationId = conversationId;
		currentConversationTitle = title;
		showConversationDropdown = false;

		// Clear current messages and context items immediately
		messagesStore.set([]);
		contextItems.set([]);

		// Reset historyLoaded to show skeleton during conversation switch
		historyLoaded = false;

		// Make backend request and load messages in background
		try {
			const response = await privateRequest('switchConversation', {
				conversation_id: conversationId
			});

			if (response) {
				// Load the conversation messages and scroll to bottom
				await loadConversationHistory(true);
			} else {
				// If backend fails, restore previous state
				currentConversationId = previousConversationId;
				currentConversationTitle = previousTitle;
				messagesStore.set(previousMessages);
				contextItems.set(previousContext);
			}
		} catch (error) {
			console.error('Error switching conversation:', error);

			// Restore previous state on error
			currentConversationId = previousConversationId;
			currentConversationTitle = previousTitle;
			messagesStore.set(previousMessages);
			contextItems.set(previousContext);
		}
	}

	async function deleteConversation(conversationId: string, event: MouseEvent) {
		event.stopPropagation(); // Prevent switching to the conversation

		// Instead of confirm dialog, set the conversation to delete mode
		conversationToDelete = conversationId;
	}

	async function confirmDeleteConversation(conversationId: string) {
		// Store the conversation being deleted for potential restoration
		const conversationToDeleteData = conversations.find(
			(c) => c.conversation_id === conversationId
		);

		// Immediately remove from UI for snappy feel
		conversations = conversations.filter((c) => c.conversation_id !== conversationId);
		conversationToDelete = ''; // Clear delete mode immediately

		// If we deleted the current conversation, start a new one immediately
		if (conversationId === currentConversationId) {
			createNewConversation(); // This will clear the UI state
		}

		// Make backend request in background
		try {
			await privateRequest('deleteConversation', {
				conversation_id: conversationId
			});
			// Success - no need to do anything as UI is already updated
		} catch (error) {
			console.error('Error deleting conversation:', error);

			// Restore the conversation on error if we have the data
			if (conversationToDeleteData) {
				// Insert back into conversations list in the right position
				const updatedConversations = [...conversations, conversationToDeleteData];
				// Sort by updated_at to maintain proper order
				conversations = updatedConversations.sort(
					(a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
				);
			}

			// Could show a toast notification here about the error
			// For now, just log the error
		}
	}

	function cancelDeleteConversation() {
		conversationToDelete = ''; // Clear delete mode
	}

	function toggleConversationDropdown() {
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

		// Don't handle clicks on the retry button itself
		if (target && target.closest('.retry-container')) {
			return;
		}

		if (
			showConversationDropdown &&
			conversationDropdown &&
			!conversationDropdown.contains(event.target as Node)
		) {
			showConversationDropdown = false;
		}
		// Close retry popup when clicking outside
		if (showRetryPopup && retryPopupRef && !retryPopupRef.contains(event.target as Node)) {
			closeRetryPopup();
		}
	}

	// Load any existing conversation history from the server
	async function loadConversationHistory(shouldAutoScroll: boolean = true) {
		try {
			isLoading = true;
			let response;
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

				// Process each message in the conversation history
				conversation.messages.forEach((msg) => {
					const msgTimestamp = new Date(msg.timestamp);
					const msgCompletedAt = msg.completed_at ? new Date(msg.completed_at) : undefined;
					const isCompleted = msg.status === 'completed';
					const isPending = msg.status === 'pending';

					// Only process messages that have backend message IDs
					if (!msg.message_id) {
						console.warn('Skipping message without backend message_id:', msg);
						return;
					}

					// Now we know msg.message_id is defined (TypeScript will recognize this)
					const messageId = msg.message_id;
					messagesStore.update((current) => [
						...current,
						{
							message_id: messageId, // Use backend ID
							content: msg.query,
							sender: 'user',
							timestamp: msgTimestamp,
							contextItems: msg.context_items || [],
							status: msg.status,
							completedAt: msgCompletedAt
						}
					]);

					// Only add assistant message if it's completed (has content)
					if (isCompleted && (msg.content_chunks || msg.response_text)) {
						// Assistant messages use a different ID pattern since they're not editable
						const assistantMessageId = messageId + '_response';
						messagesStore.update((current) => [
							...current,
							{
								message_id: assistantMessageId,
								sender: 'assistant',
								content: msg.response_text || '',
								contentChunks: msg.content_chunks || [],
								timestamp: msgTimestamp,
								suggestedQueries: msg.suggested_queries || [],
								status: msg.status,
								completedAt: msgCompletedAt
							}
						]);
					} else if (isPending) {
						// Add a loading message for pending requests
						const loadingMessageId = messageId + '_loading';
						messagesStore.update((current) => [
							...current,
							{
								message_id: loadingMessageId,
								sender: 'assistant',
								content: '',
								timestamp: msgTimestamp,
								isLoading: true,
								status: 'pending'
							}
						]);
					}
				});

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
					// Use requestAnimationFrame to ensure scrolling happens after all rendering is complete
					requestAnimationFrame(() => {
						scrollToBottomImmediate();
						// Double-check with a small delay to handle any late-rendering content
						setTimeout(() => {
							scrollToBottomImmediate();
						}, 50);
					});
				}
			} else {
				messagesStore.set([]);

				if (!currentConversationId) {
					currentConversationTitle = 'Chat';
				}
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

	onMount(() => {
		if (queryInput && !isPublicViewing) {
			setTimeout(() => {
				if (queryInput) {
					queryInput.focus();
				}
			}, 100);
		}
		// Auto-scroll to bottom for authenticated users, start at top for shared chats
		loadConversationHistory(!isPublicViewing);
		loadConversations(); // Load conversations on mount (will be skipped for public viewing)

		document.addEventListener('click', handleClickOutside); // Add click outside listener

		// Add delegated event listener for ticker buttons
		if (messagesContainer) {
			messagesContainer.addEventListener('click', handleTickerButtonClick);
		}

		// Cleanup listener on component destroy
		return () => {
			document.removeEventListener('click', handleClickOutside); // Clean up click outside listener
			if (messagesContainer) {
				messagesContainer.removeEventListener('click', handleTickerButtonClick);
			}
		};
	});

	onDestroy(() => {
		// Clean up copy timeout
		if (copyTimeout) {
			clearTimeout(copyTimeout);
		}

		// Clean up typing interval
		if (typingInterval) {
			clearInterval(typingInterval);
		}
	});

	// Scroll to bottom of chat (for user-initiated actions)
	function scrollToBottom() {
		setTimeout(() => {
			if (messagesContainer) {
				messagesContainer.style.scrollBehavior = 'smooth';
				messagesContainer.scrollTop = messagesContainer.scrollHeight;
			}
		}, 100);
	}

	// Helper function for immediate scrolling to bottom (used when loading conversations)
	function scrollToBottomImmediate() {
		if (messagesContainer) {
			messagesContainer.style.scrollBehavior = 'auto';
			messagesContainer.scrollTop = messagesContainer.scrollHeight;
			messagesContainer.style.scrollBehavior = 'smooth';
		}
	}

	// Function to handle clicking on a suggested query
	function handleSuggestedQueryClick(query: string) {
		if (isPublicViewing) {
			showAuthModal('conversations', 'signup');
			return;
		}
		inputValue.set(query);
		handleSubmit();
	}

	async function handleSubmit() {
		if (!$inputValue.trim() || isLoading) return;

		isLoading = true;
		let loadingMessage: Message | null = null;
		let backendConversationId = ''; // Track the conversation ID from backend

		// Create new abort controller for this request
		currentAbortController = new AbortController();

				// Note: Create a temporary user message for immediate UI feedback
		// The backend will provide the actual message ID, but we need something for the UI
		const userMessage: Message = {
			message_id: 'temp_' + Date.now(), // Temporary ID for UI only
			content: $inputValue,
			sender: 'user',
			timestamp: new Date(),
			contextItems: [...$contextItems],
			status: 'pending'
		};

		try {
			// Initialize processing timeline
			processingTimeline = [
				{
					message: 'Message sent to server',
					timestamp: new Date()
				}
			];

			isProcessingMessage = true;
			lastStatusMessage = ''; // Reset last status message
			showTimelineDropdown = false; // Reset dropdown state for new message

			messagesStore.update((current) => [...current, userMessage]);

			// Create loading message placeholder
			loadingMessage = {
				message_id: 'temp_loading_' + Date.now(),
				content: '', // Content is now handled by the store
				sender: 'assistant',
				timestamp: new Date(),
				isLoading: true
			};
			messagesStore.update((current) => [...current, loadingMessage as Message]);

			// <-- Set initial status immediately -->
			functionStatusStore.set({
				type: 'function_status',
				userMessage: 'Thinking...'
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
				const { promise, cancel } = sendChatQuery(
					finalQuery,
					$contextItems, // Send only manually added context items
					currentActiveChart, // Send active chart separately
					currentConversationId || '' // Send empty string for new chats
				);

				// Store the cancel function
				currentWebSocketCancel = cancel;

				const response = await promise;

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
						const newConversation = conversations.find(
							(c) => c.conversation_id === currentConversationId
						);
						if (newConversation) {
							currentConversationTitle = newConversation.title;
						}
					}
				}

				// Update the temporary user message with the real backend message ID
				if (typedResponse.message_id) {
					messagesStore.update((current) =>
						current.map((msg) =>
							msg.message_id === userMessage.message_id
								? { ...msg, message_id: typedResponse.message_id! }
								: msg
						)
					);
				}

				// Use timestamps from backend response if available, otherwise use current time
				const messageTimestamp = typedResponse.timestamp
					? new Date(typedResponse.timestamp)
					: new Date();
				const messageCompletedAt = typedResponse.completed_at
					? new Date(typedResponse.completed_at)
					: new Date();

				const assistantMessage: Message = {
					message_id: typedResponse.message_id + '_response' || '',
					content: typedResponse.text || 'Error processing request.',
					sender: 'assistant',
					timestamp: messageTimestamp,
					contentChunks: typedResponse.content_chunks,
					suggestedQueries: typedResponse.suggestions || [],
					status: 'completed',
					completedAt: messageCompletedAt
				};

				messagesStore.update((current) => [...current, assistantMessage]);

				// Clear processing state
				isProcessingMessage = false;
				processingTimeline = [];
				lastStatusMessage = '';

				// If we didn't have a conversation ID before, we should have one now
				// Load conversation history to get the new conversation ID
				if (!currentConversationId) {
					await loadConversationHistory(false); // Don't scroll since we just added the message
					await loadConversations(); // Refresh conversation list
				}
			} catch (error: any) {
				// Check if the request was cancelled (either by AbortController or by our cancellation response)
				if (requestCancelled || error.cancelled === true) {
					functionStatusStore.set(null);
					// Try to clean up pending message on backend
					await cleanupPendingMessage(currentProcessingQuery);
					return;
				}

				console.error('Error fetching response:', error);

				// Clear status store on error
				functionStatusStore.set(null);

				// Check if the error contains backend response data with messageID and conversationID
				let errorMessageId = 'temp_error_' + Date.now();
				let shouldUpdateUserMessage = false;
				let shouldReloadConversations = false;
				
				// Try to extract backend response data from error
				if (error.response && typeof error.response === 'object') {
					if (error.response.message_id) {
						shouldUpdateUserMessage = true;
						errorMessageId = error.response.message_id + '_response';
					}
					if (error.response.conversation_id && !currentConversationId) {
						currentConversationId = error.response.conversation_id;
						backendConversationId = error.response.conversation_id;
						shouldReloadConversations = true;
					}
				}

				// Update the temporary user message with real backend message ID if available
				if (shouldUpdateUserMessage && error.response?.message_id) {
					messagesStore.update((current) =>
						current.map((msg) =>
							msg.message_id === userMessage.message_id
								? { ...msg, message_id: error.response.message_id, status: 'error' }
								: msg
						)
					);
				}

				// Try to clean up pending message on backend for network errors (only if we don't have a real message ID)
				if (!shouldUpdateUserMessage) {
					await cleanupPendingMessage(currentProcessingQuery);
				}

				const errorMessage: Message = {
					message_id: errorMessageId,
					content: `Error: ${error.message || 'Failed to get response'}`,
					sender: 'assistant',
					timestamp: new Date(),
					status: 'error'
				};

				messagesStore.update((current) => [...current, errorMessage]);
			}
		} catch (error: any) {
			console.error('Error in handleSubmit:', error);

			// Clear status store on any error
			functionStatusStore.set(null);

			// Check if the error contains backend response data with messageID and conversationID
			let errorMessageId = 'temp_error_' + Date.now();
			let shouldUpdateUserMessage = false;
			let shouldReloadConversations = false;
			
			// Try to extract backend response data from error
			if (error.response && typeof error.response === 'object') {
				if (error.response.message_id) {
					shouldUpdateUserMessage = true;
					errorMessageId = error.response.message_id + '_response';
				}
				if (error.response.conversation_id && !currentConversationId) {
					currentConversationId = error.response.conversation_id;
					backendConversationId = error.response.conversation_id;
					shouldReloadConversations = true;
				}
			}

			// Update the temporary user message with real backend message ID if available
			if (shouldUpdateUserMessage && error.response?.message_id) {
				messagesStore.update((current) =>
					current.map((msg) =>
						msg.message_id === userMessage.message_id
							? { ...msg, message_id: error.response.message_id, status: 'error' }
							: msg
					)
				);
			}

			// Try to clean up pending message on backend (only if we don't have a real message ID)
			if (!shouldUpdateUserMessage) {
				await cleanupPendingMessage(currentProcessingQuery);
			}

			// Add error message if we have a loading message
			if (loadingMessage) {
				const errorMessage: Message = {
					message_id: errorMessageId,
					content: `Error: ${error.message || 'An unexpected error occurred'}`,
					sender: 'assistant',
					timestamp: new Date(),
					status: 'error'
				};

				messagesStore.update((current) => [...current, errorMessage]);
			}
		} finally {
			// Always clean up loading message and reset state
			if (loadingMessage) {
				messagesStore.update((current) =>
					current.filter((m) => m.message_id !== loadingMessage!.message_id)
				);
			}

			// Clear processing state immediately on error
			isProcessingMessage = false;
			processingTimeline = [];
			lastStatusMessage = '';

			isLoading = false;
			currentAbortController = null;
			currentWebSocketCancel = null;
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

			// Cancel WebSocket request if active
			if (currentWebSocketCancel) {
				currentWebSocketCancel();
				currentWebSocketCancel = null;
			}

			// Abort HTTP request if active (fallback)
			if (currentAbortController) {
				currentAbortController.abort();
			}

			// Clean up pending message on backend using the tracked query
			if (currentProcessingQuery) {
				await cleanupPendingMessage(currentProcessingQuery);
			}

			// Remove any loading messages
			messagesStore.update((current) => current.filter((m) => !m.isLoading));

			// Clear processing state immediately on cancellation
			isProcessingMessage = false;
			processingTimeline = [];
			lastStatusMessage = '';

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

	// Function to safely access table data properties
	function isTableData(content: any): content is TableData {
		return (
			typeof content === 'object' &&
			content !== null &&
			Array.isArray(content.headers) &&
			Array.isArray(content.rows) &&
			content.rows.every((row: any) => Array.isArray(row))
		); // Ensure each row is also an array
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
				const numA = Number(String(valA).replace(/[^0-9.-]+/g, ''));
				const numB = Number(String(valB).replace(/[^0-9.-]+/g, ''));

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
		messagesStore.update((current) =>
			current.map((msg) => {
				if (msg.contentChunks) {
					msg.contentChunks = msg.contentChunks.map((chunk, idx) => {
						const currentTableKey = msg.message_id + '-' + idx;
						if (
							currentTableKey === tableKey &&
							chunk.type === 'table' &&
							isTableData(chunk.content)
						) {
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
			})
		);
	}

	// Reactive block to handle pending query
	$: if ($pendingChatQuery && browser && historyLoaded) {
		const queryData = $pendingChatQuery;
		pendingChatQuery.set(null); // Clear the pending query immediately to prevent re-triggering

		// Add context items (preventing duplicates)
		contextItems.update((currentItems) => {
			const newItems = queryData.context.filter(
				(newItem: Instance | FilingContext) =>
					!currentItems.some(
						(existingItem) =>
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
				textToCopy = message.contentChunks
					.map((chunk) => getContentChunkTextForCopy(chunk, isTableData, plotDataToText))
					.filter((text) => text.length > 0)
					.join('\n\n');
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
			const editingMessage = $messagesStore.find((msg) => msg.message_id === editingMessageId);

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
				contextItems.update((currentItems) => {
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

	function showRetryOptions(message: Message) {
		showRetryPopup = message.message_id;
	}

	function closeRetryPopup() {
		showRetryPopup = '';
	}

	async function retryMessage(message: Message) {
		// Find the corresponding user message to retry
		let userMessage: Message | undefined;

		if (message.sender === 'assistant') {
			// For assistant messages (including errors), find the preceding user message
			const messageIndex = $messagesStore.findIndex((msg) => msg.message_id === message.message_id);
			if (messageIndex !== -1) {
				// Look backwards for the most recent user message
				for (let i = messageIndex - 1; i >= 0; i--) {
					if ($messagesStore[i].sender === 'user') {
						userMessage = $messagesStore[i];
						break;
					}
				}
			}
		} else if (message.sender === 'user') {
			// If called from user message directly
			userMessage = message;
		}

		if (!userMessage) {
			console.error('Could not find user message to retry');
			return;
		}

		try {
			// Always use backend retry API
			const response = await privateRequest('retryMessage', {
				conversation_id: currentConversationId,
				message_id: userMessage.message_id
			});

			if (response && (response as any).success) {
				// Update conversation ID if needed
				if ((response as any).conversation_id) {
					currentConversationId = (response as any).conversation_id;
				}

				// Reload conversation history to reflect the archived state
				await loadConversationHistory(false);
				await tick();

				// Set the input value and context from the retry response
				inputValue.set((response as any).original_query || '');
				contextItems.update((currentItems) => {
					return (response as any).context_items || [];
				});

				await tick();
				handleSubmit();
			} else {
				console.error('Backend retry failed or success=false');
				await loadConversationHistory(false);
			}
		} catch (error) {
			console.error('Error retrying message:', error);
			await loadConversationHistory(false);
		}
	}

	// Share conversation functions
	async function handleShareConversation(event?: Event) {
		// Stop event propagation to prevent immediate closing
		if (event) {
			event.stopPropagation();
			event.preventDefault();
		}

		// Close conversation dropdown if it's open
		if (showConversationDropdown) {
			showConversationDropdown = false;
		}

		// Toggle the share modal
		if (shareModalRef) {
			shareModalRef.toggleModal();
		}
	}

	// Function to create typing effect for title
	function startTitleTypingEffect(newTitle: string) {
		// Don't start typing if already typing or if the title is the same
		if (isTypingTitle || currentConversationTitle === newTitle) {
			return;
		}

		isTypingTitle = true;
		typingTitleTarget = newTitle;
		typingTitleText = '';

		let currentIndex = 0;
		const typingSpeed = 50; // milliseconds per character

		// Clear any existing interval
		if (typingInterval) {
			clearInterval(typingInterval);
		}

		typingInterval = setInterval(() => {
			if (currentIndex < typingTitleTarget.length) {
				typingTitleText = typingTitleTarget.substring(0, currentIndex + 1);
				currentIndex++;
			} else {
				// Typing complete
				clearInterval(typingInterval!);
				typingInterval = null;
				isTypingTitle = false;
				currentConversationTitle = typingTitleTarget;
				typingTitleText = '';
			}
		}, typingSpeed);
	}

	// Reactive block to handle title updates from websocket
	$: if ($titleUpdateStore && browser) {
		const titleUpdate = $titleUpdateStore;

		// Handle title updates for current conversation OR new conversations (when currentConversationId is empty)
		if (
			titleUpdate.conversation_id === currentConversationId ||
			(!currentConversationId && titleUpdate.conversation_id)
		) {
			// If this is a new conversation, set the conversation ID
			if (!currentConversationId) {
				currentConversationId = titleUpdate.conversation_id;
			}

			// Start typing effect for the new title
			startTitleTypingEffect(titleUpdate.title);

			// Also update the conversations list if it's loaded
			if (conversations.length > 0) {
				conversations = conversations.map((conv) =>
					conv.conversation_id === titleUpdate.conversation_id
						? { ...conv, title: titleUpdate.title }
						: conv
				);
			}
		}

		// Clear the store after processing
		titleUpdateStore.set(null);
	}

	// Reactive block to capture function status messages and build timeline
	$: if ($functionStatusStore && browser && isProcessingMessage) {
		const statusUpdate = $functionStatusStore;

		// Only add if this is a new message different from the last one
		if (statusUpdate.userMessage && statusUpdate.userMessage !== lastStatusMessage) {
			lastStatusMessage = statusUpdate.userMessage;

			// Add new timeline event with the raw message from backend
			processingTimeline = [
				...processingTimeline,
				{
					message: statusUpdate.userMessage,
					timestamp: new Date()
				}
			];
		}
	}
</script>

<div class="chat-container">
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
		{isTypingTitle}
		{typingTitleText}
		{isPublicViewing}
		{sharedConversationId}
		{toggleConversationDropdown}
		{createNewConversation}
		{switchToConversation}
		{deleteConversation}
		{confirmDeleteConversation}
		{cancelDeleteConversation}
		{handleShareConversation}
	/>

	<div class="chat-messages" bind:this={messagesContainer}>
		{#if isLoading && !historyLoaded}
			<!-- Show skeleton loading for conversation history -->
			<div class="chat-skeleton">
				<div class="skeleton-shimmer"></div>
			</div>
		{:else if $messagesStore.length === 0}
			<!-- Only show the container and header when chat is empty -->
			<div class="initial-container">
				<!-- Capabilities text merged here -->
				<p class="capabilities-text">
					Chat is a powerful interface for analyzing market data, filings, news, backtesting
					strategies, and more. It can answer questions and perform tasks.
				</p>
				<p class="suggestions-header">
					Ask Atlantis a question or to perform a task to get started.
				</p>
			</div>
		{:else}
			{#each $messagesStore as message (message.message_id)}
				<div class="message-wrapper {message.sender}">
					<div
						class="message {message.sender} {message.status === 'error' ||
						message.content.includes('Error:')
							? 'error'
							: ''} {editingMessageId === message.message_id ? 'editing' : ''} {message.sender ===
						'user'
							? 'glass glass--pill glass--responsive'
							: ''}"
					>
						{#if message.isLoading}
							<!-- Always show the current status message at top with dropdown toggle -->
							<div class="loading-status-container">
								<p class="loading-status">{$functionStatusStore?.userMessage || 'Thinking...'}</p>
								{#if isProcessingMessage && processingTimeline.length > 1}
									<button
										class="timeline-dropdown-toggle"
										on:click={() => (showTimelineDropdown = !showTimelineDropdown)}
										aria-label={showTimelineDropdown ? 'Hide timeline' : 'Show timeline'}
									>
										<svg
											viewBox="0 0 24 24"
											width="14"
											height="14"
											class="chevron-icon {showTimelineDropdown ? 'expanded' : ''}"
										>
											<path
												d="M7.41,8.58L12,13.17L16.59,8.58L18,10L12,16L6,10L7.41,8.58Z"
												fill="currentColor"
											/>
										</svg>
									</button>
								{/if}
							</div>

							<!-- Show timeline below if we have relevant timeline data and dropdown is open -->
							{#if isProcessingMessage && processingTimeline.length > 1 && showTimelineDropdown}
								<MessageTimeline timeline={processingTimeline} />
							{/if}
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
									<button
										class="edit-cancel-btn glass glass--small glass--responsive"
										on:click={cancelEditing}
									>
										Cancel
									</button>
									<button
										class="edit-save-btn glass glass--small glass--responsive"
										on:click={saveMessageEdit}
									>
										Send
									</button>
								</div>
							</div>
						{:else}
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
													{@html parseMarkdown(
														typeof chunk.content === 'string'
															? chunk.content
															: String(chunk.content)
													)}
												</div>
											{:else if chunk.type === 'table'}
												{#if isTableData(chunk.content)}
													{@const tableData = getTableData(chunk.content)}
													{@const tableKey = message.message_id + '-' + index}
													{@const isLongTable = tableData && tableData.rows.length > 5}
													{@const isExpanded = tableExpansionStates[tableKey] === true}
													{@const currentSort = tableSortStates[tableKey] || {
														columnIndex: null,
														direction: null
													}}

													{#if tableData}
														<div class="chunk-table-container">
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
																					on:click={() =>
																						sortTable(
																							tableKey,
																							colIndex,
																							JSON.parse(JSON.stringify(tableData))
																						)}
																					class:sortable={true}
																					class:sorted={currentSort.columnIndex === colIndex}
																					class:asc={currentSort.columnIndex === colIndex &&
																						currentSort.direction === 'asc'}
																					class:desc={currentSort.columnIndex === colIndex &&
																						currentSort.direction === 'desc'}
																				>
																					{@html parseMarkdown(
																						typeof header === 'string' ? header : String(header)
																					)}
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
																							<td
																								>{@html parseMarkdown(
																									typeof cell === 'string' ? cell : String(cell)
																								)}</td
																							>
																						{/each}
																					{:else}
																						<td colspan={tableData.headers.length}
																							>Invalid row data: {typeof row === 'string'
																								? row
																								: String(row)}</td
																						>
																					{/if}
																				</tr>
																			{/if}
																		{/each}
																	</tbody>
																</table>
															</div>
															{#if isLongTable}
																<button
																	class="table-toggle-btn glass glass--small glass--responsive"
																	on:click={() => toggleTableExpansion(tableKey)}
																>
																	{isExpanded
																		? 'Show less'
																		: `Show more (${tableData.rows.length} rows)`}
																</button>
															{/if}
														</div>
													{:else}
														<div class="chunk-error">Invalid table data structure</div>
													{/if}
												{:else}
													<div class="chunk-error">Invalid table data format</div>
												{/if}
											{:else if chunk.type === 'plot'}
												{@const cleanedChunk = cleanContentChunk(chunk)}
												{#if isPlotData(cleanedChunk.content)}
													{@const plotData = getPlotData(cleanedChunk.content)}
													{@const plotKey = generatePlotKey(message.message_id, index)}

													{#if plotData}
														<PlotChunk {plotData} {plotKey} />
													{:else}
														<div class="chunk-error">Invalid plot data structure</div>
													{/if}
												{:else}
													<div class="chunk-error">Invalid plot data format</div>
												{/if}
											{/if}
										{/each}
									</div>
								{:else}
									{@html parseMarkdown(message.content)}
								{/if}
							</div>

							<!-- Error message retry button -->
							{#if message.sender === 'assistant' && (message.status === 'error' || message.content.includes('Error:'))}
								<div class="error-retry-section">
									<button
										class="error-retry-btn glass glass--small glass--responsive"
										on:click={() => retryMessage(message)}
										title="Retry Message"
									>
										<svg
											viewBox="0 0 24 24"
											width="16"
											height="16"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
											stroke-linecap="round"
											stroke-linejoin="round"
										>
											<polyline points="23 4 23 10 17 10"></polyline>
											<polyline points="1 20 1 14 7 14"></polyline>
											<path
												d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"
											></path>
										</svg>
										Try again
									</button>
								</div>
							{/if}

							{#if message.sender === 'assistant' && !(message.status === 'error' || message.content.includes('Error:'))}
								<div class="message-actions">
									<button
										class="copy-btn glass glass--small glass--responsive {copiedMessageId ===
										message.message_id
											? 'copied'
											: ''}"
										on:click={() => copyMessageToClipboard(message)}
									>
										{#if copiedMessageId === message.message_id}
											<svg viewBox="0 0 24 24" width="14" height="14">
												<path
													d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z"
													fill="currentColor"
												/>
											</svg>
										{:else}
											<svg viewBox="0 0 24 24" width="14" height="14">
												<path
													d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z"
													fill="currentColor"
												/>
											</svg>
										{/if}
									</button>
									<div class="retry-container">
										<button
											class="retry-btn glass glass--small glass--responsive"
											on:click={() => showRetryOptions(message)}
											title="Retry Message"
										>
											<svg
												viewBox="0 0 24 24"
												width="14"
												height="14"
												fill="none"
												stroke="currentColor"
												stroke-width="2"
												stroke-linecap="round"
												stroke-linejoin="round"
											>
												<polyline points="23 4 23 10 17 10"></polyline>
												<polyline points="1 20 1 14 7 14"></polyline>
												<path
													d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"
												></path>
											</svg>
										</button>

										{#if showRetryPopup === message.message_id}
											<div
												class="retry-popup glass glass--rounded glass--responsive"
												bind:this={retryPopupRef}
											>
												<button
													class="retry-option"
													on:click={() => {
														retryMessage(message);
														closeRetryPopup();
													}}
												>
													<svg
														viewBox="0 0 24 24"
														width="16"
														height="16"
														fill="none"
														stroke="currentColor"
														stroke-width="2"
														stroke-linecap="round"
														stroke-linejoin="round"
													>
														<polyline points="23 4 23 10 17 10"></polyline>
														<polyline points="1 20 1 14 7 14"></polyline>
														<path
															d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"
														></path>
													</svg>
													Retry
												</button>
											</div>
										{/if}
									</div>
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
								class="copy-btn glass glass--small glass--responsive {copiedMessageId ===
								message.message_id
									? 'copied'
									: ''}"
								on:click={() => copyMessageToClipboard(message)}
							>
								{#if copiedMessageId === message.message_id}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path
											d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z"
											fill="currentColor"
										/>
									</svg>
								{:else}
									<svg viewBox="0 0 24 24" width="14" height="14">
										<path
											d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z"
											fill="currentColor"
										/>
									</svg>
								{/if}
							</button>
							<button
								class="edit-btn glass glass--small glass--responsive"
								on:click={() => startEditing(message)}
								title="Edit this message"
							>
								<svg viewBox="0 0 24 24" width="14" height="14">
									<path
										d="M20.71,7.04C21.1,6.65 21.1,6 20.71,5.63L18.37,3.29C18,2.9 17.35,2.9 16.96,3.29L15.12,5.12L18.87,8.87M3,17.25V21H6.75L17.81,9.93L14.06,6.18L3,17.25Z"
										fill="currentColor"
									/>
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
							<button
								class="chip suggestion-chip glass glass--pill glass--responsive"
								on:click={() => handleSuggestedQueryClick(q)}
							>
								<kbd>{i + 1}</kbd>
								{q}
							</button>
						{/if}
					{/each}
					{#if initialSuggestions.length > 3 && !showAllInitialSuggestions}
						<button
							class="chip suggestion-chip glass glass--pill glass--responsive more"
							on:click={() => (showAllInitialSuggestions = true)}>⋯ More</button
						>
					{/if}
				</div>
			{/if}

			<div class="input-area-wrapper">
				{#if $contextItems.length > 0}
					<div class="context-chips">
						{#each $contextItems as item (item.securityId + '-' + ('filingType' in item ? item.link : item.timestamp))}
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
						aria-label={isLoading ? 'Cancel request' : 'Send message'}
						disabled={!isLoading && !$inputValue.trim()}
					>
						{#if isLoading}
							<svg viewBox="0 0 24 24" class="send-icon pause-icon">
								<path d="M6,6H18V18H6V6Z" />
							</svg>
						{:else}
							<svg viewBox="0 0 18 18" class="send-icon">
								<path
									d="M7.99992 14.9993V5.41334L4.70696 8.70631C4.31643 9.09683 3.68342 9.09683 3.29289 8.70631C2.90237 8.31578 2.90237 7.68277 3.29289 7.29225L8.29289 2.29225L8.36906 2.22389C8.76184 1.90354 9.34084 1.92613 9.70696 2.29225L14.707 7.29225L14.7753 7.36842C15.0957 7.76119 15.0731 8.34019 14.707 8.70631C14.3408 9.07242 13.7618 9.09502 13.3691 8.77467L13.2929 8.70631L9.99992 5.41334V14.9993C9.99992 15.5516 9.55221 15.9993 8.99992 15.9993C8.44764 15.9993 7.99993 15.5516 7.99992 14.9993Z"
								/>
							</svg>
						{/if}
					</button>
				</div>
			</div>
		</div>
	{:else}
		<!-- Public viewing message -->
		<div class="public-viewing-notice">
			<p>
				You are viewing a shared conversation. <button
					class="auth-link"
					on:click={() => showAuthModal('conversations', 'login')}>Sign in</button
				> to start your own chat.
			</p>
		</div>
	{/if}

	<!-- Share Modal Component -->
	<ShareModal
		bind:this={shareModalRef}
		{currentConversationId}
		{sharedConversationId}
		{isPublicViewing}
	/>
</div>
