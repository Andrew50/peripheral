<script lang="ts">
	import type { TimelineEvent } from '../interface';

	export let timeline: TimelineEvent[] = [];
	export let currentStatus: string = '';
	export let showTimelineDropdown: boolean = false;
	export let onToggleDropdown: (() => void) | undefined = undefined;

	// Show dropdown toggle if there are timeline items to show
	$: showDropdownToggle = timeline.length > 1 && onToggleDropdown;
	
	// For web searches, we only show chips (no status text)
	$: shouldShowStatusHeader = currentStatus && typeof currentStatus === 'string' && currentStatus.trim().length > 0;

	// Check if there's an active web search
	$: hasActiveWebSearch = timeline.some(event => 
		event.type === 'webSearchQuery' && 
		!timeline.some(laterEvent => 
			laterEvent.type === 'webSearchCitations' && 
			laterEvent.timestamp > event.timestamp
		)
	);
	$: hasActiveBacktest = currentStatus && 
		typeof currentStatus === 'string' && 
		currentStatus.toLowerCase().includes('backtest');

	// Always show recent timeline items (even when dropdown is collapsed)
	$: lastTimelineItem = timeline.slice(-1); // Show last item
	$: allTimelineItems = showTimelineDropdown ? timeline : lastTimelineItem;

</script>

