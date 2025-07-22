import type { ServerLoad } from '@sveltejs/kit';

export const load: ServerLoad = async ({ request, url }) => {
    // Collect visitor data
    const userAgent = request.headers.get('user-agent') || '';
    const referrer = request.headers.get('referer') || '';
    const path = url.pathname;
    // Log page view to backend (fire and forget - don't block page load)
    try {
        // Construct backend URL - adjust port/host as needed for your setup
        const backendUrl = process.env.BACKEND_URL || 'http://backend:5058';
        const cfIP = request.headers.get('cf-connecting-ip') || '127.0.0.1';
        const forwarded = request.headers.get('x-forwarded-for') || '127.0.0.1';
        
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
                    ip_address: forwarded,
                    cloudflare_ipv6: cfIP,
                    timestamp: new Date().toISOString(),
                }
            }),
            // Add timeout to prevent hanging
            signal: AbortSignal.timeout(3000) // 3 second timeout
        });
        
        if (!response.ok) {
            const errorBody = await response.text().catch(() => 'Unable to read error body');
            console.error(`Failed to log page view: ${response.status} ${response.statusText} - ${errorBody}`);
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