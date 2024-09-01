<!-- app/+page.svelte -->
<script lang="ts">
    import Chart from './chart.svelte'
    import Instance from './instance.svelte'
    import Study from './study.svelte'
    import { onMount } from 'svelte';
    import { privateRequest } from '../../store'
    import { goto } from '$app/navigation';
    onMount(() => {
        privateRequest<string>("verifyAuth", {}).catch((error) => {
            goto('/login')
        });
    });
</script>

<div class="container">
    <Chart/>
    <Instance/>
    <Study/>
</div>
