package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/r59q/hub-rearranger/services/identity/internal/domain"
	contract "github.com/r59q/hub-rearranger/services/identity/internal/infrastructure/agents/contract"
)

func (p Planner) Assignment(ctx context.Context, repo domain.Repository, number int64) (domain.AssignmentPlan, error) {
	result := domain.AssignmentPlan{}
	request, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(p.URL, "/")+"/v1/repositories/"+url.PathEscape(repo.Owner)+"/"+url.PathEscape(repo.Name)+"/issues/"+strconv.FormatInt(number, 10)+"/assignment", nil)
	if err != nil {
		return result, domain.ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	response, err := p.Client.Do(request)
	if err != nil {
		return result, domain.ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return result, domain.ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 {
		return result, domain.ErrUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var projection contract.IssueAssignment
	if decoder.Decode(&projection) != nil || decoder.Decode(new(any)) != io.EOF {
		return result, domain.ErrUnavailable
	}
	return domain.AssignmentPlan{Repository: projection.Repository, RepositoryID: projection.RepositoryId, Number: projection.Number, LastCommentID: projection.LastCommentId, Title: projection.Title, Revision: projection.ProfileRevision, Command: projection.Command, Assignable: projection.Assignable}, nil
}
