import {
	assignmentConventionProfile_catalog_pathValues,
	assignmentConventionProfile_schema_versionValues,
	assignmentConventionVersionValues,
	assignmentPolicyAuthorityValues,
	assignmentPolicyCommand_templateValues,
	assignmentPolicyEventValues,
	assignmentPolicyRequired_rolesValues,
	assignmentPolicySource_kindValues,
	profileRevisionPolicyFormatValues,
	type components
} from './agents-contract.gen';

export type AssignmentConvention = components['schemas']['AssignmentConvention'];

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

// Expected enum values come from OpenAPI generation, not a second policy list.
export function isAssignmentConvention(value: unknown): value is AssignmentConvention {
	if (!isRecord(value) || !isRecord(value.assignment)) {
		return false;
	}
	const policy = value.assignment;
	if (!isRecord(policy.profile_revision) || !Array.isArray(policy.required_roles)) {
		return false;
	}
	const revision = policy.profile_revision;
	return (
		Object.keys(value).length === 4 &&
		Object.keys(policy).length === 6 &&
		Object.keys(revision).length === 2 &&
		assignmentConventionVersionValues.some((expected) => expected === value.version) &&
		assignmentConventionProfile_catalog_pathValues.some(
			(expected) => expected === value.profile_catalog_path
		) &&
		assignmentConventionProfile_schema_versionValues.some(
			(expected) => expected === value.profile_schema_version
		) &&
		assignmentPolicyEventValues.some((expected) => expected === policy.event) &&
		assignmentPolicySource_kindValues.some((expected) => expected === policy.source_kind) &&
		assignmentPolicyCommand_templateValues.some(
			(expected) => expected === policy.command_template
		) &&
		assignmentPolicyAuthorityValues.some((expected) => expected === policy.authority) &&
		policy.required_roles.length === assignmentPolicyRequired_rolesValues.length &&
		new Set(policy.required_roles).size === policy.required_roles.length &&
		policy.required_roles.every((role) =>
			assignmentPolicyRequired_rolesValues.some((expected) => expected === role)
		) &&
		profileRevisionPolicyFormatValues.some((expected) => expected === revision.format) &&
		revision.default_branch_ancestor === true
	);
}
