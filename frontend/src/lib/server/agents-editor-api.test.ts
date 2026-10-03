import { describe, expect, it, vi } from 'vitest';
import { profileEditor, previewProfile, transportDraft } from './agents-editor-api';
import { base, editorDraft, preview } from '../../test/bootstrap-fixtures';

describe('profile authoring adapter', () => {
	it.each(['extra', 'missing', 'diagnostics', 'revision', 'secret', 'source'])(
		'rejects malformed authoring context: %s',
		async (variant) => {
			const data = { base_revision: base, draft: transportDraft(editorDraft()), diagnostics: [] };
			if (variant === 'extra') {
				Object.assign(data, { access_token: 'synthetic-never-live' });
			}
			if (variant === 'missing') {
				Reflect.deleteProperty(data.draft, 'enabled');
			}
			if (variant === 'diagnostics') {
				Object.assign(data, { diagnostics: [{ code: 'BAD' }] });
			}
			if (variant === 'revision') {
				data.base_revision = 'bad';
			}
			if (variant === 'secret') {
				Object.assign(data.draft, { access_token: 'synthetic-never-live' });
			}
			if (variant === 'source') {
				Object.assign(data.draft, { context_sources: ['unknown'] });
			}
			await expect(
				profileEditor(vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)), 'octo', 'demo')
			).rejects.toThrow('Profile choices are unavailable');
		}
	);
	it('returns an existing-policy conflict with no partial editable draft', async () => {
		const data = {
			base_revision: base,
			draft: null,
			diagnostics: [
				{ code: 'PROFILE_CONFLICT', path: '/model', message: 'Review fixed model policy.' }
			]
		};
		expect(
			await profileEditor(
				vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)),
				'octo',
				'demo'
			)
		).toMatchObject({ draft: null, diagnostics: data.diagnostics });
	});
	it('rejects unbound generated source without forwarding it to the browser', async () => {
		const data = preview();
		data.files[0].content = 'private-sentinel';
		await expect(
			previewProfile(
				vi.fn<typeof fetch>().mockResolvedValue(Response.json(data)),
				'octo',
				'demo',
				editorDraft()
			)
		).rejects.toThrow('invalid review');
	});
});
