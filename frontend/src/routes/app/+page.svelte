<!-- app/+page.svelte -->
<script lang="ts">
    import Chart from '$lib/features/chart.svelte'
    import RightClick from '$lib/utils/rightClick.svelte'
    import Input from '$lib/utils/input.svelte'
    import Study from '$lib/features/study.svelte'
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/api/backend'
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
    <Chart/>
    <Study/>
</div>
