
import { logout } from '$lib/auth';

export let base_url: string;
const pollInterval = 300; // Poll every 100ms

// Determine base URL for server-side (Node) context first
if (typeof process !== 'undefined') {
	// 1. Explicit override always wins
	if (process.env?.BACKEND_URL) {
		base_url = process.env.BACKEND_URL;
	}
	// 2. If running inside a Kubernetes pod (KUBERNETES_SERVICE_HOST is automatically
	//    injected by Kubernetes) fall back to the ClusterIP service name.
	else if (process.env?.KUBERNETES_SERVICE_HOST) {
		// "backend" is the Service name defined in k8s manifests.
		base_url = 'http://backend:5058';
	}
}

if (typeof window !== 'undefined') {
	// For client-side code
	if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
		// In development, always use the backend server URL
		base_url = 'http://localhost:5058';
	} else {
		// In production always use the current origin
		base_url = window.location.origin;
	}
}

export async function publicRequest<T>(func: string, args: Record<string, unknown>): Promise<T> {
	const payload = JSON.stringify({
		func: func,
		args: args
	});

	try {
		const response = await fetch(`${base_url}/public`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: payload
		});

		if (response.ok) {
			const result = (await response.json()) as T;
			return result;
		} else {
			const errorMessage = await response.text();
			console.error('Request failed with status:', response.status, 'Error:', errorMessage);
			return Promise.reject(`Server error: ${response.status} - ${errorMessage}`);
		}
	} catch (error) {
		console.error('Connection error:', error);
		// Check if the backend URL is correct
		console.error('Current backend URL:', base_url);
		return Promise.reject(
			`Connection error: Could not connect to backend at ${base_url}. Please check if the backend service is running and accessible.`
		);
	}
}

export async function uploadRequest<T>(
	func: string,
	file: File,
	additionalArgs: object = {}
): Promise<T> {
	let authToken;
	try {
		authToken = sessionStorage.getItem('authToken');
	} catch {
		throw new Error('Failed to get auth token');
	}
	const formData = new FormData();
	formData.append('file', file);
	formData.append('func', func);
	formData.append('args', JSON.stringify(additionalArgs));

	// Create headers object with optional authorization
	const headers: HeadersInit = {};
	if (authToken) {
		headers['Authorization'] = authToken;
	}

	const response = await fetch(`${base_url}/upload`, {
		method: 'POST',
		headers,
		body: formData
	}).catch((e) => {
		return Promise.reject(e);
	});

	if (response.status === 401) {
		logout('/login');
		throw new Error('Authentication failed');
	}
	if (!response.ok) {
		const errorMessage = await response.text();
		console.error('Error:', errorMessage);
		return Promise.reject(errorMessage);
	}

	return response.json() as Promise<T>;
}

// Smart chart request function that automatically chooses public or private endpoint based on auth
export async function chartRequest<T>(
	func: string,
	args: Record<string, unknown>,
	verbose = false,
	keepalive = false,
	signal?: AbortSignal
): Promise<T> {
	// Skip API calls during SSR to prevent crashes
	if (typeof window === 'undefined') {
		return {} as T; // Return empty data during SSR
	}

	// Check if user is authenticated
	let authToken;
	try {
		authToken = sessionStorage.getItem('authToken');
	} catch {
		// If we can't access sessionStorage, treat as unauthenticated
		authToken = null;
	}
	if (authToken) {
		// User is authenticated - use private endpoint with full features
		return privateRequest<T>(func, args, verbose, keepalive, signal);
	} else {
		// User is not authenticated - use public endpoint with limited features
		return publicRequest<T>(func, args);
	}
}

export async function privateRequest<T>(
	func: string,
	args: Record<string, unknown>,
	verbose = false,
	keepalive = false,
	signal?: AbortSignal
): Promise<T> {
	// Skip API calls during SSR to prevent crashes
	if (typeof window === 'undefined') {
		return {} as T; // Return empty data during SSR
	}

	//console.log(`privateRequest called: func=${func}, args=`, args);

	let authToken;
	try {
		authToken = sessionStorage.getItem('authToken');
	} catch {
		throw new Error('Failed to get auth token');
	}
	const headers = {
		'Content-Type': 'application/json',
		...(authToken ? { Authorization: authToken } : {})
	};
	const payload = {
		func: func,
		args: args
	};

	//console.log(`Making request to ${base_url}/private with payload:`, payload);
	const response = await fetch(`${base_url}/private`, {
		method: 'POST',
		headers: headers,
		body: JSON.stringify(payload),
		keepalive: keepalive,
		signal: signal
	}).catch((e) => {
		return Promise.reject(e);
	});


	if (response.status === 401) {
		// Clear auth state and redirect to login page for authentication errors
		console.warn('Authentication required for:', func);
		logout('/login');
		throw new Error('Authentication required');
	} else if (response.ok) {
		const result = (await response.json()) as T;

		// Check if this is a cancellation response
		if (
			typeof result === 'object' &&
			result !== null &&
			'type' in result &&
			(result as { type: string }).type === 'cancelled'
		) {
			// Throw a special cancellation error that can be handled differently
			const cancelError = new Error('Request was cancelled') as Error & { cancelled: boolean };
			cancelError.cancelled = true;
			throw cancelError;
		}

		if (verbose) {
			// Removed console.log(payload)
		}
		return result;
	} else {
		const errorMessage = await response.text();
		console.error(
			`Error for ${func} - Status: ${response.status}, payload:`,
			payload,
			'error:',
			errorMessage
		);
		return Promise.reject(errorMessage);
	}

	// This should never be reached, but TypeScript requires a return
	throw new Error('Unexpected end of function');
}

