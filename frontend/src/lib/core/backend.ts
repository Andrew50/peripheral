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

export async function publicRequest<T>(func: string, args: any): Promise<T> {
    const payload = JSON.stringify({
        func: func,
        args: args
    })
    const response = await fetch(`${base_url}/public`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload});
    if (response.ok){
        const result = await response.json() as T
        console.log("payload: ",payload, "result: ", result)
        return result;
    }else{
        const errorMessage = await response.text()
        console.error("payload: ",payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}


export async function privateRequest<T>(func: string, args: any): Promise<T> {
    let authToken;
    authToken = sessionStorage.getItem("authToken")
    const headers = {
        'Content-Type': 'application/json',
        ...(authToken ? { 'Authorization': authToken} : {}),
    };
    const payload = {
        func: func,
        args: args
    }
    const response = await fetch(`${base_url}/private`, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(payload)
    }).catch();
    if (response.ok){
        const result = await response.json() as T
        console.log("payload: ",payload, "result: ", result)
        return result;
    }else{
        const errorMessage = await response.text()
       console.error("payload: ",payload, "error: ", errorMessage)
        return Promise.reject(errorMessage);
    }
}

export async function queueRequest<T>(func: string, args: any): Promise<T> {
    const payload = JSON.stringify({
        func: func,
        args: args
    });
    const queueResponse = await fetch(`${base_url}/queue`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: payload
    });
    if (!queueResponse.ok) {
        const errorMessage = await queueResponse.text();
        console.error("Error queuing task:", errorMessage);
        return Promise.reject(errorMessage);
    }
    const { taskID } = await queueResponse.json();
    console.log("Task queued with ID:", taskID);
    return new Promise<T>((resolve, reject) => {
        const eventSource = new EventSource(`${base_url}/${taskID}`);
        eventSource.onmessage = (event) => {
            const data = JSON.parse(event.data);
            if (data.status === 'completed') {
                console.log("Task completed:", data.result);
                eventSource.close();
                resolve(data.result);
            } else if (data.status === 'error') {
                console.error("Task error:", data.error);
                eventSource.close();
                reject(data.error);
            }
        };
        eventSource.onerror = (err) => {
            console.error("SSE error:", err);
            eventSource.close();
            reject("An error occurred during SSE connection");
        };
    });
}
