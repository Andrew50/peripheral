<!-- account.svelte -->
<script lang="ts">
    import {publicRequest} from '$lib/core/backend'
        import '$lib/core/global.css'

    import Header from '$lib/utils/header.svelte';
    import { goto } from '$app/navigation';
    import { writable } from 'svelte/store';
  
  export let loginMenu: boolean = false;
  let username = '';
  let password = '';
  let errorMessage = writable('');

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
        if (loginMenu){
          signIn(username, password);
        }else{
            signUp(username,password);
        }
    }
  }

  interface Login {
      token: string;
      settings: string;
  }

    async function signIn(username: string, password: string) {
        publicRequest<Login>('login', {username:username, password:password}).then((r : Login) => {
            sessionStorage.setItem("authToken",r.token)
            goto('/app');
        }).catch((error: string) => {
            errorMessage.set(error)
            });
    }

  async function signUp(username : string, password : string) {
    try {
        await publicRequest('signup', {username:username, password:password});
        await signIn(username, password); // Automatically sign in after account creation
    } catch {
        errorMessage.set('Failed to create account');
    }
}
</script>

<div class="page">
<Header />
<main>
  <div class="dcontainer">
    <input autofocus placeholder="Username" bind:value={username} on:keydown={handleKeydown} />
    <input placeholder="Password" bind:value={password} on:keydown={handleKeydown} />
    {#if loginMenu}
      <button  on:click={() => signIn(username, password)}>Sign In</button>
    {:else}
      <button on:click={() => signUp(username, password)} class="color2">Create Account</button>
    {/if}
    <p class="error-message">{$errorMessage}</p>
  </div>
</main>
</div>

<style>
  button {
      width: 100%;
  }
</style>
