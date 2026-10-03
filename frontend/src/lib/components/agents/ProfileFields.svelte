<script lang="ts">
	import { contextChoices, type EditorDraft } from '$lib/agents/editor';
	let { draft = $bindable() }: { draft: EditorDraft } = $props();
</script>

<fieldset>
	<legend>1. Profile identity</legend>
	<p>Profile ID: <code>codex-thorough</code> · Role: implementation</p>
	<label>Display name <input name="name" bind:value={draft.name} required maxlength="80" /></label>
	<label
		>Description <textarea
			name="description"
			bind:value={draft.description}
			required
			maxlength="500"
			rows="3"
		></textarea></label
	>
	<small
		>These fields are public display text. Keep credentials and private source out of them.</small
	>
	<label
		>Assignments
		<select
			name="enabled"
			value={String(draft.enabled)}
			onchange={(event) => {
				draft.enabled = event.currentTarget.value === 'true';
			}}
		>
			<option value="true">Enabled after setup and runtime verification</option>
			<option value="false">Disabled — block new assignments and continuation</option>
		</select>
	</label>
</fieldset>

<fieldset>
	<legend>2. Runtime and model policy</legend>
	<dl>
		<dt>Adapter</dt>
		<dd><code>codex-chatgpt-private-runner</code>, contract v1</dd>
		<dt>Runner</dt>
		<dd><code>self-hosted</code>, <code>linux</code>, <code>hub-agent-codex</code></dd>
		<dt>Model and reasoning</dt>
		<dd><code>gpt-6.1-sol/high</code>, no fallback</dd>
		<dt>Assignment trigger</dt>
		<dd>New issue comment (<code>issue_comment.created</code>)</dd>
	</dl>
	<p>
		The installed workflows verify these settings explicitly. Other IDs, models, runner labels and
		adapters require a reviewed adapter change.
	</p>
</fieldset>

<fieldset>
	<legend>3. Live context and continuation</legend>
	{#each contextChoices as choice (choice.id)}
		<label>
			<input
				type="checkbox"
				name="context_sources"
				value={choice.id}
				bind:group={draft.contextSources}
				disabled={choice.required}
			/>
			{choice.label}{choice.required ? ' (required)' : ''}
		</label>
		{#if choice.required}<input type="hidden" name="context_sources" value={choice.id} />{/if}
	{/each}
	<label
		>Trusted review comments
		<select
			name="review_comments"
			value={String(draft.reviewComments)}
			onchange={(event) => {
				draft.reviewComments = event.currentTarget.value === 'true';
			}}
		>
			<option value="true">Permit continuation on the profile’s PR</option>
			<option value="false">Disable review continuation</option>
		</select>
	</label>
	<small
		>Permitting review continuation requires pull request, review thread and related checks context.
		Its execution workflow remains AW-016.</small
	>
	<p>Image context: disabled in v1. Pipeline events and automatic remediation: disabled in v1.</p>
</fieldset>

<fieldset>
	<legend>4. Authority and validation</legend>
	<p>
		<code>branch-draft-pr</code> authority creates a dedicated branch and draft PR. Sandbox:
		<code>workspace-write</code>; workload networking is disabled.
	</p>
	<p>
		Validation: <code>repository-check</code> runs the fixed <code>make check</code>. Failures
		remain visible under <code>draft-with-evidence</code>.
	</p>
	<p>
		Changing configuration requires a new review and fresh runtime evidence. Enabling a profile in
		this draft does not install or enable a runner.
	</p>
</fieldset>

<style>
	fieldset {
		padding: 1rem;
		border: 1px solid var(--pico-muted-border-color);
		border-radius: var(--pico-border-radius);
	}
	dl {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
		gap: 0.5rem 1rem;
	}
	dd {
		margin: 0;
		overflow-wrap: anywhere;
	}
	code {
		overflow-wrap: anywhere;
	}
	@media (max-width: 500px) {
		dl {
			grid-template-columns: minmax(0, 1fr);
		}
		dt {
			font-weight: bold;
		}
	}
</style>
