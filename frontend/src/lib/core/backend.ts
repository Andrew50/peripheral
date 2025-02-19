export let base_url: string;
const pollInterval = 300; // Poll every 100ms
import { goto } from '$app/navigation';

base_url = 'http://localhost:5057';

if (typeof window !== 'undefined') {
    if (window.location.hostname === 'localhost') {
        /*const url = new URL(window.location.origin);
        url.port = "5057";
        base_url = url.toString();
        base_url = base_url.substring(0, base_url.length - 1);*/
        base_url = 'http://localhost:5057'; //dev
    } else {
        base_url = window.location.origin; //prod
    }
}

export async function publicRequest<T>(func: string, args: any): Promise<T> {
    const payload = JSON.stringify({
        func: func,
        args: args
    })
    const response = await fetch(`${base_url}/public`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload
    });
    if (response.ok) {
        const result = await response.json() as T
        console.log("payload: ", payload, "result: ", result)
        return result;
    } else {
        const errorMessage = await response.text()
        console.error("payload: ", payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}

export async function privateFileRequest<T>(func: string, file: File, additionalArgs: object = {}): Promise<T> {
    let authToken;
    try {
        authToken = sessionStorage.getItem("authToken")
    } catch {
        return
    }
    const formData = new FormData();
    formData.append('file', file);
    formData.append('func', func);
    formData.append('args', JSON.stringify(additionalArgs));

    const headers = {
        'Content-Type': 'multipart/form-data',
        ...(authToken ? { 'Authorization': authToken } : {}),
    };
    const response = await fetch(`${base_url}/private-upload`, {
        method: 'POST',
        headers: {
            ...(authToken ? { 'Authorization': authToken } : {})
        },
        body: formData
    }).catch((e) => {
        return Promise.reject(e);
    });

    if (response.status === 401) {
        goto('/login');
    }
    if (!response.ok) {
        const errorMessage = await response.text();
        console.error("Error:", errorMessage);
        return Promise.reject(errorMessage);
    }

    return response.json();
}

export async function privateRequest<T>(func: string, args: any, verbose = false): Promise<T> {
    let authToken;
    try {
        authToken = sessionStorage.getItem("authToken")
    } catch {
        return
    }
    const headers = {
        'Content-Type': 'application/json',
        ...(authToken ? { 'Authorization': authToken } : {}),
    };
    const payload = {
        func: func,
        args: args
    }
    const response = await fetch(`${base_url}/private`, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload)
    }).catch((e) => {
        return Promise.reject(e);
    });

    if (response.status === 401) {
        goto('/login')
    } else if (response.ok) {
        const result = await response.json() as T
        if (verbose) {
            console.log("payload: ", payload, "result: ", result)
        }
        return result;
    } else {
        const errorMessage = await response.text()
        console.error("payload: ", payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}

export async function queueRequest<T>(func: string, args: any, verbose = true): Promise<T> {
    let authToken;
    authToken = sessionStorage.getItem("authToken")
    const headers = {
        'Content-Type': 'application/json',
        ...(authToken ? { 'Authorization': authToken } : {}),
    };
    const payload = {
        func: func,
        args: args
    }
    const response = await fetch(`${base_url}/queue`, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload)
    }).catch();

    if (response.status === 401) {
        goto('/login')
    } else if (!response.ok) {
        const errorMessage = await response.text();
        console.error("Error queuing task:", errorMessage);
        return Promise.reject(errorMessage);
    }
    if (verbose) {
        console.log(payload)
    }
    const result = await response.json()
    const taskId = result.taskId
    return new Promise<T>((resolve, reject) => {
        const intervalID = setInterval(async () => {
            const pollResponse = await fetch(`${base_url}/poll`, {
                method: 'POST',
                headers: headers,
                body: JSON.stringify({ taskId: taskId })
            }).catch()
            if (!pollResponse.ok) {
                const errorMessage = await pollResponse.text();
                console.error("Error polling task:", errorMessage);
                clearInterval(intervalID);
                reject(errorMessage);
                return
            }
            const data = await pollResponse.json();
            console.log(data)
            if (data.status === 'completed') {
                console.log("Task completed:", data.result);
                clearInterval(intervalID); // Stop polling
                resolve(data.result);
            } else if (data.status === 'error') {
                console.error("Task error:", data.error);
                clearInterval(intervalID); // Stop polling
                reject(data.error);
            }
        }, pollInterval);
    });
}
