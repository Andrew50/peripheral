import { writable } from 'svelte/store';
import type { Writable } from 'svelte/store';
import { get } from 'svelte/store';
import { goto } from "$app/navigation";
import { onMount } from 'svelte';

export let menuLeftPos = writable(400);
export let annotateData = writable([]);
export let journalData = writable([]);
export let screener_data = writable([]);
export let chartQuery = writable([]);
export let match_data = writable([[], [], []]);
export let setups_list = writable([]);
export let currentEntry = writable("");
export let watchlist_data = writable({});
export let settings = writable({});
export const focus = writable(null);

let base_url: string;

if (typeof window !== 'undefined') {
    const url = new URL(window.location.origin);
    url.port = "5057";
    base_url = url.toString();
    base_url = base_url.substring(0,base_url.length - 1);
/*    if (window.location.hostname === 'localhost') {
        base_url = 'http://localhost:5057'; //dev
    } else {
        base_url = window.location.origin; //prod
    }*/
}

export function logout() {
    sessionStorage.setItem("authToken","")
    goto('/login');
}

export async function publicRequest<T>(func: string, args: any): Promise<T> {
    const payload = JSON.stringify({
        func: func,
        args: args
    })
    const response = await fetch(`${base_url}/public`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload});
    if (response.ok){
        const result = await response.json() as T
        console.log("payload: ",payload, "result: ", result)
        return result;
    }else{
        const errorMessage = await response.text()
        console.error("payload: ",payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}


export async function privateRequest<T>(func: string, args: any): Promise<T> {
    const authToken = sessionStorage.getItem("authToken")
    const headers = {
        'Content-Type': 'application/json',
        ...(authToken ? { 'Authorization': authToken} : {}),
    };
    const payload = {
        func: func,
        args: args
    }
    const response = await fetch(`${base_url}/private`, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload)
    });
    if (response.ok){
        const result = await response.json() as T
        console.log("payload: ",payload, "result: ", result)
        return result;
    }else{
        const errorMessage = await response.text()
       console.error("payload: ",payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}
