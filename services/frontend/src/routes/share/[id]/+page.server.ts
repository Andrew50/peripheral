import { publicRequest } from '$lib/utils/helpers/backend';
import type { ServerLoad } from '@sveltejs/kit';

function isBotUserAgent(userAgent: string): boolean {
	const botPatterns = [
		'facebookexternalhit',
		'facebot',
		'twitterbot',
		'linkedinbot',
		'discordbot',
		'whatsapp',
		'applebot',
		'googlebot',
		'bingbot',
		'slurp',
		'bot',
		'crawler',
		'spider',
		'scraper',
		'preview'
	];

	const lowerUA = userAgent.toLowerCase();
	return botPatterns.some((pattern) => lowerUA.includes(pattern));
}

export const load: ServerLoad = async ({ params, request, url }) => {
	// Detect if this is a bot/scraper request
	const userAgent = request.headers.get('user-agent') || '';
	const isBot = isBotUserAgent(userAgent);
	const referrer = request.headers.get('referer') || '';
	const path = url.pathname;

	try {
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
					timestamp: new Date().toISOString()
				}
			}),
			signal: AbortSignal.timeout(2000) // 2 second timeout
		});
		if (!response.ok) {
			const errorBody = await response.text().catch(() => 'Unable to read error body');
            console.error(`Failed to log page view: ${response.status} ${response.statusText} - ${errorBody}`);
		}

		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
			first_query: string;
		}

		const result = await publicRequest<ConversationSnippetResponse>('getConversationSnippet', {
			conversation_id: params.id
		});

		const cleanTitle = result.title || 'Peripheral';

		const origin = process.env.ORIGIN || request.url.split('/').slice(0, 3).join('/');
		const shareUrl = `${origin}/share/${params.id}`;
		const ogImageUrl = `${origin}/og/${params.id}`;

		return {
			conversationId: params.id,
			isBot,
			meta: {
				title: cleanTitle,
				shareUrl,
				ogImageUrl
			}
		};
	} catch (error) {
		console.error('Error loading conversation for preview:', error);

		// Return fallback meta data
		const origin = process.env.ORIGIN || request.url.split('/').slice(0, 3).join('/');
		const fallbackUrl = `${origin}/share/${params.id}`;
		const fallbackImageUrl = `${origin}/og/${params.id}`;

		return {
			conversationId: params.id,
			isBot,
			meta: {
				title: 'Peripheral',
				shareUrl: fallbackUrl,
				ogImageUrl: fallbackImageUrl
			}
		};
	}
};
