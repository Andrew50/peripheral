import fs from 'fs/promises';
import path from 'path';
import satori from 'satori';
import { Resvg } from '@resvg/resvg-js';

const WIDTH = 1200;
const HEIGHT = 630;
const CACHE_DIR = '/tmp/og'; // Use /tmp instead of /var/www for development

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
		const { q, plot } = snippetData ?? {
			t: 'Peripheral Chat',
			q: 'what do you think about $RDDT here based on the price action and give a preview of the upcoming earnings',
			a: 'The new best way to trade.',
			plot: ''
		};

		// Use the user query as the main title
		const rawTitle = q || 'Peripheral Query';
		const title = rawTitle.length > 105 ? rawTitle.substring(0, 105) + '...' : rawTitle;

		// ---------- 3) Load background image -----------
		let backgroundImageBase64 = '';
		try {
			const backgroundImagePath = path.join(process.cwd(), 'static', 'opengraph-background.png');
			const backgroundImageBuffer = await fs.readFile(backgroundImagePath);
			backgroundImageBase64 = `data:image/png;base64,${backgroundImageBuffer.toString('base64')}`;
		} catch (error) {
			console.warn('Could not load background image, falling back to gradient:', error);
		}

		// ---------- 4) Render SVG via Satori -----------
		const svg = await satori(
			{
				type: 'div',
				props: {
					style: {
						width: WIDTH,
						height: HEIGHT,
						display: 'flex',
						flexDirection: 'row',
						...(backgroundImageBase64 
							? {
								backgroundImage: `url(${backgroundImageBase64})`,
								backgroundSize: 'cover',
								backgroundPosition: 'center',
								backgroundRepeat: 'no-repeat'
							}
							: {
								background: 'linear-gradient(135deg, #308494 0%, #2c7a85 100%)'
							}
						),
						color: 'white',
						padding: '24px 48px 0px 0px',
						fontFamily: 'Inter',
						position: 'relative',
						borderRadius: '16px'
					},
					children: [
						// Left side - Title area
						{
							type: 'div',
							props: {
								style: {
									width: '415px',
									display: 'flex',
									flexDirection: 'column',
									justifyContent: 'flex-start',
									paddingTop: '106px',
									paddingLeft: '20px',
									paddingRight: '15px'
								},
								children: [
									{
										type: 'h1',
										props: {
											style: {
												fontSize: '44px',
												margin: '0',
												lineHeight: 1.3,
												fontWeight: '400',
												color: '#ffffff',
												letterSpacing: '-0.04em'
											},
											children: title
										}
									}
								]
							}
						},
						// Right side - Content area (for charts, etc.)
						{
							type: 'div',
							props: {
								style: {
									flex: '1',
									display: 'flex',
									flexDirection: 'column',
									justifyContent: 'flex-start',
									alignItems: 'stretch',
									paddingLeft: '0px',
									paddingTop: '106px',
									paddingBottom: '0px',
									height: '100%'
								},
								children: [
									// Plot area - use actual plot if available, otherwise placeholder
									plot ? {
										type: 'div',
										props: {
											style: {
												width: '100%',
												height: '100%',
												borderRadius: '12px 12px 0px 0px',
												display: 'flex',
												alignItems: 'center',
												justifyContent: 'center',
												backgroundImage: `url(data:image/png;base64,${plot})`,
												backgroundSize: 'contain',
												backgroundPosition: 'center',
												backgroundRepeat: 'no-repeat'
											}
										}
									} : {
										type: 'div',
										props: {
											style: {
												width: '100%',
												height: '100%',
												backgroundColor: 'rgba(255, 255, 255, 0.1)',
												borderRadius: '12px 12px 0px 0px',
												display: 'flex',
												alignItems: 'center',
												justifyContent: 'center'
											},
											children: [
												{
													type: 'span',
													props: {
														style: {
															fontSize: '24px',
															color: 'rgba(255, 255, 255, 0.7)',
															textAlign: 'center'
														},
														children: 'Chart Area'
													}
												}
											]
										}
									}
								]
							}
						},
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

		// ---------- 5) Convert SVG → PNG ---------------
		const png = new Resvg(svg).render().asPng();

		// ---------- 6) Write-through cache -------------
		await fs.mkdir(CACHE_DIR, { recursive: true });
		fs.writeFile(filePath, png).catch(() => { }); // fire-and-forget

		// ---------- 7) Return asset --------------------
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
): Promise<{ t: string; q: string; a: string; plot: string } | null> {
	try {
		interface ConversationSnippetResponse {
			title: string;
			first_response: string;
			first_query: string;
			plot: string;
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
			t: result.title || 'Peripheral Chat',
			a: result.first_response || 'The new best way to trade',
			q: result.first_query || 'Peripheral Chat',
			plot: result.plot || ''
		};
	} catch (error) {
		console.error('Error fetching conversation snippet:', error);
		return null;
	}
}
