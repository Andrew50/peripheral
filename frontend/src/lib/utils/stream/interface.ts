
import { socket, subscribe, unsubscribe, activeChannels, subscribeSECFilings } from './socket'
import type { SubscriptionRequest, StreamCallback } from './socket'

import { DateTime } from 'luxon';
import { eventChart } from '$lib/features/chart/interface';
import { streamInfo } from '$lib/core/stores';
import { chartEventDispatcher } from '$lib/features/chart/interface';
import {
	getReferenceStartTimeForDateMilliseconds,
	isOutsideMarketHours,
	ESTSecondstoUTCMillis,
	getRealTimeTime
} from '$lib/core/timestamp';
import type { ReplayInfo } from '$lib/core/stores';
import { type Instance } from '$lib/core/types';
import { get } from 'svelte/store';
import type { StreamData, ChannelType, TimeType } from './socket';
export function releaseStream(channelName: string, callback: StreamCallback) {
	let callbacks = activeChannels.get(channelName);
	if (callbacks) {
		callbacks = callbacks.filter((v) => v !== callback);
		if (callbacks.length === 0) {
			activeChannels.delete(channelName);
			unsubscribe(channelName);
		} else {
			activeChannels.set(channelName, callbacks);
		}
	}
}

export function addStream<T extends StreamData>(
	instance: Instance,
	channelType: ChannelType,
	callback: (v: T) => void
): Function {
	if (!instance.securityId) return () => {};
	const channelName = `${instance.securityId}-${channelType}`;
	const callbacks = activeChannels.get(channelName);

	// If callbacks exist, this channel is already active
	if (callbacks) {
		if (!callbacks.includes(callback as StreamCallback)) {
			callbacks.push(callback as StreamCallback);
			// Always try to subscribe - the socket.ts will handle pending subscriptions
			subscribe(channelName);
		}
	} else {
		// New channel, set up normally
		activeChannels.set(channelName, [callback as StreamCallback]);
		subscribe(channelName);
	}

	return () => releaseStream(channelName, callback as StreamCallback);
}

export function startReplay(instance: Instance) {
	if (!instance.timestamp) return;
	if (get(streamInfo).replayActive) {
		stopReplay();
	}
	if (socket?.readyState === WebSocket.OPEN) {
		const timestampToUse = isOutsideMarketHours(instance.timestamp)
			? ESTSecondstoUTCMillis(getReferenceStartTimeForDateMilliseconds(instance.timestamp) / 1000)
			: instance.timestamp;
		setExtended(instance.extendedHours ?? false);
		const replayRequest: SubscriptionRequest = {
			action: 'replay',
			timestamp: timestampToUse
		};
		socket.send(JSON.stringify(replayRequest));
		console.log('replay request sent');
		streamInfo.update((v) => {
			return {
				...v,
				replayActive: true,
				replayPaused: false,
				startTimestamp: timestampToUse,
				timestamp: timestampToUse
			};
		});
		chartEventDispatcher.set({ event: 'replay', chartId: 'all' });
		//timeEvent.update((v: TimeEvent) => ({ ...v, event: 'replay' }));
	}
}

export function pauseReplay() {
	if (socket?.readyState === WebSocket.OPEN) {
		const pauseRequest: SubscriptionRequest = {
			action: 'pause'
		};
		socket.send(JSON.stringify(pauseRequest));
	}
	streamInfo.update((v) => ({ ...v, replayPaused: true, pauseTime: Date.now() }));
}

export function resumeReplay() {
	if (socket?.readyState === WebSocket.OPEN) {
		const playRequest: SubscriptionRequest = {
			action: 'play'
		};
		socket.send(JSON.stringify(playRequest));
	}
	streamInfo.update((v) => {
		const pauseDuration = Date.now() - (v.pauseTime || Date.now());
		return {
			...v,
			replayPaused: false,
			timestamp: v.timestamp + pauseDuration * v.replaySpeed,
			lastUpdateTime: Date.now()
		};
	});
}

export function stopReplay() {
	if (socket?.readyState === WebSocket.OPEN) {
		const stopRequest: SubscriptionRequest = {
			action: 'realtime'
		};
		socket.send(JSON.stringify(stopRequest));
	}
	streamInfo.update((r: ReplayInfo) => ({
		...r,
		replayActive: false,
		timestamp: getRealTimeTime()
	}));
}
export function changeSpeed(speed: number) {
	if (socket?.readyState === WebSocket.OPEN) {
		const stopRequest: SubscriptionRequest = {
			action: 'speed',
			speed: speed
		};
		socket.send(JSON.stringify(stopRequest));
	}
	streamInfo.update((r: ReplayInfo) => ({ ...r, replaySpeed: speed }));
}

export function nextDay() {
	if (socket?.readyState === WebSocket.OPEN) {
		const stopRequest: SubscriptionRequest = {
			action: 'nextOpen'
		};
		socket.send(JSON.stringify(stopRequest));
	}
	chartEventDispatcher.set({ event: 'replay', chartId: 'all' });
}
export function setExtended(extendedHours: boolean) {
	if (socket?.readyState === WebSocket.OPEN) {
		const stopRequest: SubscriptionRequest = {
			action: 'setExtended',
			extendedHours: extendedHours
		};
		socket.send(JSON.stringify(stopRequest));
	}
	streamInfo.update((r: ReplayInfo) => ({ ...r, extendedHours: extendedHours }));
}


// Function to subscribe to global SEC filings feed
export function addGlobalSECFilingsStream(callback: StreamCallback): Function {
    const channelName = 'sec-filings';
    const callbacks = activeChannels.get(channelName);

    // If callbacks exist, this channel is already active
    if (callbacks) {
        if (!callbacks.includes(callback)) {
            callbacks.push(callback);
            // Re-subscribe to get initial value
            if (socket?.readyState === WebSocket.OPEN) {
                subscribeSECFilings();
            }
        }
    } else {
        // New channel, set up normally
        activeChannels.set(channelName, [callback]);
        subscribeSECFilings();
    }

    return () => releaseStream(channelName, callback);
}
export function releaseGlobalSECFilingsStream(callback: StreamCallback) {
    const channelName = 'sec-filings';
    releaseStream(channelName, callback);
}

// /streamInterface.ts
