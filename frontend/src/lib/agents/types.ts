// View data is a projection of validated server responses, never a GitHub payload.
export type ProfileDiagnostic = { code: string; path: string; message: string };

export type ReadinessState =
	'configuration_missing' | 'verification_pending' | 'runtime_verified' | 'verification_failed';

export type ProfileEvidence = {
	runUrl: string;
	verifiedAt: string;
	cliVersion: string | null;
	requestedModel: string;
	requestedEffort: string;
	effectiveModel: string | null;
	effectiveEffort: string | null;
	outcome: 'verified' | 'failed';
	reason: string;
};

export type AgentProfile = {
	id: string;
	name: string;
	description: string;
	enabled: boolean;
	role: string;
	model: string;
	effort: string;
	runner: string;
	authority: string;
	sandbox: string;
	network: boolean;
	checks: string[];
	reviewComments: boolean;
	readiness: {
		state: ReadinessState;
		nextAction: string;
		diagnostics: ProfileDiagnostic[];
		evidence: ProfileEvidence | null;
	} | null;
};

export type AgentWorkspace = {
	repository: string;
	branch: string;
	revision: string;
	catalogUrl: string;
	commitUrl: string;
	diagnosticUrl: string;
	loadedAt: string;
	state: 'ready' | 'catalog_missing' | 'catalog_invalid' | 'catalog_unsupported' | 'error';
	profiles: AgentProfile[];
	diagnostics: ProfileDiagnostic[];
	error: string | null;
	readinessError: string | null;
	inconsistent: boolean;
};
