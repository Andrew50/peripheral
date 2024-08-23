<!-- app/+page.svelte -->
<script lang"ts">
    import {auth_data} from '../../store.js'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    let entry: string

    $: if ($auth_data == null && browser) {
        goto('/login');
    }
</script>



<textarea bind:value={entry}> </textarea>
