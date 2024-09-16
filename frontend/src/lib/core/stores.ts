import type {Setup} from '$lib/core/types'
import {writable} from 'svelte/store'
import type {Writable} from 'svelte/store'
import {privateRequest} from '$lib/core/backend'

privateRequest<Setup[]>('getSetups', {})
.then((v: Setup[]) => {
    v = v.map((v:Setup) => {
        return {...v,
          activeScreen: true}
    })
    console.log(v)
    setups.set(v);
})
.catch((error) => {
    console.error('Error fetching setups:', error);
});
export let setups: Writable<Setup[]> = writable([]);