{#if timeline.length > 1}
	<div class="thinking-trace">
		{#if shouldShowStatusHeader}
			<div class="status-header">
				<div class="current-status">
					<div class="status-icon">
						<svg width="24" height="24" viewBox="0 0 20 20" fill={hasActiveBacktest ? 'none' : 'currentColor'} stroke={hasActiveBacktest ? 'currentColor' : 'none'} stroke-width={hasActiveBacktest ? '1.2' : '0'} xmlns="http://www.w3.org/2000/svg" class={hasActiveWebSearch ? 'globe-spinner' : ''}>
							{#if hasActiveWebSearch}
								<path d="M10 2.125C14.3492 2.125 17.875 5.65076 17.875 10C17.875 14.3492 14.3492 17.875 10 17.875C5.65076 17.875 2.125 14.3492 2.125 10C2.125 5.65076 5.65076 2.125 10 2.125ZM7.88672 10.625C7.94334 12.3161 8.22547 13.8134 8.63965 14.9053C8.87263 15.5194 9.1351 15.9733 9.39453 16.2627C9.65437 16.5524 9.86039 16.625 10 16.625C10.1396 16.625 10.3456 16.5524 10.6055 16.2627C10.8649 15.9733 11.1274 15.5194 11.3604 14.9053C11.7745 13.8134 12.0567 12.3161 12.1133 10.625H7.88672ZM3.40527 10.625C3.65313 13.2734 5.45957 15.4667 7.89844 16.2822C7.7409 15.997 7.5977 15.6834 7.4707 15.3486C6.99415 14.0923 6.69362 12.439 6.63672 10.625H3.40527ZM13.3633 10.625C13.3064 12.439 13.0059 14.0923 12.5293 15.3486C12.4022 15.6836 12.2582 15.9969 12.1006 16.2822C14.5399 15.467 16.3468 13.2737 16.5947 10.625H13.3633ZM12.1006 3.7168C12.2584 4.00235 12.4021 4.31613 12.5293 4.65137C13.0059 5.90775 13.3064 7.56102 13.3633 9.375H16.5947C16.3468 6.72615 14.54 4.53199 12.1006 3.7168ZM10 3.375C9.86039 3.375 9.65437 3.44756 9.39453 3.7373C9.1351 4.02672 8.87263 4.48057 8.63965 5.09473C8.22547 6.18664 7.94334 7.68388 7.88672 9.375H12.1133C12.0567 7.68388 11.7745 6.18664 11.3604 5.09473C11.1274 4.48057 10.8649 4.02672 10.6055 3.7373C10.3456 3.44756 10.1396 3.375 10 3.375ZM7.89844 3.7168C5.45942 4.53222 3.65314 6.72647 3.40527 9.375H6.63672C6.69362 7.56102 6.99415 5.90775 7.4707 4.65137C7.59781 4.31629 7.74073 4.00224 7.89844 3.7168Z"></path>
							{:else if hasActiveBacktest}
								<path stroke-linecap="round" stroke-linejoin="round" d="M8.62 3.28c.07-.45.46-.78.92-.78h.91c.46 0 .85.33.92.78l.12.74c.06.35.32.64.65.77.33.14.71.12 1-.09l.61-.44c.37-.27.88-.22 1.2.1l.64.65c.32.32.37.83.1 1.2l-.44.61c-.21.29-.23.67-.09 1 .14.33.42.59.77.65l.74.12c.45.07.78.46.78.92v.91c0 .46-.33.85-.78.92l-.74.12c-.35.06-.63.32-.77.65-.14.33-.12.71.09 1l.44.61c.27.37.22.88-.1 1.2l-.65.64c-.32.32-.83.37-1.2.1l-.61-.44c-.29-.21-.67-.23-1-.09-.33.14-.59.42-.65.77l-.12.74c-.07.45-.46.78-.92.78h-.91c-.46 0-.85-.33-.92-.78l-.12-.74c-.06-.35-.32-.64-.65-.77-.33-.14-.71-.12-1 .09l-.61.44c-.37.27-.88.22-1.2-.1l-.64-.65c-.32-.32-.37-.83-.1-1.2l.44-.61c.21-.29.23-.67.09-1-.14-.33-.42-.59-.77-.65l-.74-.12c-.45-.07-.78-.46-.78-.92v-.91c0-.46.33-.85.78-.92l.74-.12c.35-.06.63-.32.77-.65.14-.33.12-.71-.09-1l-.44-.61c-.27-.37-.22-.88.1-1.2l.65-.64c.32-.32.83-.37 1.2-.1l.61.44c.29.21.67.23 1 .09.33-.14.59-.42.65-.77l.12-.74Z"></path>
								<path stroke-linecap="round" stroke-linejoin="round" d="M12.5 10a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0Z"></path>
							{:else}
								<path d="M15.1687 8.0855C15.1687 5.21138 12.8509 2.88726 9.99976 2.88726C7.14872 2.88744 4.83179 5.21149 4.83179 8.0855C4.8318 9.91374 5.7711 11.5187 7.19019 12.4459H12.8103C14.2293 11.5187 15.1687 9.91365 15.1687 8.0855ZM8.47046 16.1099C8.72749 16.6999 9.31515 17.1127 9.99976 17.1128C10.6844 17.1128 11.2719 16.6999 11.5291 16.1099H8.47046ZM7.65894 14.7798H12.3416V13.7759H7.65894V14.7798ZM16.4988 8.0855C16.4988 10.3216 15.3777 12.2942 13.6716 13.4703V15.4449C13.6714 15.8119 13.3736 16.1098 13.0066 16.1099H12.9216C12.6187 17.4453 11.4268 18.4429 9.99976 18.4429C8.57283 18.4428 7.3807 17.4453 7.07788 16.1099H6.9939C6.62677 16.1099 6.32909 15.8119 6.32886 15.4449V13.4703C4.62271 12.2942 3.50172 10.3217 3.50171 8.0855C3.50171 4.48337 6.40777 1.55736 9.99976 1.55718C13.5919 1.55718 16.4988 4.48326 16.4988 8.0855Z"></path>
							{/if}
						</svg>	
					</div>
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
		
		{#if allTimelineItems.length > 0}
			<div class="timeline-items">
				{#each allTimelineItems as event, index}
					<div class="timeline-item">
						<div class="timeline-dot"></div>
						<div class="timeline-content">
							{#if event.type === 'webSearchQuery' && event.data?.query}
								<div class="timeline-websearch">
									<div class="web-search-chip glass glass--pill glass--responsive">
										<svg class="search-icon" viewBox="0 0 24 24" width="18" height="18" fill="none">
											<path
												d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z"
												stroke="currentColor"
												stroke-width="2"
												stroke-linecap="round"
												stroke-linejoin="round"
											/>
										</svg>
										<span class="search-query">{event.data.query}</span>
									</div>
								</div>
							{:else if event.type === 'webSearchCitations' && event.data?.citations}
								<div class="timeline-citations">
									<div class="citations-header">

										<span>Reading sources · {event.data.citations.length} </span>
									</div>
									<div class="citations-container">
										{#each event.data.citations as citation, index}
											<div class="citation-item" 
												on:click={() => {
													if (citation.url) {
														window.open(citation.url, '_blank');
													}
												}} 
												on:keydown={(e) => {
													if (e.key === 'Enter' && citation.url) {
														window.open(citation.url, '_blank');
													}
												}} 
												role="button" 
												tabindex="0">
												{#if citation.urlIcon}
													<img 
														src={citation.urlIcon} 
														alt="Site icon" 
														class="citation-favicon"
														on:error={(e) => {
															if (e.target && 'style' in e.target) {
																e.target.style.display = 'none';
															}
														}}
													/>
												{:else}
													<div class="citation-favicon-placeholder"></div>
												{/if}
												<span class="citation-title">{citation.title || 'Untitled'}</span>
												<span class="citation-url">{citation.url ? citation.url.replace(/^https?:\/\//, '').split('/')[0].split('.').slice(0, -1).join('.') : 'Unknown URL'}</span>
											</div>
										{/each}
									</div>
								</div>
							{:else}
							 <span> {event.headline} </span>						
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
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	@keyframes loading-text-highlight {
		0% {
			background-position: 200% center;
		}
		100% {
			background-position: -200% center;
		}
	}

	.status-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
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
		min-width: 0; /* Allows flex item to shrink */
		max-width: 100%;
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

	.timeline-citations {
		margin-top: 0.25rem;
		width: 100%;
		max-width: 100%;
		overflow: hidden;
	}

	.citations-header {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		margin-bottom: 0.5rem;
		font-size: 0.75rem;
		color: var(--text-secondary);
		opacity: 0.8;
	}

	.citations-container {
		max-height: 200px;
		overflow-y: auto;
		border: 1.5px solid #272929;
		border-radius: 0.5rem;
		background: #1f2121;
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.citation-item {
		padding: 0.5rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		cursor: pointer;
		transition: background-color 0.2s ease;
		animation: fadeInUp 0.3s ease-out;
		width: 100%;
		box-sizing: border-box;
		min-width: 0; /* Allows text to shrink */
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.citation-item:last-child {
		border-bottom: none;
	}

	.citation-item:hover {
		background-color: rgba(255, 255, 255, 0.05);
	}

	.citation-item:focus {
		outline: 1px solid var(--c-blue);
		outline-offset: -1px;
		background-color: rgba(255, 255, 255, 0.05);
	}

	.citation-title {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--text-primary);
		line-height: 1.3;
		white-space: nowrap;
		overflow: hidden;
		flex: 1;
		min-width: 0;
	}

	.citation-url {
		font-size: 0.7rem;
		color: var(--text-secondary);
		opacity: 0.7;
		font-family: monospace;
		flex-shrink: 0;
		white-space: nowrap;
	}

	.citation-favicon {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-right: 0.5rem;
		object-fit: cover;
	}

	.citation-favicon-placeholder {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-right: 0.5rem;
		background-color: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
	}
</style>
