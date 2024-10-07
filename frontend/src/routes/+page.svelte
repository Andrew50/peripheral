<script>
    import  Header  from '$lib/utils/modules/header.svelte';
    import '$lib/core/global.css'
    import {browser} from '$app/environment'
    if (browser){
        document.title = "Atlantis"
    }

</script>
<Header />
<main class="main-container">
  <div class="content">
    <h1 class="title" style="font-size: 64px;">Atlantis</h1>
  </div>
</main>
