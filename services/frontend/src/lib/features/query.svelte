<script lang="ts">
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { onMount } from 'svelte';
	import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';

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
		query: string;
		response_text: string;
		function_calls?: any[];
		tool_results?: FunctionResult[];
		timestamp: string | Date;
	};

	// Message type for chat history
	type Message = {
		id: string;
		content: string;
		sender: 'user' | 'assistant';
		timestamp: Date;
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
			if (conversation && conversation.query && conversation.response_text) {
				// Add user message from history
				messages.push({
					id: generateId(),
					content: conversation.query,
					sender: 'user',
					timestamp: new Date(conversation.timestamp)
				});

				// Add assistant response from history
				messages.push({
					id: generateId(),
					content: conversation.response_text,
					sender: 'assistant',
					timestamp: new Date(conversation.timestamp),
					responseType: conversation.function_calls?.length ? 'function_calls' : 'text',
					functionResults: conversation.tool_results || []
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

				const assistantMessage: Message = {
					id: generateId(),
					content: typedResponse.text || 'Function calls executed successfully.',
					sender: 'assistant',
					timestamp: new Date(),
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
</script>

<div class="chat-container">
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
					<div class="message {message.sender} {message.responseType === 'error' ? 'error' : ''}">
						{#if message.isLoading}
							<div class="typing-indicator">
								<span></span>
								<span></span>
								<span></span>
							</div>
						{:else}
							<div class="message-content">
								<p>{message.content}</p>
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

							<div class="message-timestamp">
								{formatTimestamp(message.timestamp)}
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
	}

	.message-timestamp {
		font-size: 0.7rem;
		opacity: 0.7;
		text-align: right;
		margin-top: 0.25rem;
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
		font-size: 0.875rem;
		color: var(--accent-color, #3a8bf7);
	}

	.function-error {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem;
		color: var(--error-color, #f44336);
		font-size: 0.875rem;
	}

	.function-result-data {
		padding: 0.75rem;
		font-size: 0.8125rem;
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
</style>
