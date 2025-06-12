<script lang="ts">
    import type { ConversationSummary } from '../interface';
    
    export let conversationDropdown: HTMLDivElement;
    export let showConversationDropdown: boolean;
    export let conversations: ConversationSummary[];
    export let currentConversationId: string;
    export let currentConversationTitle: string;
    export let loadingConversations: boolean;
    export let conversationToDelete: string;
    export let messagesStore: any;
    export let isLoading: boolean;
    
    export let toggleConversationDropdown: () => void;
    export let createNewConversation: () => void;
    export let switchToConversation: (id: string, title: string) => void;
    export let deleteConversation: (id: string, e: MouseEvent) => void;
    export let confirmDeleteConversation: (id: string) => void;
    export let cancelDeleteConversation: () => void;
    export let handleShareConversation: () => void;
    export let clearConversation: () => void;
</script>

<div class="chat-header">
    <div class="header-left">
        <div class="conversation-dropdown-container" bind:this={conversationDropdown}>
            <button class="hamburger-button" on:click={toggleConversationDropdown} aria-label="Open conversations menu">
                <svg viewBox="0 0 24 24" width="20" height="20">
                    <path d="M3,6H21V8H3V6M3,11H21V13H3V11M3,16H21V18H3V16Z" fill="currentColor" />
                </svg>
            </button>
            
            {#if showConversationDropdown}
                <div class="conversation-dropdown glass glass--rounded glass--responsive">
                                        <div class="dropdown-header">
                    <h4>Conversations</h4>
                    <div class="header-buttons">
                        <button class="new-conversation-btn glass glass--small glass--responsive" on:click={createNewConversation}>
                            <svg viewBox="0 0 24 24" width="16" height="16">
                                <path d="M19,13H13V19H11V13H5V11H11V5H13V11H19V13Z" fill="currentColor" />
                            </svg>
                            New Chat
                        </button>
                    </div>
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
                                    
                                    {#if conversationToDelete === conversation.conversation_id}
                                        <!-- Show Yes/No buttons when in delete mode -->
                                        <div class="delete-confirmation-buttons">
                                            <button 
                                                class="confirm-delete-btn yes"
                                                on:click={(e) => {
                                                    e.stopPropagation();
                                                    confirmDeleteConversation(conversation.conversation_id);
                                                }}
                                                aria-label="Confirm delete"
                                            >
                                                Delete
                                            </button>
                                            <button 
                                                class="confirm-delete-btn no"
                                                on:click={(e) => {
                                                    e.stopPropagation();
                                                    cancelDeleteConversation();
                                                }}
                                                aria-label="Cancel delete"
                                            >
                                                Cancel
                                            </button>
                                        </div>
                                    {:else}
                                        <!-- Show normal delete button -->
                                        <button 
                                            class="delete-conversation-btn"
                                            on:click={(e) => deleteConversation(conversation.conversation_id, e)}
                                            aria-label="Delete conversation"
                                        >
                                            <svg viewBox="0 0 24 24" width="14" height="14">
                                                <path d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z" fill="currentColor" />
                                            </svg>
                                        </button>
                                    {/if}
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
            <div class="header-buttons">
                <button 
                    class="header-btn share-btn glass glass--small glass--responsive" 
                    on:click={handleShareConversation}
                    disabled={!currentConversationId}
                    title="Share Current Conversation"
                >
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/>
                        <polyline points="16,6 12,2 8,6"/>
                        <line x1="12" y1="2" x2="12" y2="15"/>
                    </svg>
                    Share
                </button>
                <button class="header-btn new-chat-btn glass glass--small glass--responsive" on:click={clearConversation} disabled={isLoading}>
                    <svg viewBox="0 0 24 24" width="14" height="14">
                        <path d="M19,13H13V19H11V13H5V11H11V5H13V11H19V13Z" fill="currentColor" />
                    </svg>
                    <span class="new-chat-text">New Chat</span>
                </button>
            </div>
        {/if}
    </div>
</div>