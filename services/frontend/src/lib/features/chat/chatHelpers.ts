import { privateRequest } from '$lib/utils/helpers/backend';

export async function generateSharedConversationLink(
	conversationId: string
): Promise<string | null> {
	try {
		const response = await privateRequest<{ success: boolean }>('setConversationVisibility', {
			conversation_id: conversationId,
			is_public: true
		});

		if (response.success) {
			return `${window.location.origin}/share/${conversationId}`;
		}
		return null;
	} catch (error) {
		console.error('Error generating shared conversation link:', error);
		return null;
	}
}
