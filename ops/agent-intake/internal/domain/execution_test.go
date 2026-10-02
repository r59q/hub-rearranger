package domain

import (
	"context"
	"encoding/json"
	"testing"
)

func TestExecutionReconstructsClaimWithoutWritingReceipt(t *testing.T) {
	service, repository, _, event := setup()
	repository.repo.Private = true
	repository.run.Attempt = 1
	decision, err := service.Intake(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	// The artifact contains transport-visible repository fields only.
	data, _ := json.Marshal(decision.Invocation)
	var input Invocation
	_ = json.Unmarshal(data, &input)

	verified, err := service.Reauthorize(context.Background(), event, input)

	if err != nil || verified.BaseSHA != decision.Invocation.BaseSHA || repository.accepted != 1 {
		t.Fatalf("execution did not independently reconstruct the existing claim: %v", err)
	}
}

func TestExecutionRejectsForgedClaimsAndCurrentRevocation(t *testing.T) {
	for _, variant := range []string{"assignment", "base", "attempt", "receipt", "profile", "url", "role", "public", "closed", "policy", "canonical"} {
		t.Run(variant, func(t *testing.T) {
			service, repository, policy, event := setup()
			repository.repo.Private = true
			repository.run.Attempt = 1
			decision, err := service.Intake(context.Background(), event)
			if err != nil {
				t.Fatal(err)
			}
			input := *decision.Invocation
			switch variant {
			case "assignment":
				input.AssignmentID = "42:100"
			case "base":
				input.BaseSHA = input.BaseSHA[:39] + "2"
			case "attempt":
				input.RunAttempt++
			case "receipt":
				input.ReceiptCommentID++
			case "profile":
				input.Profile = json.RawMessage(`{"adapter":{"id":"other"}}`)
			case "url":
				input.RequestURL = "https://github.com/unrelated/repository"
			case "role":
				repository.roles["maintainer"] = "read"
			case "public":
				repository.repo.Private = false
			case "closed":
				repository.source.Open = false
			case "policy":
				policy.failure = ProfileChanged
			case "canonical":
				repository.run.ID--
			}

			_, err = service.Reauthorize(context.Background(), event, input)

			if err == nil || repository.accepted != 1 {
				t.Fatal("forged/revoked execution was accepted or wrote a new receipt")
			}
		})
	}
}

func TestPublicRepositoryExceptionIsExplicitlyScoped(t *testing.T) {
	for _, name := range []string{"hub-rearranger", "other"} {
		service, repository, _, event := setup()
		repository.repo.Owner, repository.repo.Name = "r59q", name
		event.Repository = repository.repo
		repository.run.Attempt = 1
		decision, err := service.Intake(context.Background(), event)
		if err != nil {
			t.Fatal(err)
		}

		_, err = service.Reauthorize(context.Background(), event, *decision.Invocation)

		if (name == "hub-rearranger") != (err == nil) {
			t.Fatalf("public exception leaked outside the operator-approved repository: %v", err)
		}
	}
}
