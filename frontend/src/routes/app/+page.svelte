<!-- app/+page.svelte -->
<script lang="ts">
    import {auth_data, request} from '../../store.js'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    let securityId: string;
    let timestamp: number;

    $: if ($auth_data == null && browser) {
        goto('/login');
    }
    function submit (): void {
        if (securityId && timestamp) {
            request(null, true, "NewInstance", securityId, timestamp)
        }
    }
        
</script>
<h1> nedfsfdrm -rf node_modules package-lock.jsonw instance </h1>
<div class="form" >
<div>
<input bind:value={securityId}/>
</div>
<div>
<input type="number" bind:value={timestamp}/>
</div>
<div>
<button on:click={submit}> enter </button>
</div>
</div>


<style>



</style>




