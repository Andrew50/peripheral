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


//export async function queueRequest
