import { json } from '@sveltejs/kit';
import fs from 'fs/promises';
import path from 'path';

const WAITLIST_FILE = path.join(process.cwd(), 'waitlist.json');

// Initialize waitlist file if it doesn't exist
async function initWaitlistFile() {
	try {
		await fs.access(WAITLIST_FILE);
	} catch {
		await fs.writeFile(WAITLIST_FILE, JSON.stringify({ emails: [] }, null, 2));
	}
}

export async function POST({ request }) {
	try {
		const { email } = await request.json();

		if (!email || !email.includes('@')) {
			return json({ error: 'Invalid email address' }, { status: 400 });
		}

		// Initialize file if needed
		await initWaitlistFile();

		// Read current waitlist
		const data = JSON.parse(await fs.readFile(WAITLIST_FILE, 'utf-8'));

		// Check if email already exists
		if (data.emails.some(entry => entry.email === email)) {
			return json({ error: 'Email already registered' }, { status: 409 });
		}

		// Add new email
		data.emails.push({
			email,
			timestamp: new Date().toISOString()
		});

		// Write back to file
		await fs.writeFile(WAITLIST_FILE, JSON.stringify(data, null, 2));

		return json({ success: true, message: 'Successfully added to waitlist' });
	} catch (error) {
		console.error('Waitlist error:', error);
		return json({ error: 'Internal server error' }, { status: 500 });
	}
}

export async function GET() {
	try {
		await initWaitlistFile();
		const data = JSON.parse(await fs.readFile(WAITLIST_FILE, 'utf-8'));
		return json({ count: data.emails.length });
	} catch (error) {
		console.error('Waitlist error:', error);
		return json({ error: 'Internal server error' }, { status: 500 });
	}
} 