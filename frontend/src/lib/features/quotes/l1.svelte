
<script lang='ts'>
    export let ticker
    let store: Writable<Quote>;
    onMount(()=>{
        [store,release] = getStream(ticker,"fast")
    })
    onDestroy(()=>{

</script>

<

