import { writable } from 'svelte/store';
import type { Writable } from 'svelte/store';
import { get } from 'svelte/store';
import { goto } from "$app/navigation";
export let menuLeftPos = writable(400);
export let annotateData = writable([]);
export let journalData = writable([]);
export let screener_data = writable([]);
export let chartQuery = writable([]);
export let match_data = writable([[], [], []]);
export let auth_data = writable({});
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
    auth_data.set("");
    goto('/');
}

/*export async function publicRequest(){
    url = `${base_url}/public`;
    headers = { 'Content-Type': 'application/json' };
}*/
export async function publicRequest<T>(func: string, args: any, error: Writable<string>): Promise<T> {
    const payload = JSON.stringify({
        func: func,
        args: args
    })
    const response = await fetch(`${base_url}/public`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload});
    console.log('request:', payload,'result:', response);
    if (response.ok){
        error.set("")
        return await response.json() as T;
    }else{
        const errorMessage = await response.text()
        error.set(errorMessage);
        return Promise.reject();
    }
}

export async function privateRequest<T>(func: string, args: any, error: Writable<string>): Promise<T> {
    const authToken = get(auth_data)
    const headers = {
        'Content-Type': 'application/json',
        ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
    };
    const filteredHeaders = Object.fromEntries(
        Object.entries(headers).filter(([_, value]) => value !== undefined)
    );
    const payload = JSON.stringify({
        func: func,
        args: args
    })
    const response = await fetch(`${base_url}/private`, {
        method: 'POST',
        headers: filteredHeaders,
        body: payload});
    console.log('request:', payload,'result:', response);
    if (response.ok){
        error.set("")
        return await response.json() as T;
    }else{
        const errorMessage = await response.text()
        error.set(errorMessage);
        return Promise.reject();
    }
}
