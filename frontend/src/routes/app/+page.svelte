<!-- app/+page.svelte -->
<script lang="ts">
    import Entry from './entry.svelte'
    import Related from './related.svelte'
    import Chart from './chart.svelte'
    import { onMount } from 'svelte';
    import { privateRequest } from '../../store'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import Instance from './instance.svelte'
    import { writable ,  get} from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import { createChart } from 'lightweight-charts';
    let ticker: string;
    let timestamp: number;
    let errorMessage = writable<string>('')
    let selectedInstanceId = writable(0);
    let currentAnnotation = "";
    let securityId: number;
    onMount(() => {
        privateRequest<string>("verifyAuth", {}).catch((error) => {
            goto('/login')
        });
    });
    

</script>

<Instance {securityId}/>
<Chart/>
<div class="chart">
    <h1> chart </h1>
</div>
