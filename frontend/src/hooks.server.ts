import type { Handle } from '@sveltejs/kit';

// Account page data includes identity and an anti-forgery nonce. Never
// let a shared intermediary cache one visitor's account into another's page.
export const handle: Handle = async ({ event, resolve }) => {
	const response = await resolve(event);
	response.headers.set('cache-control', 'no-store');
	return response;
};
