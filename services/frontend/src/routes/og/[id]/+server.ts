import fs from 'fs/promises';
import path from 'path';
import { publicRequest } from '$lib/utils/helpers/backend';

const WIDTH = 1200;
const HEIGHT = 630;
const CACHE_DIR = '/tmp/og'; // Use /tmp instead of /var/www for development

export async function GET({ params }: { params: { id: string } }) {
  try {
    // Dynamic imports for server-side modules
    const satori = (await import('satori')).default;
    const { Resvg } = await import('@resvg/resvg-js');

    // ---------- 1) Try disk-cache  -----------------
    const filePath = path.join(CACHE_DIR, `${params.id}.png`);
    try {
      const cached = await fs.readFile(filePath);
      return new Response(cached, {
        headers: { 
          'Content-Type': 'image/png', 
          'Cache-Control': 'public, max-age=31536000, immutable' 
        }
      });
    } catch { 
      // cache miss — fall through 
    }

    // ---------- 2) Fetch data ----------------------
    const snippetData = await getChatSnippet(params.id);
    const { q, a } = snippetData ?? { q: 'Atlantis', a: 'The new best way to trade.' };

    // Truncate text to fit nicely in the image
    const truncatedQ = q.length > 80 ? q.substring(0, 77) + '...' : q;
    const truncatedA = a.length > 200 ? a.substring(0, 197) + '...' : a;

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
            background: 'linear-gradient(135deg, #0a0a0a 0%, #1a1a1a 100%)',
            color: 'white',
            padding: '60px',
            fontFamily: 'Inter, system-ui, sans-serif',
            position: 'relative'
          },
          children: [
            // Atlantis branding
            {
              type: 'div',
              props: {
                style: {
                  display: 'flex',
                  alignItems: 'center',
                  marginBottom: '40px'
                },
                children: [
                  {
                    type: 'div',
                    props: {
                      style: {
                        fontSize: '24px',
                        fontWeight: 'bold',
                        color: '#3b82f6'
                      },
                      children: 'Atlantis'
                    }
                  }
                ]
              }
            },
            // Main question
            {
              type: 'h1',
              props: {
                style: {
                  fontSize: '48px',
                  margin: '0 0 30px 0',
                  lineHeight: 1.2,
                  fontWeight: 'bold',
                  color: '#ffffff'
                },
                children: truncatedQ
              }
            },
            // Response preview
            {
              type: 'p',
              props: {
                style: {
                  fontSize: '28px',
                  margin: 0,
                  lineHeight: 1.4,
                  color: '#e5e7eb',
                  opacity: 0.9
                },
                children: truncatedA
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
            data: await getInterFont(),
            weight: 400,
            style: 'normal',
          },
          {
            name: 'Inter',
            data: await getInterFont(),
            weight: 700,
            style: 'normal',
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
        'Cache-Control': 'public, max-age=31536000, immutable' 
      }
    });

  } catch (error) {
    console.error('Error generating OG image:', error);
    
    // Return the actual error for debugging
    return new Response(`Image generation failed: ${error.message || error}`, {
      status: 500,
      headers: { 'Content-Type': 'text/plain' }
    });
  }
}

async function getChatSnippet(conversationId: string): Promise<{ q: string; a: string } | null> {
  try {
    interface ConversationSnippetResponse {
      title: string;
      first_response: string;
    }

    const result = await publicRequest<ConversationSnippetResponse>('getConversationSnippet', {
      conversation_id: conversationId
    });

    return {
      q: result.title || 'Atlantis Chat',
      a: result.first_response || 'The new best way to trade'
    };
  } catch (error) {
    console.error('Error fetching conversation snippet:', error);
    return null;
  }
}

async function getInterFont(): Promise<ArrayBuffer> {
  // For development, we'll use a fallback. In production, you'd want to load the actual Inter font
  // This is a minimal font that works with Satori
  return new ArrayBuffer(0);
}
