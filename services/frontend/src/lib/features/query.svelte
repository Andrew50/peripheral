<script lang="ts">
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { onMount } from 'svelte';
	import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import { marked } from 'marked'; // Import the markdown parser

	// Set default options for the markdown parser (optional)
	marked.setOptions({
		breaks: true, // Adds support for GitHub-flavored markdown line breaks
		gfm: true,    // GitHub-flavored markdown
		sanitize: false // HTML sanitization is handled by Svelte
	});

	// Function to parse markdown content
	function parseMarkdown(content: string): string {
		try {
			return marked.parse(content);
		} catch (error) {
			console.error('Error parsing markdown:', error);
			return content; // Fallback to plain text if parsing fails
		}
	}

	// Define types for our response data
	type FunctionResult = {
		function_name: string;
		result?: any;
		error?: string;
	};

	type QueryResponse = {
		type: 'text' | 'function_calls' | string;
		text?: string;
		results?: FunctionResult[];
		history?: any;
	};

	// Conversation history type
	type ConversationData = {
		messages: Array<{
			query: string;
			response_text: string;
			function_calls?: any[];
			tool_results?: FunctionResult[];
			timestamp: string | Date;
			expires_at?: string | Date; // When this message expires
		}>;
		timestamp: string | Date;
	};

	// Message type for chat history
	type Message = {
		id: string;
		content: string;
		sender: 'user' | 'assistant' | 'system';
		timestamp: Date;
		expiresAt?: Date; // When this message expires
		functionResults?: FunctionResult[];
		responseType?: string;
		isLoading?: boolean;
	};

	let inputValue = '';
	let queryInput: HTMLInputElement;
	let isLoading = false;
	let messagesContainer: HTMLDivElement;

	// Chat history
	let messages: Message[] = [];

	// Load any existing conversation history from the server
	async function loadConversationHistory() {
		try {
			isLoading = true;
			const response = await privateRequest('getUserConversation', {});

			// Check if we have a valid conversation history
			const conversation = response as ConversationData;
			if (conversation && conversation.messages && conversation.messages.length > 0) {
				// Process each message in the conversation history
				conversation.messages.forEach((msg) => {
					// Add user message
					messages.push({
						id: generateId(),
						content: msg.query,
						sender: 'user',
						timestamp: new Date(msg.timestamp),
						expiresAt: msg.expires_at ? new Date(msg.expires_at) : undefined
					});

					// Add assistant response
					messages.push({
						id: generateId(),
						content: msg.response_text,
						sender: 'assistant',
						timestamp: new Date(msg.timestamp),
						expiresAt: msg.expires_at ? new Date(msg.expires_at) : undefined,
						responseType: msg.function_calls?.length ? 'function_calls' : 'text',
						functionResults: msg.tool_results || []
					});
				});

				scrollToBottom();
			}
		} catch (error) {
			console.error('Error loading conversation history:', error);
			// If we can't load history, just continue with empty messages
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		if (queryInput) {
			setTimeout(() => queryInput.focus(), 100);
		}
		loadConversationHistory();
	});

	// Generate unique IDs for messages
	function generateId(): string {
		return Date.now().toString(36) + Math.random().toString(36).substring(2);
	}

	// Scroll to bottom of chat
	function scrollToBottom() {
		setTimeout(() => {
			if (messagesContainer) {
				messagesContainer.scrollTop = messagesContainer.scrollHeight;
			}
		}, 100);
	}

	function handleSubmit() {
		if (!inputValue.trim()) return;

		const userMessage: Message = {
			id: generateId(),
			content: inputValue,
			sender: 'user',
			timestamp: new Date()
		};

		messages = [...messages, userMessage];

		// Create loading message placeholder
		const loadingMessage: Message = {
			id: generateId(),
			content: '',
			sender: 'assistant',
			timestamp: new Date(),
			isLoading: true
		};

		messages = [...messages, loadingMessage];
		scrollToBottom();

		const queryText = inputValue;
		inputValue = '';

		privateRequest('getQuery', { query: queryText })
			.then((response) => {
				console.log('response', response);
				// Type assertion to handle the unknown response type
				const typedResponse = response as QueryResponse;
				console.log(typedResponse);

				// Remove loading message and add actual response
				messages = messages.filter((m) => m.id !== loadingMessage.id);

				// Check if the response includes expiration information
				let expiresAt: Date | undefined = undefined;
				if (
					typedResponse.history &&
					typedResponse.history.messages &&
					typedResponse.history.messages.length > 0
				) {
					const lastMessage =
						typedResponse.history.messages[typedResponse.history.messages.length - 1];
					if (lastMessage.expires_at) {
						expiresAt = new Date(lastMessage.expires_at);
					}
				}

				const assistantMessage: Message = {
					id: generateId(),
					content: typedResponse.text || 'Function calls executed successfully.',
					sender: 'assistant',
					timestamp: new Date(),
					expiresAt: expiresAt,
					responseType: typedResponse.type,
					functionResults:
						typedResponse.type === 'function_calls' ? typedResponse.results || [] : undefined
				};

				messages = [...messages, assistantMessage];
				scrollToBottom();
			})
			.catch((error) => {
				console.error('Error fetching response:', error);

				// Remove loading message and add error message
				messages = messages.filter((m) => m.id !== loadingMessage.id);

				const errorMessage: Message = {
					id: generateId(),
					content: `Error: ${error.message || 'Failed to get response'}`,
					sender: 'assistant',
					timestamp: new Date(),
					responseType: 'error'
				};

				messages = [...messages, errorMessage];
				scrollToBottom();
			});
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			handleSubmit();
		}
	}

	function formatTimestamp(date: Date): string {
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	// Function to determine if a message is near expiration (less than 6 hours)
	function isNearExpiration(message: Message): boolean {
		if (!message.expiresAt) return false;
		const now = new Date();
		const sixHoursMs = 6 * 60 * 60 * 1000;
		return message.expiresAt.getTime() - now.getTime() < sixHoursMs;
	}

	// Function to format expiration time
	function formatExpiration(expiresAt: Date): string {
		const now = new Date();
		const diff = expiresAt.getTime() - now.getTime();

		// Less than an hour
		if (diff < 60 * 60 * 1000) {
			const mins = Math.max(1, Math.floor(diff / (60 * 1000)));
			return `Expires in ${mins} minute${mins !== 1 ? 's' : ''}`;
		}

		// Hours
		if (diff < 24 * 60 * 60 * 1000) {
			const hours = Math.floor(diff / (60 * 60 * 1000));
			return `Expires in ${hours} hour${hours !== 1 ? 's' : ''}`;
		}

		// Days
		const days = Math.floor(diff / (24 * 60 * 60 * 1000));
		return `Expires in ${days} day${days !== 1 ? 's' : ''}`;
	}

	// Function to clear conversation history
	async function clearConversation() {
		try {
			isLoading = true;
			const response = await privateRequest('clearConversationHistory', {});
			
			// Clear local messages
			messages = [];
			
			// Optional: Show a temporary system message that history was cleared
			const systemMessage: Message = {
				id: generateId(),
				content: "Conversation history has been cleared.",
				sender: 'assistant',
				timestamp: new Date(),
				responseType: 'system'
			};
			
			messages = [systemMessage];
			
			// Remove the system message after a few seconds
			setTimeout(() => {
				if (messages.length === 1 && messages[0].id === systemMessage.id) {
					messages = [];
				}
			}, 3000);
			
		} catch (error) {
			console.error('Error clearing conversation history:', error);
			
			// Show error message
			const errorMessage: Message = {
				id: generateId(),
				content: `Error: Failed to clear conversation history`,
				sender: 'assistant',
				timestamp: new Date(),
				responseType: 'error'
			};
			
			messages = [...messages, errorMessage];
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="chat-container">
	<div class="chat-header">
		<h3>Chat</h3>
		{#if messages.length > 0}
			<button class="clear-button" on:click={clearConversation} disabled={isLoading}>
				<svg viewBox="0 0 24 24" width="16" height="16">
					<path d="M19,4H15.5L14.5,3H9.5L8.5,4H5V6H19M6,19A2,2 0 0,0 8,21H16A2,2 0 0,0 18,19V7H6V19Z" fill="currentColor" />
				</svg>
				Clear History
			</button>
		{/if}
	</div>

	<div class="chat-messages" bind:this={messagesContainer}>
		{#if messages.length === 0}
			<div class="empty-chat">
				<div class="empty-chat-icon">
					<svg viewBox="0 0 24 24" width="48" height="48">
						<path
							d="M20,2H4C2.9,2 2,2.9 2,4V22L6,18H20C21.1,18 22,17.1 22,16V4C22,2.9 21.1,2 20,2M20,16H5.2L4,17.2V4H20V16Z"
							fill="currentColor"
						/>
					</svg>
				</div>
				<p>No messages yet. Start a conversation!</p>
			</div>
		{:else}
			{#each messages as message (message.id)}
				<div class="message-wrapper {message.sender}">
					<div
						class="message {message.sender} {message.responseType === 'error'
							? 'error'
							: ''} {isNearExpiration(message) ? 'expiring' : ''}"
					>
						{#if message.isLoading}
							<div class="typing-indicator">
								<span></span>
								<span></span>
								<span></span>
							</div>
						{:else}
							<div class="message-content">
								<p>{@html parseMarkdown(message.content)}</p>
							</div>

							{#if message.functionResults && message.functionResults.length > 0}
								<div class="function-tools">
									{#each message.functionResults as result}
										<div class="function-tool {result.error ? 'error' : 'success'}">
											<div class="function-header">
												<div class="function-icon">
													<svg viewBox="0 0 24 24" width="16" height="16">
														<path
															d="M20,19V7H4V19H20M20,3C21.1,3 22,3.9 22,5V19C22,20.1 21.1,21 20,21H4C2.9,21 2,20.1 2,19V5C2,3.9 2.9,3 4,3H20M13,17V15H18V17H13M9.58,13L5.57,9H8.4L11.7,12.3C12.09,12.69 12.09,13.33 11.7,13.72L8.42,17H5.59L9.58,13Z"
															fill="currentColor"
														/>
													</svg>
												</div>
												<div class="function-name">{result.function_name}</div>
											</div>

											{#if result.error}
												<div class="function-error">
													<svg viewBox="0 0 24 24" width="16" height="16">
														<path
															d="M13,13H11V7H13M13,17H11V15H13M12,2A10,10 0 0,0 2,12A10,10 0 0,0 12,22A10,10 0 0,0 22,12A10,10 0 0,0 12,2Z"
															fill="currentColor"
														/>
													</svg>
													<span>{result.error}</span>
												</div>
											{:else}
												<div class="function-result-data">
													<pre>{JSON.stringify(result.result, null, 2)}</pre>
												</div>
											{/if}
										</div>
									{/each}
								</div>
							{/if}

							<div class="message-footer">
								<div class="message-timestamp">
									{formatTimestamp(message.timestamp)}
								</div>
								{#if message.expiresAt}
									<div class="message-expiration {isNearExpiration(message) ? 'expiring' : ''}">
										{formatExpiration(message.expiresAt)}
									</div>
								{/if}
							</div>
						{/if}
					</div>
				</div>
			{/each}
		{/if}
	</div>

	<div class="chat-input-wrapper">
		<input
			type="text"
			class="chat-input"
			placeholder="Type a message..."
			bind:value={inputValue}
			bind:this={queryInput}
			on:keydown={handleKeyDown}
		/>
		<button
			class="send-button"
			on:click={handleSubmit}
			aria-label="Send message"
			disabled={!inputValue.trim() || isLoading}
		>
			<svg viewBox="0 0 24 24" class="send-icon">
				<path d="M2,21L23,12L2,3V10L17,12L2,14V21Z" />
			</svg>
		</button>
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
		border-bottom: 1px solid var(--ui-border, #444);
		background: var(--ui-bg-element-darker, #2a2a2a);
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
		padding: 0.5rem 0.75rem;
		background: rgba(244, 67, 54, 0.1);
		color: var(--error-color, #f44336);
		border: 1px solid var(--error-color, #f44336);
		border-radius: 0.25rem;
		font-size: 0.75rem;
		cursor: pointer;
		transition: all 0.2s;
	}

	.clear-button:hover:not(:disabled) {
		background: rgba(244, 67, 54, 0.2);
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
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		border-bottom-left-radius: 0.25rem;
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

	.message-expiration {
		font-style: italic;
	}

	.message-expiration.expiring {
		color: orange;
		font-weight: 500;
		opacity: 1;
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

	.function-result-data pre {
		margin: 0;
		font-family: monospace;
		white-space: pre-wrap;
	}

	.chat-input-wrapper {
		display: flex;
		padding: clamp(0.75rem, 2vw, 1.5rem);
		background: var(--ui-bg-element-darker, #2a2a2a);
		border-top: 1px solid var(--ui-border, #444);
	}

	.chat-input {
		flex: 1;
		padding: clamp(0.75rem, 1.5vw, 1rem);
		font-size: clamp(0.875rem, 1vw, 1rem);
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		border-radius: 1.5rem;
		min-height: clamp(36px, 5vh, 48px);
		padding-right: clamp(3rem, 5vw, 3.5rem);
	}

	.send-button {
		position: absolute;
		right: clamp(1.25rem, 3vw, 2rem);
		transform: translateY(-50%);
		top: 50%;
		background: transparent;
		border: none;
		cursor: pointer;
		color: var(--text-secondary, #aaa);
		width: clamp(2rem, 4vw, 2.25rem);
		height: clamp(2rem, 4vw, 2.25rem);
		display: flex;
		align-items: center;
		justify-content: center;
		transition: color 0.2s;
	}

	.send-button:hover:not(:disabled) {
		color: var(--text-primary, #fff);
	}

	.send-button:disabled {
		cursor: not-allowed;
		opacity: 0.6;
	}

	.send-icon {
		width: clamp(1rem, 2vw, 1.25rem);
		height: clamp(1rem, 2vw, 1.25rem);
		fill: currentColor;
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
	
	.message-content :global(pre) {
		background: rgba(0, 0, 0, 0.2);
		padding: 0.5rem;
		border-radius: 0.25rem;
		overflow-x: auto;
		margin: 0.5rem 0;
		font-size: 0.8rem;
	}
	
	.message-content :global(code) {
		font-family: monospace;
		background: rgba(0, 0, 0, 0.2);
		padding: 0.1rem 0.25rem;
		border-radius: 0.25rem;
		font-size: 0.8rem;
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
</style>
