import { createServer } from 'http';
import { handler } from './build/handler.js';
import path from 'path';
import { Transform } from 'stream';

// Create a transform stream to modify HTML
class HtmlTransformer extends Transform {
    constructor(options) {
        super(options);
        this.chunks = [];
    }

    _transform(chunk, encoding, callback) {
        this.chunks.push(chunk);
        callback();
    }

    _flush(callback) {
        let html = Buffer.concat(this.chunks).toString('utf8');

        // Fix SvelteKit base path without adding a base tag
        // This ensures paths are resolved correctly without causing recursive path issues
        html = html.replace('base: ""', 'base: "/"');

        this.push(Buffer.from(html, 'utf8'));
        callback();
    }
}

// Create a custom handler
const customHandler = (req, res) => {
    // Store the original methods
    const originalWriteHead = res.writeHead;
    const originalWrite = res.write;
    const originalEnd = res.end;

    // Log the request for debugging
    console.log(`Request: ${req.url}`);

    // Set proper MIME types based on URL
    if (req.url.endsWith('.js')) {
        res.setHeader('Content-Type', 'application/javascript');
    } else if (req.url.endsWith('.mjs')) {
        res.setHeader('Content-Type', 'application/javascript');
    } else if (req.url.endsWith('.css')) {
        res.setHeader('Content-Type', 'text/css');
    } else if (req.url.includes('/_app/') && !req.url.includes('.css')) {
        res.setHeader('Content-Type', 'application/javascript');
    }

    // Override writeHead to intercept HTML responses
    res.writeHead = function (statusCode, statusMessage, headers) {
        // Call the original method
        return originalWriteHead.apply(this, arguments);
    };

    // Create a transform stream for HTML content
    let transformer = null;

    // Override write method
    res.write = function (chunk, encoding, callback) {
        const contentType = this.getHeader('content-type');

        // If this is an HTML response, use the transformer
        if (contentType && contentType.includes('text/html')) {
            if (!transformer) {
                transformer = new HtmlTransformer();
                transformer.on('data', (data) => {
                    originalWrite.call(res, data);
                });
            }

            transformer.write(chunk, encoding, callback);
            return true;
        }

        // Otherwise, use the original write method
        return originalWrite.apply(this, arguments);
    };

    // Override end method
    res.end = function (chunk, encoding, callback) {
        if (transformer && chunk) {
            transformer.write(chunk, encoding, () => {
                transformer.end();
            });
            return originalEnd.call(this, null, encoding, callback);
        }

        if (transformer) {
            transformer.end();
            return originalEnd.call(this, null, encoding, callback);
        }

        return originalEnd.apply(this, arguments);
    };

    // Pass the request to the SvelteKit handler
    return handler(req, res);
};

const server = createServer(customHandler);

server.listen(3000, '0.0.0.0', () => {
    console.log('SvelteKit server running on port 3000');
}); 