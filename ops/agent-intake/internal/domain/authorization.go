package domain

import (
	"context"
	"strings"
)

func (s *Service) source(ctx context.Context, event Event) (Repository, Source, Command, error) {
	if ctx.Err() != nil {
		return Repository{}, Source{}, Command{}, Unavailable
	}
	if event.Repository.ID <= 0 || !loginPattern.MatchString(event.Repository.Owner) || !repoPattern.MatchString(event.Repository.Name) ||
		event.Repository.Name == "." || event.Repository.Name == ".." || event.IssueNumber <= 0 || event.CommentID <= 0 ||
		event.RunID <= 0 || event.Attempt <= 0 || !ValidSHA(event.WorkflowSHA) {
		return Repository{}, Source{}, Command{}, Invalid
	}

	command, err := Parse(event.Body)
	if err != nil {
		return Repository{}, Source{}, Command{}, err
	}

	repo, err := s.repository.Repository(ctx, event)
	if err != nil {
		return Repository{}, Source{}, Command{}, err
	}
	if repo.ID != event.Repository.ID || !strings.EqualFold(repo.Owner+"/"+repo.Name, event.Repository.Owner+"/"+event.Repository.Name) ||
		repo.Fork || repo.Archived || repo.DefaultBranch == "" {
		return Repository{}, Source{}, Command{}, Denied
	}

	source, err := s.repository.Source(ctx, repo, event)
	if err != nil {
		return Repository{}, Source{}, Command{}, err
	}
	if !source.Open || source.PullRequest || source.CommentID != event.CommentID || source.IssueNumber != event.IssueNumber ||
		source.Body != event.Body || !source.Created.Equal(source.Updated) || source.Created.IsZero() ||
		source.Requester.ID <= 0 || source.Requester != event.Requester {
		return Repository{}, Source{}, Command{}, SourceChanged
	}

	if _, err := s.authorize(ctx, repo, event, source); err != nil {
		return Repository{}, Source{}, Command{}, err
	}

	return repo, source, command, nil
}

func (s *Service) authorize(ctx context.Context, repo Repository, event Event, source Source) (string, error) {
	seen := map[string]bool{}
	requesterRole := ""
	for _, login := range []string{source.Requester.Login, event.Actor, event.RerunActor} {
		if !loginPattern.MatchString(login) {
			return "", Denied
		}
		key := strings.ToLower(login)
		if seen[key] {
			continue
		}
		seen[key] = true

		user, role, err := s.repository.Role(ctx, repo, login)
		if err != nil {
			return "", err
		}
		if (role != "maintain" && role != "admin") || !strings.EqualFold(user.Login, login) || user.ID <= 0 ||
			(key == strings.ToLower(source.Requester.Login) && user.ID != source.Requester.ID) {
			return "", Denied
		}

		if key == strings.ToLower(source.Requester.Login) {
			requesterRole = role
		}
	}
	return requesterRole, nil
}
