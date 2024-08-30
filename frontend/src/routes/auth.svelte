<!-- account.svelte -->
<script lang="ts">
    import {publicRequest} from '../store'
    import Header from './header.svelte';
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
        await publicRequest('signup', {username:username, password:password},errorMessage);
        await signIn(username, password); // Automatically sign in after account creation
    } catch {
        errorMessage.set('Failed to create account');
    }
}
</script>

<div class="page">
<Header />
<main>
  <div class="container">
    <input placeholder="Username" bind:value={username} on:keydown={handleKeydown} />
    <input placeholder="Password" bind:value={password} on:keydown={handleKeydown} />
    {#if loginMenu}
      <button on:click={() => signIn(username, password)} class="login-btn">Sign In</button>
    {:else}
      <button on:click={() => signUp(username, password)} class="signup-btn">Create Account</button>
    {/if}
    <p class="error-message">{$errorMessage}</p>
  </div>
</main>
</div>

<style>
    @import "../global.css";
  main {
    display: flex; 
    justify-content: center;
    align-items: center;
    height: 100%;
    width: 100%;
    background-color: var(--c2);
    position: absolute;
  }

  .container {
    text-align: center;
  }

  input {
    display: block;
    margin: 10px auto;
    padding: 8px;
    font-size: 16px;
    border-radius: 5px;
    border: 1px solid #ccc;
    width: 80%;
    color: var(--f1);
    background-color: var(--c1);
    border: none;
  }
  input:focus {
    outline: none;
  }

  button {
    color: var(--f1);
    border: none;
    padding: 10px 20px;
    border-radius: 5px;
    cursor: pointer;
    font-size: 16px;
    transition: background-color 0.3s;
    margin-top: 10px;
    width: 80%;
  }
    .signup-btn {
        background-color: var(--c6);
    }
    .login-btn {
        background-color: var(--c3);
    }

  .signup-btn:hover {
    background-color: var(--c6-hover);
  }
  .login-btn:hover {
    background-color: var(--c3-hover);
  }

  .error-message {
    color: var(--c5);
    margin-top: 10px;
    width: 80%;
  }

</style>
