import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import evidenceSchema from './evidence-schema.gen.json';
import { isRepositoryProfiles } from './agents-profiles-response';
import {
	profileReadinessStateValues,
	readinessDiagnosticCodeValues,
	repositoryReadinessCatalog_stateValues,
	profileDiagnosticCodeValues,
	type components
} from './agents-contract.gen';

export type RepositoryReadiness = components['schemas']['RepositoryReadiness'];

const ajv = new Ajv2020({ strict: false, allErrors: false });
addFormats(ajv, ['date-time']);
const isEvidence = ajv.compile(evidenceSchema);

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function diagnostics(value: unknown, codes: readonly string[]): boolean {
	return (
		Array.isArray(value) &&
		value.every(
			(issue) =>
				isRecord(issue) &&
				Object.keys(issue).length === 3 &&
				typeof issue.code === 'string' &&
				codes.includes(issue.code) &&
				typeof issue.path === 'string' &&
				typeof issue.message === 'string' &&
				issue.message.length > 0
		)
	);
}

// Only safe contract data reaches view code. Evidence is displayed solely for
// matching verified/failed states; configuration or pending states cannot carry it.
export function isRepositoryReadiness(value: unknown): value is RepositoryReadiness {
	if (
		!isRecord(value) ||
		Object.keys(value).length !== 6 ||
		typeof value.repository !== 'string' ||
		!/^[A-Za-z0-9][A-Za-z0-9-]{0,38}\/[A-Za-z0-9_.-]{1,100}$/.test(value.repository) ||
		typeof value.default_branch !== 'string' ||
		value.default_branch.length === 0 ||
		typeof value.revision !== 'string' ||
		!/^[0-9a-f]{40}$/.test(value.revision) ||
		!repositoryReadinessCatalog_stateValues.some((state) => state === value.catalog_state) ||
		!Array.isArray(value.profiles) ||
		!diagnostics(value.diagnostics, profileDiagnosticCodeValues)
	) {
		return false;
	}

	if (value.catalog_state !== 'valid') {
		return isRepositoryProfiles({
			repository: value.repository,
			default_branch: value.default_branch,
			revision: value.revision,
			catalog_path: '.github/agent-profiles.yml',
			state: value.catalog_state,
			profiles: value.profiles,
			diagnostics: value.diagnostics
		});
	}

	if ((value.diagnostics as unknown[]).length > 0 || value.profiles.length === 0) {
		return false;
	}

	const repository = value.repository;
	const ids = new Set<string>();
	return value.profiles.every((profile) => {
		if (
			!isRecord(profile) ||
			typeof profile.id !== 'string' ||
			!/^[a-z][a-z0-9-]{0,63}$/.test(profile.id) ||
			ids.has(profile.id) ||
			profile.revision !== value.revision ||
			!profileReadinessStateValues.some((state) => state === profile.state) ||
			typeof profile.runner_label !== 'string' ||
			profile.runner_label.length === 0 ||
			typeof profile.next_action !== 'string' ||
			profile.next_action.length === 0 ||
			!diagnostics(profile.diagnostics, readinessDiagnosticCodeValues)
		) {
			return false;
		}

		ids.add(profile.id);
		const evidence = profile.evidence;
		const matched = profile.state === 'runtime_verified' || profile.state === 'verification_failed';
		if (Object.keys(profile).length !== (matched ? 7 : 6)) {
			return false;
		}
		if (!matched) {
			return evidence === undefined && (profile.diagnostics as unknown[]).length > 0;
		}

		if (
			!isRecord(evidence) ||
			!isEvidence(evidence) ||
			typeof evidence.repository !== 'string' ||
			evidence.repository.toLowerCase() !== repository.toLowerCase() ||
			evidence.profile_id !== profile.id ||
			evidence.profile_revision !== value.revision ||
			evidence.runner_label !== profile.runner_label
		) {
			return false;
		}

		return profile.state === 'runtime_verified'
			? evidence.outcome === 'verified' && (profile.diagnostics as unknown[]).length === 0
			: evidence.outcome === 'failed' && (profile.diagnostics as unknown[]).length > 0;
	});
}
