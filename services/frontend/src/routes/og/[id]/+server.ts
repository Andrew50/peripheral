import fs from 'fs/promises';
import path from 'path';
import satori from 'satori';
import { Resvg } from '@resvg/resvg-js';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const WIDTH = 1200;
const HEIGHT = 630;
const CACHE_DIR = '/tmp/og'; // Use /tmp instead of /var/www for development

// Helper function to get logo as base64
async function getLogoBase64(): Promise<string> {
	try {
		const logoPath = path.join(process.cwd(), 'static', 'atlantis_logo_transparent.png');
		const logoBuffer = await fs.readFile(logoPath);
		return logoBuffer.toString('base64');
	} catch (error) {
		console.error('Failed to load logo:', error);
		// Return empty string as fallback
		return '';
	}
}

export async function HEAD({ params }) {
	// Reuse the same meta lookup you do for GET
	const { headers } = await GET({ params });
	// SvelteKit lets you return only headers for HEAD
	return new Response(null, {
		status: 200,
		headers
	});
}
export async function GET({ params }: { params: { id: string } }) {
	try {
		// ---------- 1) Try disk-cache  -----------------
		const filePath = path.join(CACHE_DIR, `${params.id}.png`);
		try {
			const cached = await fs.readFile(filePath);
			return new Response(cached, {
				headers: {
					'Content-Type': 'image/png',
					'Cache-Control': 'public, max-age=31536000, immutable',
					'Content-Length': cached.length.toString()
				}
			});
		} catch {
			// cache miss — fall through
		}

		// ---------- 2) Fetch data ----------------------
		const snippetData = await getChatSnippet(params.id);
		const { t, q, a } = snippetData ?? {
			t: 'Peripheral Chat',
			q: 'Can you get all the dates of the big boeing plane crashes within the last 5 years....',
			a: 'The new best way to trade.'
		};

		// Use the user query as the main title
		const rawTitle = q || 'Peripheral Query';
		const title = rawTitle.length > 95 ? rawTitle.substring(0, 95) + '...' : rawTitle;

		// ---------- 3) Render SVG via Satori -----------
		const svg = await satori(
			{
				type: 'div',
				props: {
					style: {
						width: WIDTH,
						height: HEIGHT,
						display: 'flex',
						flexDirection: 'column',
						background: 'linear-gradient(135deg, #308494 0%, #2c7a85 100%)',
						color: 'white',
						padding: '48px',
						fontFamily: 'Inter',
						position: 'relative',
						borderRadius: '16px'
					},
					children: [
						// Main title at the top
						{
							type: 'h1',
							props: {
								style: {
									fontSize: '72px',
									margin: '0 0 60px 0',
									lineHeight: 1.2,
									fontWeight: '600',
									color: '#ffffff',
									flex: '1'
								},
								children: title
							}
						},
						// Bottom section with logo/branding and arrow
						{
							type: 'div',
							props: {
								style: {
									display: 'flex',
									justifyContent: 'space-between',
									alignItems: 'center',
									width: '100%'
								},
								children: [
									// Left side - Atlantis branding
									{
										type: 'div',
										props: {
											style: {
												display: 'flex',
												alignItems: 'center'
											},
											children: [
												{
													type: 'span',
													props: {
														style: {
															fontSize: '84px',
															fontWeight: '600',
															color: '#ffffff'
														},
														children: 'Peripheral'
													}
												}
											]
										}
									},
									// Right side - Arrow
									{
										type: 'div',
										props: {
											style: {
												width: '108px',
												height: '108px',
												borderRadius: '50%',
												background: '#43daf5',
												display: 'flex',
												alignItems: 'center',
												justifyContent: 'center'
											},
											children: [
												{
													type: 'svg',
													props: {
														viewBox: '0 0 18 18',
														width: '70',
														height: '70',
														style: {
															transform: 'rotate(90deg)',
															fill: '#ffffff'
														},
														children: [
															{
																type: 'path',
																props: {
																	d: 'M7.99992 14.9993V5.41334L4.70696 8.70631C4.31643 9.09683 3.68342 9.09683 3.29289 8.70631C2.90237 8.31578 2.90237 7.68277 3.29289 7.29225L8.29289 2.29225L8.36906 2.22389C8.76184 1.90354 9.34084 1.92613 9.70696 2.29225L14.707 7.29225L14.7753 7.36842C15.0957 7.76119 15.0731 8.34019 14.707 8.70631C14.3408 9.07242 13.7618 9.09502 13.3691 8.77467L13.2929 8.70631L9.99992 5.41334V14.9993C9.99992 15.5516 9.55221 15.9993 8.99992 15.9993C8.44764 15.9993 7.99993 15.5516 7.99992 14.9993Z',
																	fill: '#ffffff'
																}
															}
														]
													}
												}
											]
										}
									}
								]
							}
						}
					]
				}
			},
			{
				width: WIDTH,
				height: HEIGHT,
				fonts: [
					{
						name: 'Inter',
						data: await fetch(
							'https://cdn.jsdelivr.net/fontsource/fonts/inter@latest/latin-400-normal.ttf'
						).then((res) => res.arrayBuffer()),
						weight: 400,
						style: 'normal'
					},
					{
						name: 'Inter',
						data: await fetch(
							'https://cdn.jsdelivr.net/fontsource/fonts/inter@latest/latin-600-normal.ttf'
						).then((res) => res.arrayBuffer()),
						weight: 600,
						style: 'normal'
					}
				]
			}
		);

		// ---------- 4) Convert SVG → PNG ---------------
		const png = new Resvg(svg).render().asPng();

		// ---------- 5) Write-through cache -------------
		await fs.mkdir(CACHE_DIR, { recursive: true });
		fs.writeFile(filePath, png).catch(() => {}); // fire-and-forget

		// ---------- 6) Return asset --------------------
		return new Response(png, {
			headers: {
				'Content-Type': 'image/png',
				'Cache-Control': 'public, max-age=31536000, immutable',
				'Content-Length': png.length.toString(),
				'Last-Modified': new Date().toUTCString(),
				'Access-Control-Allow-Origin': '*',
				'Access-Control-Allow-Methods': 'GET, HEAD, OPTIONS',
				'Access-Control-Allow-Headers': 'Content-Type'
			}
		});
	} catch (error) {
		console.error('Error generating OG image:', error);

		// Return the actual error for debugging
		return new Response(
			`Image generation failed: ${error instanceof Error ? error.message : String(error)}`,
			{
				status: 500,
				headers: { 'Content-Type': 'text/plain' }
			}
		);
	}
}

async function getChatSnippet(
	conversationId: string
): Promise<{ t: string; q: string; a: string } | null> {
	try {
		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
			first_query: string;
		}
		// For server-side requests in Docker, use the backend service name
		const backendUrl = process.env.BACKEND_URL || 'http://backend:5058';

		const response = await fetch(`${backendUrl}/public`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				func: 'getConversationSnippet',
				args: { conversation_id: conversationId }
			})
		});

		if (!response.ok) {
			throw new Error(`HTTP ${response.status}: ${await response.text()}`);
		}

		const result = (await response.json()) as ConversationSnippetResponse;
		console.log(result);
		return {
			t: result.title || 'Atlantis Chat',
			a: result.first_response || 'The new best way to trade',
			q: result.first_query || 'Atlantis Chat'
		};
	} catch (error) {
		console.error('Error fetching conversation snippet:', error);
		return null;
	}
}
