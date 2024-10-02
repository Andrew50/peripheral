import type {Instance} from '$lib/core/types'
import {privateRequest} from '$lib/core/backend'
import {writable} from 'svelte/store'
import {dispatchMenuChange} from '$lib/core/stores'
type SetupEvent = "new"|"save"|"cancel"
export let eventDispatcher = writable<SetupEvent>()
export function setSample (setupId:number|"new",instance:Instance):void{
    if (setupId === "new"){
        dispatchMenuChange.set("setups")
        eventDispatcher.set("new")
        const unsub = eventDispatcher.subscribe((v:SetupEvent)=>{
            if (v === "new"){
            }else if (v === "cancel"){
                unsub()
            }else{
                privateRequest<void>("setSample",{setupId:v,...instance})
                unsub()
            }
        })
    }else{
        privateRequest<void>("setSample",{setupId:setupId,...instance})
    }
}
