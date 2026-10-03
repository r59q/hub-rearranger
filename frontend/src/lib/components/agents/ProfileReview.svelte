<script lang="ts">
	import type { BootstrapView } from '$lib/server/agents-bootstrap-api';
	let { preview }: { preview: BootstrapView } = $props();
	let changed = $derived(
		preview.files.filter((file) => ['create', 'update'].includes(file.status))
	);
</script>

<section aria-labelledby="profile-review-title">
	<h2 id="profile-review-title">6. Review the proposed GitHub changes</h2>
	<p>
		Default branch: <code>{preview.default_branch}</code> · reviewed commit:
		<code>{preview.base_revision}</code>
	</p>
	{#if preview.state === 'conflict'}
		<p role="alert">Check these choices or resolve the existing configuration before publishing.</p>
		<ul>
			{#each preview.diagnostics as issue, i (i)}<li>
					<code>{issue.path}</code>: {issue.message}
				</li>{/each}
		</ul>
	{:else if preview.state === 'unchanged'}
		<p role="status">
			The files already match these choices. No PR is needed. Runtime verification remains separate.
		</p>
	{:else}
		<p>
			{changed.length} files will be created or updated. The selected profile’s editable fields change
			explicitly; unrelated profiles and instructions are preserved.
		</p>
	{/if}
	<details>
		<summary>File changes ({changed.length})</summary>
		<ul>
			{#each preview.files as file (file.path)}<li>
					<code>{file.path}</code> — {file.status}
				</li>{/each}
		</ul>
	</details>
	<details open>
		<summary>Full Git diff</summary>
		<!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users need to focus and scroll this bounded diff.) -->
		<pre role="region" tabindex="0" aria-label="Proposed profile diff">{preview.diff ||
				'No changes.'}</pre>
	</details>
	<p>
		The review binds these exact bytes to this commit. Editing choices requires another review. A
		changed default branch requires a fresh review before publication.
	</p>
	<h3>GitHub writes you are approving</h3>
	<ol>
		<li>Create Git file/tree objects for the reviewed changes.</li>
		<li>Create one commit attributed to your signed-in GitHub identity.</li>
		<li>Create a dedicated <code>hub-bootstrap/codex-thorough-…</code> branch.</li>
		<li>
			Create a draft PR into <code>{preview.default_branch}</code>, linked to this editor and the
			manual setup checklist.
		</li>
	</ol>
	<p>
		Hub rechecks user/App access during publication and verifies existing GitHub results before
		retrying. You review and merge on GitHub. Creating this PR does not start an agent or enable
		execution.
	</p>
</section>

<style>
	pre {
		max-height: 28rem;
		overflow: auto;
		font-size: 0.8rem;
	}
	pre:focus-visible {
		outline: 3px solid var(--pico-primary);
	}
	code,
	li {
		overflow-wrap: anywhere;
	}
</style>
