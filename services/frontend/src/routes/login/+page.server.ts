import { redirect, type ServerLoad } from '@sveltejs/kit';

export const load: ServerLoad = async ({ cookies }) => {
    const authToken = cookies.get('authToken');

    if (authToken) {
        let isValid = false;
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
                body: JSON.stringify({ func: 'verifyAuth', args: {} })
            });
            isValid = response.ok;
        } catch {
            // Ignore network errors; fall through to show login page
        }
        if (isValid) {
            throw redirect(302, '/app');
        }
    }

    return {};
};


