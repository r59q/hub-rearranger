import { describe, expect, it, vi } from 'vitest';
import type { ActionResult, SubmitFunction } from '@sveltejs/kit';
import { createOptimisticSubmit } from './optimistic-submit';

async function runSubmit(submit: SubmitFunction, result: ActionResult, update = vi.fn()) {
	const complete = submit({} as Parameters<SubmitFunction>[0]);
	if (!complete) throw new Error('Expected a submit completion callback.');
	await complete({ result, update } as Parameters<typeof complete>[0]);
}

describe('createOptimisticSubmit', () => {
	it('applies immediately and reconciles a successful response', async () => {
		const onMutate = vi.fn();
		const onSuccess = vi.fn();
		const onSettled = vi.fn();
		const result = { type: 'success', status: 200, data: {} } satisfies ActionResult;
		const submit = createOptimisticSubmit({
			value: 7,
			onMutate,
			onSuccess,
			onSettled
		});

		await runSubmit(submit, result);

		expect(onMutate).toHaveBeenCalledWith(7);
		expect(onSuccess).toHaveBeenCalledWith(7, result);
		expect(onSettled).toHaveBeenCalledWith(7, result);
	});

	it('runs error and settled hooks for a failed response', async () => {
		const onError = vi.fn();
		const onSettled = vi.fn();
		const result = { type: 'failure', status: 422, data: {} } satisfies ActionResult;
		const submit = createOptimisticSubmit({
			value: 7,
			onMutate: vi.fn(),
			onError,
			onSettled
		});

		await runSubmit(submit, result);

		expect(onError).toHaveBeenCalledWith(7, result);
		expect(onSettled).toHaveBeenCalledWith(7, result);
	});

	it('always settles when applying the response throws', async () => {
		const onSettled = vi.fn();
		const result = {
			type: 'error',
			status: 500,
			error: new Error('failed')
		} satisfies ActionResult;
		const submit = createOptimisticSubmit({
			value: 7,
			onMutate: vi.fn(),
			onSettled
		});

		await expect(
			runSubmit(submit, result, vi.fn().mockRejectedValue(new Error('failed')))
		).rejects.toThrow('failed');
		expect(onSettled).toHaveBeenCalledWith(7, result);
	});
});
