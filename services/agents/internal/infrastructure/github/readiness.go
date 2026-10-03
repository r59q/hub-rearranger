package github

import (
	"context"
	"net/http"
	"path"
	"time"

	gh "github.com/google/go-github/v74/github"
	"github.com/r59q/hub-rearranger/services/agents/internal/domain"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/evidence"
	"github.com/r59q/hub-rearranger/services/agents/internal/infrastructure/profiles"
)

type ReadinessReader struct {
	client   *gh.Client
	download *http.Client
	decoder  *evidence.Decoder
}

// Download uses a separate unauthenticated client; the GitHub token must never
// follow an artifact redirect to storage. HTTPS is required, with no redirects.
func NewReadinessReader(client *gh.Client, decoder *evidence.Decoder) *ReadinessReader {
	return &ReadinessReader{client: client, decoder: decoder, download: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (r *ReadinessReader) ReadReadiness(ctx context.Context, repo domain.Repository, catalog domain.ProfileCatalog) (domain.ReadinessFacts, error) {
	result := domain.ReadinessFacts{MissingFiles: []string{}}
	var diagnosticID int64
	for _, filePath := range domain.ReadinessFiles {
		content, exists, err := r.readFile(ctx, repo, filePath, catalog.Revision)
		if err != nil {
			return result, err
		}
		if !exists {
			result.MissingFiles = append(result.MissingFiles, filePath)
			continue
		}
		if filePath != domain.AssignmentWorkflowPath && filePath != domain.DiagnosticWorkflowPath {
			continue
		}

		workflow, response, err := r.client.Actions.GetWorkflowByFileName(ctx, repo.Owner, repo.Name, path.Base(filePath))
		if err != nil && (response == nil || response.StatusCode != http.StatusNotFound) {
			return result, classify(err, response)
		}
		facts := workflowFacts(content, filePath == domain.DiagnosticWorkflowPath)
		facts.Active = err == nil && workflow.GetID() > 0 && workflow.GetState() == "active" && workflow.GetPath() == filePath
		if filePath == domain.AssignmentWorkflowPath {
			result.Assignment = facts
		} else {
			result.Diagnostic = facts
			diagnosticID = workflow.GetID()
		}
	}

	// Missing workflow configuration has its own actionable state. Avoid querying
	// Actions evidence when the diagnostic workflow is absent or unregistered.
	if !result.Diagnostic.Active {
		return result, nil
	}

	origin, artifact, err := r.latestEvidence(ctx, repo, catalog.DefaultBranch, diagnosticID)
	if err != nil {
		return result, err
	}

	result.Origin = origin
	result.Evidence = artifact
	return result, nil
}

func (r *ReadinessReader) readFile(ctx context.Context, repo domain.Repository, filePath, revision string) ([]byte, bool, error) {
	content, directory, response, err := r.client.Repositories.GetContents(ctx, repo.Owner, repo.Name, filePath, &gh.RepositoryContentGetOptions{Ref: revision})
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, classify(err, response)
	}
	if content == nil || directory != nil || content.GetType() != "file" || content.GetTarget() != "" || content.GetSubmoduleGitURL() != "" || content.GetSize() <= 0 || content.GetSize() > domain.ReadinessFileMaxBytes {
		return nil, false, nil
	}

	decoded, err := content.GetContent()
	if err != nil || content.GetEncoding() != "base64" || len(decoded) > domain.ReadinessFileMaxBytes {
		return nil, false, nil
	}
	return []byte(decoded), true, nil
}

func workflowFacts(content []byte, diagnostic bool) domain.WorkflowFacts {
	result := domain.WorkflowFacts{}
	value, valid := profiles.ParseDocument(content)
	if !valid {
		return result
	}
	workflow, ok := value.(map[string]any)
	if !ok {
		return result
	}
	events, ok := workflow["on"].(map[string]any)
	if !ok {
		return result
	}

	if diagnostic {
		_, result.TriggerPresent = events["workflow_dispatch"]
		result.TriggerPresent = result.TriggerPresent && len(events) == 1
	} else {
		comments, _ := events["issue_comment"].(map[string]any)
		types, _ := comments["types"].([]any)
		for _, kind := range types {
			if kind == "created" {
				result.TriggerPresent = true
			}
		}
	}

	jobs, _ := workflow["jobs"].(map[string]any)
	_, authorize := jobs["authorize"].(map[string]any)
	target := "patch"

	if diagnostic {
		target = "diagnostic"
	}
	job, targetPresent := jobs[target].(map[string]any)
	result.JobsPresent = authorize && targetPresent
	if !diagnostic {
		_, publish := jobs["publish"].(map[string]any)
		result.JobsPresent = result.JobsPresent && publish
	}

	labels, _ := job["runs-on"].([]any)
	for _, label := range labels {
		if text, ok := label.(string); ok {
			result.RunnerLabels = append(result.RunnerLabels, text)
		}
	}
	return result
}
