import { socket, subscribe, unsubscribe, activeChannels, subscribeSECFilings } from './socket';
import type { SubscriptionRequest, StreamCallback } from './socket';

import { streamInfo } from '$lib/utils/stores/stores';
import { chartEventDispatcher } from '$lib/features/chart/interface';
import {
	getReferenceStartTimeForDateMilliseconds,
	isOutsideMarketHours,
	ESTSecondstoUTCMillis
} from '$lib/utils/helpers/timestamp';
import type { ReplayInfo } from '$lib/utils/stores/stores';
import { type Instance } from '$lib/utils/types/types';
import { get } from 'svelte/store';
import type { StreamData, ChannelType } from './socket';
import { latestValue } from './socket';
export function releaseStream(channelName: string, callback: StreamCallback) {
	let callbacks = activeChannels.get(channelName);
	if (!callbacks) {
		return;
	}
	callbacks = callbacks.filter((v) => v !== callback);
	if (callbacks.length === 0) {
		activeChannels.delete(channelName);
		latestValue.delete(channelName);
		unsubscribe(channelName);
	} else {
		activeChannels.set(channelName, callbacks);
	}
}

export function addStream<T extends StreamData>(
	instance: Instance,
	channelType: ChannelType,
	callback: (v: T) => void
): () => void {
	if (!instance.securityId) return () => { };
	const channelName = `${instance.securityId}-${channelType}`;
	const callbacks = activeChannels.get(channelName);
	const add = () => {
		const list = activeChannels.get(channelName);
		if (!list?.includes(callback as StreamCallback)) list?.push(callback as StreamCallback);
	};
	// If callbacks exist, this channel is already active
	if (callbacks) {
		add();
	} else {
		// New channel, set up normally
		activeChannels.set(channelName, [callback as StreamCallback]);
		subscribe(channelName);
	}
	const cached = latestValue.get(channelName);
	if (cached) {
		queueMicrotask(() => callback(cached as T));
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
		streamInfo.update((v) => {
			return {
				...v,
				replayActive: true,
				replayPaused: false,
				startTimestamp: timestampToUse,
				timestamp: timestampToUse
			};
		});
		chartEventDispatcher.set({ event: 'replay', chartId: 'all' as unknown as number, data: null });
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
	streamInfo.update((v: ReplayInfo) => {
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
		const realtimeRequest: SubscriptionRequest = {
			action: 'realtime'
		};
		socket.send(JSON.stringify(realtimeRequest));

		// When leaving replay mode, we need to update streamInfo
		streamInfo.update((v) => {
			// Calculate current live timestamp using serverTimeOffset
			// This ensures we immediately show the correct live time
			const currentLiveTime = Date.now() + (v.serverTimeOffset || 0);

			return {
				...v,
				replayActive: false,
				replayPaused: false,
				timestamp: currentLiveTime,
				lastUpdateTime: Date.now()
			};
		});

		// Notify charts to update
		chartEventDispatcher.set({
			event: 'realtime',
			chartId: 'all' as unknown as number,
			data: null
		});
	}
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
			action: 'nextOpen' as unknown as 'replay'
		};
		socket.send(JSON.stringify(stopRequest));
	}
	chartEventDispatcher.set({ event: 'replay', chartId: 'all' as unknown as number, data: null });
}
export function setExtended(extendedHours: boolean) {
	if (socket?.readyState === WebSocket.OPEN) {
		const stopRequest: SubscriptionRequest = {
			action: 'setExtended' as unknown as 'replay',
			extendedHours: extendedHours
		};
		socket.send(JSON.stringify(stopRequest));
	}
	streamInfo.update((r: ReplayInfo) => ({ ...r, extendedHours: extendedHours }));
}

// Function to subscribe to global SEC filings feed
export function addGlobalSECFilingsStream(callback: StreamCallback): () => void {
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
