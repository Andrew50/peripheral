<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { streamInfo } from '$lib/core/stores';
    import { writable, derived } from 'svelte/store';
    import type {Instance} from '$lib/core/types'
    import { UTCSecondstoESTSeconds, ESTSecondstoUTCSeconds, ESTSecondstoUTCMillis, getReferenceStartTimeForDateMilliseconds, timeframeToSeconds} from '$lib/core/timestamp';

    export let instance: Instance;
    export let currentBarTimestamp: number;
    const countdown = writable(0);

    // Derived store to format the countdown into a human-readable format
    const formattedCountdown = derived(countdown, $countdown => {
        const years = Math.floor($countdown / (365 * 24 * 60 * 60));
        const months = Math.floor(($countdown % (365 * 24 * 60 * 60)) / (30 * 24 * 60 * 60));
        const weeks = Math.floor(($countdown % (30 * 24 * 60 * 60)) / (7 * 24 * 60 * 60));
        const days = Math.floor(($countdown % (7 * 24 * 60 * 60)) / (24 * 60 * 60));
        const hours = Math.floor(($countdown % (24 * 60 * 60)) / (60 * 60));
        const minutes = Math.floor(($countdown % (60 * 60)) / 60);
        const seconds = Math.floor($countdown % 60);

        if (years > 0) {
            return `${years}y ${months}m`;
        } else if (months > 0) {
            return `${months}m ${weeks}w`;
        } else if (weeks > 0) {
            return `${weeks}w ${days}d`;
        } else if (days > 0) {
            return `${days}d ${hours}h`;
        } else if (hours > 0) {
            return `${hours}h ${minutes}m`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds < 10 ? '0' : ''}${seconds}s`;
        } else {
            return `${seconds < 10 ? '0' : ''}${seconds}s`;
        }
    });

    let interval: NodeJS.Timeout;

    function calculateCountdown() {
        const chartTimeframeInSeconds = timeframeToSeconds(instance.timeframe);
        const nextBarClose = currentBarTimestamp + chartTimeframeInSeconds;
        const remainingTime = nextBarClose - UTCSecondstoESTSeconds($streamInfo.timestamp/1000);
        countdown.set(remainingTime > 0 ? remainingTime : 0);
    }

    onMount(() => {
        interval = setInterval(() => {
            calculateCountdown();
        }, 1000);
    });

    onDestroy(() => {
        clearInterval(interval);
    });
</script>

<div class = "countdown-overlay">
    {#if $countdown > 0}
        {$formattedCountdown}
    {:else}
        Bar Closed
    {/if}
</div>

<style>
    .countdown-overlay {
        position: absolute; /* Position relative to the parent div */
        bottom: 20px; /* Position from the bottom of the parent div */
        right: 20px; /* Position from the right of the parent div */
        background: rgba(0, 0, 0, 0.7); /* Semi-transparent background */
        padding: 10px 20px; /* Padding for better spacing */
        border-radius: 10px; /* Rounded corners */
        color: white; /* Text color */
        font-size: 12px; /* Adjust font size */
        z-index: 990; /* Ensure it's on top of other elements */
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3); /* Add a subtle shadow */
    }
</style>

