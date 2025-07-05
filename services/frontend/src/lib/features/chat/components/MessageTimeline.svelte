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
</script>

{#if currentStatus || filteredTimeline.length > 0}
	<div class="simple-timeline">
		{#if currentStatus}
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
							{event.message}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.simple-timeline {
		margin: 0.75rem 0 0 0;
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 0.5rem;
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
</style>
