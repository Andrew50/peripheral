import { type Instance } from '$lib/core/types';
import { privateRequest } from '$lib/core/backend';
import {writable, get} from "svelte/store"
export const inputValue = writable("")

export async function addInstanceToChat(instance: Instance) {
	/* -------- 1. Resolve the ticker symbol (if not supplied) -------- */
	let { ticker } = instance as any;
	if (!ticker) {
		try {
			const resp = await privateRequest('getSecurityInfo', {
				securityId: instance.securityId
			});
			ticker = resp?.ticker;
		} catch (err) {
			console.error('addInstanceToChat › unable to resolve ticker', err);
			return;
		}
	}
	if (!ticker) {
		console.error(
			`addInstanceToChat › ticker not found for securityId=${instance.securityId}`
		);
		return;
	}

	const tsMs =
		typeof instance.timestamp === 'number'
			? instance.timestamp
			: new Date(instance.timestamp as string).getTime() || 0;

	const token = `$$$${ticker.toUpperCase()}-${instance.securityId}-${tsMs}$$$`;

	if (get(inputValue).trim().length > 0) {
		// add a trailing space so users can keep typing naturally
		inputValue.set( `${get(inputValue).trim()} ${token} `);
	} else {
		inputValue.set( `${token} `)
	}

	// Optional: keep focus on the input field
	//if (queryInput) queryInput.focus();
}

