export let base_url: string;
const pollInterval = 300; // Poll every 100ms
import { goto } from '$app/navigation';

// Default value for server-side rendering - use environment variable if available
base_url = typeof process !== 'undefined' && process.env?.BACKEND_URL
    ? process.env.BACKEND_URL
    : 'http://localhost:5058';

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
        return Promise.reject(`Connection error: Could not connect to backend at ${base_url}. Please check if the backend service is running and accessible.`);
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
        goto('/login');
        throw new Error('Authentication failed');
    }
    if (!response.ok) {
        const errorMessage = await response.text();
        console.error('Error:', errorMessage);
        return Promise.reject(errorMessage);
    }

    return response.json() as Promise<T>;
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
        // For public viewing mode, silently handle 401 errors
        console.warn('Authentication required for:', func);
        throw new Error('Authentication required');
    } else if (response.ok) {
        const result = (await response.json()) as T;

        // Check if this is a cancellation response
        if (typeof result === 'object' && result !== null && 'type' in result && (result as any).type === 'cancelled') {
            // Throw a special cancellation error that can be handled differently
            const cancelError = new Error('Request was cancelled');
            (cancelError as any).cancelled = true;
            throw cancelError;
        }

        if (verbose) {
            // Removed console.log(payload)
        }
        return result;
    } else {
        const errorMessage = await response.text();
        console.error('payload: ', payload, 'error: ', errorMessage);
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
        goto('/login');
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
        goto('/login');
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
