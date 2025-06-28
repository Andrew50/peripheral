import { redirect, type RequestEvent } from '@sveltejs/kit';

export const load = async ({ cookies, url }: RequestEvent) => {
	// Get auth token from cookies
	const authToken = cookies.get('authToken');
	
	// Check for public viewing mode (shared conversations)
	const shareParam = url.searchParams.get('share');
	const isPublicViewing = !!shareParam;
	
	// If public viewing, allow access without auth
	if (isPublicViewing) {
		return {
			isAuthenticated: false,
			isPublicViewing: true,
			sharedConversationId: shareParam,
			user: null
		};
	}
	
	// For non-public viewing, require authentication
	if (!authToken) {
		throw redirect(302, '/login');
	}
	
	// Validate token server-side by making request to backend
	try {
		// Use 'backend' as the hostname, which is the service name in k8s
		const backendUrl = process.env.BACKEND_URL || 'http://backend:5058';
		
		const response = await fetch(`${backendUrl}/private`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				'Authorization': authToken
			},
			body: JSON.stringify({
				func: 'verifyAuth',
				args: {}
			})
		});
		
		if (!response.ok) {
			// Token is invalid, clear it and redirect
			cookies.delete('authToken', { path: '/' });
			throw redirect(302, '/login');
		}
		
		// Token is valid, get user info from cookies for client
		const profilePic = cookies.get('profilePic') || '';
		const username = cookies.get('username') || '';
		
		return {
			isAuthenticated: true,
			isPublicViewing: false,
			sharedConversationId: null,
			user: {
				profilePic,
				username,
				authToken // Pass to client for API calls
			}
		};
		
	} catch (error) {
		// If there's any error (network, etc.), treat as unauthenticated
		console.error('Auth verification failed:', error);
		cookies.delete('authToken', { path: '/' });
		throw redirect(302, '/login');
	}
}; 