export async function queueRequest<T>(
	func: string,
	args: Record<string, unknown>,
	verbose = true
): Promise<T> {
	let authToken;
	try {
		authToken = sessionStorage.getItem('authToken');
		if (!authToken) {
			throw new Error('No auth token found');
		}
	} catch (error) {
		window.location.href = '/login';
		throw new Error(
			'Authentication failed: ' + (error instanceof Error ? error.message : 'Unknown error')
		);
	}

	const headers = {
		'Content-Type': 'application/json',
		Authorization: authToken
	};
	const payload = {
		func: func,
		args: args
	};
	const response = await fetch(`${base_url}/queue`, {
		method: 'POST',
		headers: headers,
		body: JSON.stringify(payload)
	}).catch();

	if (response.status === 401) {
		logout('/login');
	} else if (!response.ok) {
		const errorMessage = await response.text();
		console.error('Error queuing task:', errorMessage);
		return Promise.reject(errorMessage);
	}
	if (verbose) {
		// Removed console.log(payload)
	}
	const result = await response.json();
	const taskId = result.taskId;
	return new Promise<T>((resolve, reject) => {
		const intervalID = setInterval(async () => {
			const pollResponse = await fetch(`${base_url}/poll`, {
				method: 'POST',
				headers: headers,
				body: JSON.stringify({ taskId: taskId })
			}).catch();
			if (!pollResponse.ok) {
				const errorMessage = await pollResponse.text();
				console.error('Error polling task:', errorMessage);
				clearInterval(intervalID);
				reject(errorMessage);
				return;
			}
			const data = await pollResponse.json();
			if (data.status === 'completed') {
				clearInterval(intervalID); // Stop polling
				resolve(data.result);
			} else if (data.status === 'error') {
				console.error('Task error:', data.error);
				clearInterval(intervalID); // Stop polling
				reject(data.error);
			}
		}, pollInterval);
	});
}

export async function streamingChatRequest<T>(
	func: string,
	args: Record<string, unknown>,
	progressCallback?: (progress: string) => void,
	partialCallback?: (partial: unknown) => void,
	signal?: AbortSignal
): Promise<T> {
	// Skip API calls during SSR to prevent crashes
	if (typeof window === 'undefined') {
		return {} as T; // Return empty data during SSR
	}

	let authToken;
	try {
		authToken = sessionStorage.getItem('authToken');
	} catch {
		throw new Error('Failed to get auth token');
	}

	const headers = {
		'Content-Type': 'application/json',
		...(authToken ? { Authorization: authToken } : {})
	};

	const payload = {
		func: func,
		args: args,
		stream_id: generateStreamId()
	};

	const response = await fetch(`${base_url}/streaming-chat`, {
		method: 'POST',
		headers: headers,
		body: JSON.stringify(payload),
		signal: signal
	}).catch((e) => {
		return Promise.reject(e);
	});

	if (response.status === 401) {
		logout('/login');
		throw new Error('Authentication required');
	} else if (!response.ok) {
		const errorMessage = await response.text();
		console.error('Streaming request failed:', errorMessage);
		return Promise.reject(errorMessage);
	}

	// Handle Server-Sent Events
	return new Promise<T>((resolve, reject) => {
		const reader = response.body?.getReader();
		const decoder = new TextDecoder();
		let buffer = '';

		if (!reader) {
			reject(new Error('Failed to get response reader'));
			return;
		}

		const processStream = async () => {
			try {
				while (true) {
					const { done, value } = await reader.read();

					if (done) {
						break;
					}

					buffer += decoder.decode(value, { stream: true });
					const lines = buffer.split('\n');
					buffer = lines.pop() || ''; // Keep the last incomplete line in buffer

					for (const line of lines) {
						if (line.startsWith('data: ')) {
							try {
								const data = JSON.parse(line.slice(6));

								switch (data.type) {
									case 'progress':
										if (progressCallback && data.content?.message) {
											progressCallback(data.content.message);
										}
										break;
									case 'partial':
										if (partialCallback) {
											partialCallback(data.content);
										}
										break;
									case 'complete':
										resolve(data.content as T);
										return;
									case 'error':
										reject(new Error(data.content?.error || 'Streaming error'));
										return;
								}
							} catch (parseError) {
								console.warn('Failed to parse SSE data:', line, parseError);
							}
						}
					}
				}
			} catch (error) {
				if (signal?.aborted) {
					reject(new Error('Request was cancelled'));
				} else {
					reject(error);
				}
			}
		};

		processStream();
	});
}

// Helper function to generate stream IDs
function generateStreamId(): string {
	return 'stream_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
}
