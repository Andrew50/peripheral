import type { ServerLoad } from '@sveltejs/kit';

export const load: ServerLoad = async ({ request, getClientAddress, cookies, url }) => {
    // Collect visitor data
    const ip = getClientAddress();
    const userAgent = request.headers.get('user-agent') || '';
    const referrer = request.headers.get('referer') || '';
    const path = url.pathname;
    // Log page view to backend (fire and forget - don't block page load)
    try {
        // Construct backend URL - adjust port/host as needed for your setup
        const backendUrl = process.env.BACKEND_URL || 'http://backend:5058';
        
        const response = await fetch(`${backendUrl}/frontend/server`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Peripheral-Frontend-Key': 'williamsIsTheBestLiberalArtsCollege!1@', // TODO: Move to environment variable
            },
            body: JSON.stringify({
                func: 'logSplashScreenView',
                args: {
                    path: path,
                    referrer: referrer,
                    user_agent: userAgent,
                    ip_address: ip,
                    timestamp: new Date().toISOString(),
                }
            }),
            // Add timeout to prevent hanging
            signal: AbortSignal.timeout(2000) // 2 second timeout
        });
        
        if (!response.ok) {
            console.error(`Failed to log page view: ${response.status} ${response.statusText}`);
        }
    } catch (error) {
        // Log error but don't fail page load
        console.error('Error logging page view:', error);
    }

    // Return data for the page component
    return {
        visitLogged: true
    };
}; 