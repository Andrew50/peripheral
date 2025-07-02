<script lang="ts">
	import type { TimelineEvent } from '../interface';

	export let timeline: TimelineEvent[] = [];

	// Only show messages that come from the backend (exclude frontend-generated "Message sent to server")
	$: filteredTimeline = timeline.filter((event) => {
		const message = event.message?.toLowerCase() || '';
		return !message.includes('message sent') && message.trim().length > 0;
	});
</script>

{#if filteredTimeline.length > 0}
	<div class="simple-timeline">
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
	</div>
{/if}

<style>
	.simple-timeline {
		margin: 0.75rem 0 0 0;
	}

	.timeline-items {
		margin-left: 0.5rem;
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
