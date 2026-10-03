<section aria-labelledby="runner-setup-title">
	<h2 id="runner-setup-title">5. Runner setup tutorial</h2>
	<p>
		This adapter uses subscription-backed Codex on a restricted Linux runner outside Hub. Complete
		these manual steps after reviewing and merging the configuration PR.
	</p>
	<ol>
		<li>
			Use an approved private repository and a dedicated non-root host/account. Register a
			repository-scoped runner with the <code>hub-agent-codex</code> label. Labels alone do not restrict
			repository access. The upstream public/shared-runner exception does not transfer.
		</li>
		<li>
			Install the checksum-pinned operator runtime, native Codex <code>0.159.3</code> and matching code-mode
			helper outside checkouts. Establish subscription authentication privately on the host. Start with
			a disabled operator manifest.
		</li>
		<li>
			Run the source-free preflight. Prove that workload code cannot read host authentication,
			inherit host configuration or access the network. Verify the exact <code
				>gpt-6.1-sol/high</code
			> policy with no fallback.
		</li>
		<li>
			Provide a root <code>make check</code> and its offline dependencies. Merge the reviewed files and
			allow GitHub Actions to create pull requests in repository settings.
		</li>
		<li>
			As a current maintainer/admin, run the no-write profile diagnostic from the default branch.
			Verify origin-bound evidence matching the current revision and policy. Renew evidence after
			default-branch changes and within 24 hours.
		</li>
		<li>
			Enable both the private operator manifest and <code>HUB_AGENT_EXECUTION_ENABLED=verified</code
			> only after verification. Assign through the documented GitHub issue comment.
		</li>
	</ol>
	<details>
		<summary>Recovery, rotation and removal</summary>
		<p>
			Disable both gates before replacing a runner or rotating/revoking authentication. Repeat
			isolation and exact-policy checks, then obtain fresh diagnostic evidence before enabling
			again.
		</p>
		<p>
			For a partial assignment, reconcile GitHub results before retrying. Rerun only the original
			authorization job and its dependent jobs while preserving artifacts. Keep the branch, PR and
			evidence for review.
		</p>
		<p>
			The PR includes <code>docs/agent-workflows.md</code> and the operator guides under
			<code>ops/private-runner</code>. Removing Hub leaves the GitHub files and workflow intact.
		</p>
	</details>
</section>

<style>
	li {
		margin-bottom: 0.75rem;
	}
	code {
		overflow-wrap: anywhere;
	}
</style>
