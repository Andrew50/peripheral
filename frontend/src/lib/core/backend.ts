export let base_url: string;
const pollInterval = 300; // Poll every 100ms
import { goto } from '$app/navigation';

// Default value for server-side rendering
base_url = 'http://localhost:5057';

if (typeof window !== 'undefined') {
    // For client-side code
    if (window.location.hostname === 'localhost') {
        // In development
        const url = new URL(window.location.origin);
        url.port = '5057'; // Switch to backend port
        base_url = url.toString();
        if (base_url.endsWith('/')) {
            base_url = base_url.substring(0, base_url.length - 1);
        }
        console.log('Using development backend URL:', base_url);
    } else {
        // In production always use the current origin
        base_url = window.location.origin;
        console.log('Using production backend URL:', base_url);
    }
}

// For debugging
console.log('Backend base_url set to:', base_url);

export async function publicRequest<T>(func: string, args: Record<string, unknown>): Promise<T> {
    // Log what's being sent
    console.log(`Making ${func} request with args:`, args);

    const payload = JSON.stringify({
        func: func,
        args: args
    });
    const response = await fetch(`${base_url}/public`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload
    });
    if (response.ok) {
        const result = (await response.json()) as T;
        console.log('payload: ', payload, 'result: ', result);
        return result;
    } else {
        const errorMessage = await response.text();
        console.error('payload: ', payload, 'error: ', errorMessage);
        return Promise.reject(errorMessage);
    }
}

export async function privateFileRequest<T>(
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

    const response = await fetch(`${base_url}/private-upload`, {
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
    verbose = false
): Promise<T> {
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
        body: JSON.stringify(payload)
    }).catch((e) => {
        return Promise.reject(e);
    });

    if (response.status === 401) {
        goto('/login');
        throw new Error('Authentication failed');
    } else if (response.ok) {
        const result = (await response.json()) as T;
        if (verbose) {
            console.log('payload: ', payload, 'result: ', result);
        }
        return result;
    } else {
        const errorMessage = await response.text();
        console.error('payload: ', payload, 'error: ', errorMessage);
        return Promise.reject(errorMessage);
    }
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
        console.log(payload);
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
            console.log(data);
            if (data.status === 'completed') {
                console.log('Task completed:', data.result);
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
