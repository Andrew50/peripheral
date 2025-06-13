import { publicRequest } from '$lib/utils/helpers/backend';
import type { ServerLoad } from '@sveltejs/kit';

export const load: ServerLoad = async ({ params }) => {
	try {
		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
		}

		const result = await publicRequest<ConversationSnippetResponse>('getConversationSnippet', {
			conversation_id: params.id
		});

		// Generate clean text for meta descriptions
		const cleanTitle = result.title || 'Atlantis Chat';
		const cleanResponse = (result.first_response || 'AI-powered market analysis and conversation')
			.replace(/<[^>]*>/g, '') // Remove HTML tags
			.substring(0, 160); // Limit to 160 characters for meta description

		const shareUrl = `${process.env.ORIGIN || 'http://localhost:5173'}/share/${params.id}`;
		const ogImageUrl = `${process.env.ORIGIN || 'http://localhost:5173'}/og/${params.id}`;

		return {
			conversationId: params.id,
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
		const fallbackUrl = `${process.env.ORIGIN || 'http://localhost:5173'}/share/${params.id}`;
		const fallbackImageUrl = `${process.env.ORIGIN || 'http://localhost:5173'}/og/${params.id}`;
		
		return {
			conversationId: params.id,
			meta: {
				title: 'Atlantis Chat',
				description: 'AI-powered market analysis and conversation',
				shareUrl: fallbackUrl,
				ogImageUrl: fallbackImageUrl
			}
		};
	}
}; 