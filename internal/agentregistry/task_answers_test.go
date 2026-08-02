package agentregistry

import (
	"context"
	"errors"
	"testing"

	"github.com/runspace/runspace/internal/contracts"
)

func TestAnsweringForwardsToTheAgentAndRecordsWho(t *testing.T) {
	service, publisher, _, answerer := questionService(t)
	askQuestion(t, service)
	grantRole(t, service, "nahid", "operator")
	question, err := service.AnswerTaskQuestion(
		context.Background(), "nahid", "local_session_1", "q_1_7", "once",
	)
	if err != nil {
		t.Fatal(err)
	}
	if answerer.calls != 1 || answerer.question != "q_1_7" || answerer.option != "once" {
		t.Fatalf("answer not forwarded: %+v", answerer)
	}
	if question.Status != QuestionAnswered || question.AnsweredBy != "nahid" ||
		question.AnsweredOption != "once" {
		t.Fatalf("answer not recorded: %+v", question)
	}
	var announced bool
	for _, event := range publisher.events {
		if event.Type == contracts.EventAgentQuestionAnswered {
			announced = true
		}
	}
	if !announced {
		t.Fatal("no agent.question.answered event was published")
	}
}

func TestAnsweringTwiceConflicts(t *testing.T) {
	service, _, _, _ := questionService(t)
	askQuestion(t, service)
	if _, err := service.AnswerTaskQuestion(
		context.Background(), "admin", "local_session_1", "q_1_7", "once",
	); err != nil {
		t.Fatal(err)
	}
	_, err := service.AnswerTaskQuestion(
		context.Background(), "admin", "local_session_1", "q_1_7", "reject",
	)
	if !errors.Is(err, ErrQuestionResolved) {
		t.Fatalf("second answer error=%v", err)
	}
}

func TestAnswerRejectsUnofferedOptionBeforeReachingTheAgent(t *testing.T) {
	service, _, _, answerer := questionService(t)
	askQuestion(t, service)
	if _, err := service.AnswerTaskQuestion(
		context.Background(), "admin", "local_session_1", "q_1_7", "allow_always",
	); err == nil {
		t.Fatal("accepted an option the agent never offered")
	}
	if answerer.calls != 0 {
		t.Fatal("forged option was forwarded to the agent")
	}
}

// A failed hand-off must not look answered, or the UI would stop offering the
// only control that can unblock the agent.
func TestFailedForwardLeavesQuestionOpen(t *testing.T) {
	service, _, _, answerer := questionService(t)
	answerer.err = errors.New("host agent unreachable")
	askQuestion(t, service)
	if _, err := service.AnswerTaskQuestion(
		context.Background(), "admin", "local_session_1", "q_1_7", "once",
	); err == nil {
		t.Fatal("expected the forward failure to surface")
	}
	stored, _, err := service.ListTaskQuestions(context.Background(), "admin", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Status != QuestionOpen {
		t.Fatalf("question no longer answerable: %+v", stored)
	}
}

// When a turn ends without an answer the question must stop being offered.
func TestTurnEndingClosesOpenQuestions(t *testing.T) {
	service, _, _, _ := questionService(t)
	askQuestion(t, service)
	completed := sampleUpdate("All done", "m2")
	completed.Status = "completed"
	if err := service.RecordTaskStream(
		context.Background(), "admin", "local_session_1", completed,
	); err != nil {
		t.Fatal(err)
	}
	stored, _, err := service.ListTaskQuestions(context.Background(), "admin", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Status != QuestionCancelled {
		t.Fatalf("stale question still open: %+v", stored)
	}
}
