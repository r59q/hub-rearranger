package api

import (
	"github.com/r59q/hub-rearranger/services/agents/internal/api/contract"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
)

func toConventionDTO(convention domain.Convention) contract.AssignmentConvention {
	roles := make([]contract.AssignmentPolicyRequiredRoles, 0, len(convention.Assignment.RequiredRoles))
	for _, role := range convention.Assignment.RequiredRoles {
		roles = append(roles, contract.AssignmentPolicyRequiredRoles(role))
	}
	return contract.AssignmentConvention{
		Version:              contract.AssignmentConventionVersion(convention.Version),
		ProfileCatalogPath:   contract.AssignmentConventionProfileCatalogPath(convention.ProfileCatalogPath),
		ProfileSchemaVersion: contract.AssignmentConventionProfileSchemaVersion(convention.ProfileSchemaVersion),
		Assignment: contract.AssignmentPolicy{
			Event:           contract.AssignmentPolicyEvent(convention.Assignment.Event),
			SourceKind:      contract.AssignmentPolicySourceKind(convention.Assignment.SourceKind),
			CommandTemplate: contract.AssignmentPolicyCommandTemplate(convention.Assignment.CommandTemplate),
			Authority:       contract.AssignmentPolicyAuthority(convention.Assignment.Authority),
			RequiredRoles:   roles,
			ProfileRevision: contract.ProfileRevisionPolicy{
				Format:                contract.ProfileRevisionPolicyFormat(convention.Assignment.ProfileRevision.Format),
				DefaultBranchAncestor: contract.ProfileRevisionPolicyDefaultBranchAncestor(convention.Assignment.ProfileRevision.DefaultBranchAncestor),
			},
		},
	}
}
