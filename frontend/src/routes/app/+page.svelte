<!-- app/+page.svelte -->
<script lang="ts">
    import Chart from '$lib/features/chart/chart.svelte'
    import RightClick from '$lib/utils/rightClick.svelte'
    import Input from '$lib/utils/input.svelte'
    import Similar from '$lib/utils/similar.svelte'
    import Study from '$lib/features/study.svelte'
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend'
    import { goto } from '$app/navigation';
    onMount(() => {
        privateRequest<string>("verifyAuth", {}).catch((error) => {
            goto('/login')
        });
    });
</script>

<div class="container">
    <Input/>
    <RightClick/>
    <Similar/>
    <Chart/>
    <Study/>
</div>
