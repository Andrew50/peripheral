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
	return botPatterns.some(pattern => lowerUA.includes(pattern));
}

export const load: ServerLoad = async ({ params, request }) => {
	// Detect if this is a bot/scraper request
	const userAgent = request.headers.get('user-agent') || '';
	const isBot = isBotUserAgent(userAgent);

	try {
		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
		}

		const result = await publicRequest<ConversationSnippetResponse>('getConversationSnippet', {
			conversation_id: params.id
		});

		// Generate clean text for meta descriptions
		const cleanTitle = result.title || 'Atlantis';
		const cleanResponse = (result.first_response || 'The new best way to trade.')
			.replace(/<[^>]*>/g, '') // Remove HTML tags
			.substring(0, 160); // Limit to 160 characters for meta description

		const origin = process.env.ORIGIN;
		const shareUrl = `${origin}/share/${params.id}`;
		const ogImageUrl = `${origin}/og/${params.id}`;

		return {
			conversationId: params.id,
			isBot,
			meta: {
				title: cleanTitle,
				description: cleanResponse,
				shareUrl,
				ogImageUrl
			}
		};
	} catch (error) {
		console.error('Error loading conversation for preview:', error);
		
		// Return fallback meta data
		const origin = process.env.ORIGIN;
		const fallbackUrl = `${origin}/share/${params.id}`;
		const fallbackImageUrl = `${origin}/og/${params.id}`;
		
		return {
			conversationId: params.id,
			isBot,
			meta: {
				title: 'Atlantis',
				description: 'The new best way to trade.',
				shareUrl: fallbackUrl,
				ogImageUrl: fallbackImageUrl
			}
		};
	}
}; 