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

export const load: ServerLoad = async ({ params, request }) => {
	// Detect if this is a bot/scraper request
	const userAgent = request.headers.get('user-agent') || '';
	const isBot = isBotUserAgent(userAgent);

	try {
		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
			first_query: string;
		}

		const result = await publicRequest<ConversationSnippetResponse>('getConversationSnippet', {
			conversation_id: params.id
		});

		const cleanTitle = result.title || 'Atlantis';

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
				title: 'Atlantis',
				shareUrl: fallbackUrl,
				ogImageUrl: fallbackImageUrl
			}
		};
	}
};
