<script lang="ts">
	import type { TimelineEvent } from '../interface';

	export let timeline: TimelineEvent[] = [];
	export let currentStatus: string = '';
	export let showTimelineDropdown: boolean = false;
	export let onToggleDropdown: (() => void) | undefined = undefined;

	// Only show messages that come from the backend (exclude frontend-generated "Message sent to server")
	$: filteredTimeline = timeline.filter((event) => {
		const message = event.message?.toLowerCase() || '';
		return !message.includes('message sent') && message.trim().length > 0;
	});
	
	// Show dropdown toggle if there are timeline items to show
	$: showDropdownToggle = timeline.length > 1 && onToggleDropdown;
	
	// For web searches, we only show chips (no status text)
	$: shouldShowStatusHeader = currentStatus && typeof currentStatus === 'string' && currentStatus.trim().length > 0;
</script>

{#if shouldShowStatusHeader || filteredTimeline.length > 0}
	<div class="thinking-trace">
		{#if shouldShowStatusHeader}
			<div class="status-header">
				<div class="current-status">
					{currentStatus}
				</div>
				{#if showDropdownToggle}
					<button
						class="timeline-dropdown-toggle"
						on:click={onToggleDropdown}
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
		{/if}
		
		{#if showTimelineDropdown && filteredTimeline.length > 0}
			<div class="timeline-items">
				{#each filteredTimeline as event, index}
					<div class="timeline-item">
						<div class="timeline-dot"></div>
						<div class="timeline-content">
							{#if event.type === 'webSearchQuery' && event.data?.query}
								<div class="timeline-websearch">
									<div class="web-search-chip glass glass--pill glass--responsive">
										<svg viewBox="0 0 24 24" width="14" height="14" class="search-icon">
											<path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" fill="currentColor"/>
										</svg>
										<span class="search-query">{event.data.query}</span>
									</div>
								</div>
							{:else}
								{event.message}
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.thinking-trace {
		margin: 0.75rem 0 0 0;
		border: 1px solid rgba(255, 255, 255, 0.4);
		border-radius: 1rem;
		padding: 0.75rem;
	}

	.status-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.5rem;
	}

	.current-status {
		color: transparent;
		font-size: 0.9rem;
		font-weight: 500;
		flex: 1;
		background: linear-gradient(
			90deg,
			var(--text-secondary, #aaa),
			rgba(255, 255, 255, 0.9),
			var(--text-secondary, #aaa)
		);
		background-size: 200% auto;
		background-clip: text;
		-webkit-background-clip: text;
		animation: loading-text-highlight 2.5s infinite linear;
	}

	@keyframes loading-text-highlight {
		0% {
			background-position: 200% center;
		}
		100% {
			background-position: -200% center;
		}
	}

	.timeline-dropdown-toggle {
		background: none;
		border: none;
		padding: 0.25rem;
		cursor: pointer;
		color: var(--text-secondary, #ccc);
		display: flex;
		align-items: center;
		justify-content: center;
		opacity: 0.7;
		transition: opacity 0.2s ease;
		border-radius: 0.25rem;
	}

	.timeline-dropdown-toggle:hover {
		opacity: 1;
		background-color: rgba(255, 255, 255, 0.1);
	}

	.chevron-icon {
		transition: transform 0.2s ease;
	}

	.chevron-icon.expanded {
		transform: rotate(180deg);
	}

	.timeline-items {
		margin-left: 0.5rem;
		margin-top: 0.5rem;
	}

	.timeline-item {
		position: relative;
		display: flex;
		align-items: flex-start;
		margin-bottom: 1rem;
		line-height: 1.4;
	}

	.timeline-item:last-child {
		margin-bottom: 0;
	}

	.timeline-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background-color: var(--text-secondary, #ccc);
		margin-right: 0.75rem;
		margin-top: 0.4rem;
		flex-shrink: 0;
		position: relative;
	}

	.timeline-dot::after {
		content: '';
		position: absolute;
		left: 50%;
		top: 100%;
		transform: translateX(-50%);
		width: 1px;
		height: 1.5rem;
		background-color: var(--text-secondary, #ccc);
		opacity: 0.3;
	}

	.timeline-item:last-child .timeline-dot::after {
		display: none;
	}

	.timeline-content {
		opacity: 0.8;
		flex: 1;
		font-size: 0.8rem;
		color: var(--text-secondary, #ccc);
	}

	.timeline-websearch {
		margin-top: 0.25rem;
		display: inline-block;
		width: fit-content;
	}

	.web-search-chip {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.25rem 0.5rem;
		background: var(--c-green);
		border-radius: 0.75rem;
		font-size: 0.75rem;
		color: var(--text-primary);
		border: 1px solid var(--c-green-dark);
		animation: fadeInUp 0.3s ease-out;
	}

	.web-search-chip .search-icon {
		flex-shrink: 0;
		opacity: 0.8;
	}

	.web-search-chip .search-query {
		font-weight: 300;
	}

	@keyframes fadeInUp {
		from {
			opacity: 0;
			transform: translateY(10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
