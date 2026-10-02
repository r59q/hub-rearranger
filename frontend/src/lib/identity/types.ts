export interface IdentityState {
	state: 'disabled' | 'signed_out' | 'authenticated' | 'reconnect_required' | 'unavailable';
	user: { id: number; login: string } | null;
	csrf: string | null;
	expiresAt: string | null;
}
