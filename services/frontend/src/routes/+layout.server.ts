import type { ServerLoad } from '@sveltejs/kit';

export const load: ServerLoad = async ({ cookies }) => {
    const authToken = cookies.get('authToken');

    if (!authToken) {
        return {
            isAuthenticated: false,
            user: null
        };
    }

    try {
        const backendUrl =
            process.env.BACKEND_URL ||
            (process.env.KUBERNETES_SERVICE_HOST ? 'http://backend:5058' : 'http://localhost:5058');

        const response = await fetch(`${backendUrl}/private`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                Authorization: authToken
            },
            body: JSON.stringify({
                func: 'verifyAuth',
                args: {}
            })
        });

        if (!response.ok) {
            // Token invalid; clear cookies and treat as unauthenticated
            cookies.delete('authToken', { path: '/' });
            cookies.delete('profilePic', { path: '/' });
            return {
                isAuthenticated: false,
                user: null
            };
        }

        const profilePic = cookies.get('profilePic') || '';

        return {
            isAuthenticated: true,
            user: {
                profilePic,
                authToken
            }
        };
    } catch {
        // On any error, do not crash; just treat as unauthenticated
        return {
            isAuthenticated: false,
            user: null
        };
    }
};


