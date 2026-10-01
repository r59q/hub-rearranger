import Ajv2020 from 'ajv/dist/2020.js';
import schema from './profile-schema.gen.json';
import {
	repositoryProfilesStateValues,
	repositoryProfilesCatalog_pathValues,
	profileDiagnosticCodeValues,
	type components
} from './agents-contract.gen';

export type RepositoryProfiles = components['schemas']['RepositoryProfiles'];

// Validate profile semantics with the same canonical schema as Go and Python.
// Ajv never evaluates repository code; only the trusted schema is compiled.
const ajv = new Ajv2020({ strict: false, allErrors: false });
const isCatalog = ajv.compile(schema);

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isRepositoryProfiles(value: unknown): value is RepositoryProfiles {
	if (
		!isRecord(value) ||
		Object.keys(value).length !== 7 ||
		typeof value.repository !== 'string' ||
		!/^[A-Za-z0-9][A-Za-z0-9-]{0,38}\/[A-Za-z0-9_.-]{1,100}$/.test(value.repository) ||
		typeof value.default_branch !== 'string' ||
		value.default_branch.length === 0 ||
		typeof value.revision !== 'string' ||
		!/^[0-9a-f]{40}$/.test(value.revision) ||
		!repositoryProfilesCatalog_pathValues.some((expected) => expected === value.catalog_path) ||
		!repositoryProfilesStateValues.some((expected) => expected === value.state) ||
		!Array.isArray(value.profiles) ||
		!Array.isArray(value.diagnostics)
	) {
		return false;
	}
	if (
		!value.diagnostics.every(
			(issue) =>
				isRecord(issue) &&
				Object.keys(issue).length === 3 &&
				profileDiagnosticCodeValues.some((expected) => expected === issue.code) &&
				typeof issue.path === 'string' &&
				typeof issue.message === 'string' &&
				issue.message.length > 0
		)
	) {
		return false;
	}
	if (value.state !== 'valid') {
		if (value.profiles.length !== 0 || value.diagnostics.length === 0) {
			return false;
		}
		const codes = value.diagnostics.map((issue) => (issue as Record<string, unknown>).code);
		if (value.state === 'missing') {
			return codes.length === 1 && codes[0] === 'MISSING_FILE';
		}
		if (value.state === 'unsupported') {
			return codes.length === 1 && codes[0] === 'UNSUPPORTED_VERSION';
		}
		return !codes.includes('MISSING_FILE') && !codes.includes('UNSUPPORTED_VERSION');
	}
	if (
		value.diagnostics.length !== 0 ||
		!value.profiles.every(
			(profile) =>
				isRecord(profile) &&
				Object.keys(profile).length === 3 &&
				typeof profile.id === 'string' &&
				profile.revision === value.revision
		)
	) {
		return false;
	}
	const profiles = value.profiles as { id: string; configuration: unknown }[];
	if (new Set(profiles.map((profile) => profile.id)).size !== profiles.length) {
		return false;
	}
	return isCatalog({
		schema_version: 1,
		profiles: Object.fromEntries(profiles.map((profile) => [profile.id, profile.configuration]))
	});
}
