import type { ActionResult, SubmitFunction } from '@sveltejs/kit';

type OptimisticSubmitOptions<Value> = {
	value: Value;
	onMutate: (value: Value) => void;
	onSuccess?: (value: Value, result: Extract<ActionResult, { type: 'success' }>) => void;
	onError?: (value: Value, result: Exclude<ActionResult, { type: 'success' }>) => void;
	onSettled: (value: Value, result: ActionResult) => void;
};

export function createOptimisticSubmit<Value>({
	value,
	onMutate,
	onSuccess,
	onError,
	onSettled
}: OptimisticSubmitOptions<Value>): SubmitFunction {
	return () => {
		onMutate(value);
		return async ({ result, update }) => {
			try {
				await update();
				if (result.type === 'success') {
					onSuccess?.(value, result);
				} else {
					onError?.(value, result);
				}
			} finally {
				onSettled(value, result);
			}
		};
	};
}
