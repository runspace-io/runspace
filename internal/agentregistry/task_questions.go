package agentregistry

import (
	"context"
	"strings"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

// Question lifecycle. A question is open until someone answers it, the agent
// gives up on it, or the turn ends without it being resolved.
const (
	QuestionOpen      = "open"
	QuestionAnswered  = "answered"
	QuestionCancelled = "cancelled"
)

type QuestionOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type TaskQuestion struct {
	ID             string           `json:"id"`
	TaskID         string           `json:"task_id"`
	Title          string           `json:"title"`
	Options        []QuestionOption `json:"options"`
	Status         string           `json:"status"`
	AnsweredBy     string           `json:"answered_by,omitempty"`
	AnsweredOption string           `json:"answered_option,omitempty"`
	AskedAt        time.Time        `json:"asked_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type TaskQuestionStore interface {
	UpsertAgentTaskQuestion(context.Context, TaskQuestion) error
	ListAgentTaskQuestions(context.Context, string) ([]TaskQuestion, error)
	GetAgentTaskQuestion(context.Context, string, string) (TaskQuestion, error)
	CancelOpenAgentTaskQuestions(context.Context, string, time.Time) error
}

// QuestionAnswerer forwards a resolved question back to the Host Agent that
// parked it. The agent is blocked until this lands.
type QuestionAnswerer interface {
	Answer(context.Context, AgentTask, string, string) error
}

func (s *Service) SetTaskQuestionStore(store TaskQuestionStore)  { s.questionStore = store }
func (s *Service) SetQuestionAnswerer(answerer QuestionAnswerer) { s.answerer = answerer }

// recordTaskQuestion stores a question raised by a Host Agent push and
// announces it. Like transcripts, the title and options stay off the bus: they
// describe what the agent is about to do inside a private chat.
func (s *Service) recordTaskQuestion(
	ctx context.Context, task AgentTask, question *TaskQuestion,
) {
	if s.questionStore == nil {
		return
	}
	now := s.now().UTC()
	if question == nil {
		// The turn moved on without an answer, so nothing is still waiting.
		if task.Status != "waiting_approval" {
			_ = s.questionStore.CancelOpenAgentTaskQuestions(ctx, task.ID, now)
		}
		return
	}
	question.TaskID = task.ID
	question.Status = QuestionOpen
	question.UpdatedAt = now
	if question.AskedAt.IsZero() {
		question.AskedAt = now
	}
	if s.questionStore.UpsertAgentTaskQuestion(ctx, *question) != nil {
		return
	}
	s.publishTaskEvent(ctx, task, contracts.EventAgentQuestionAsked, map[string]any{
		"task_id": task.ID, "thread_id": task.ThreadID, "owner_id": task.OwnerID,
		"agent_id": task.AgentID, "question_id": question.ID,
	}, question.AskedAt)
}

// ListTaskQuestions reports the questions on a task and whether this caller may
// answer them, so the UI can show a viewer why the agent is stopped without
// offering a control that would be rejected.
func (s *Service) ListTaskQuestions(
	ctx context.Context, callerID, taskID string,
) ([]TaskQuestion, bool, error) {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || (!contains(permissions, "task.view") && task.OwnerID != callerID) {
		return nil, false, ErrTaskUnavailable
	}
	answerable := canAnswer(permissions, task, callerID)
	if s.questionStore == nil {
		return []TaskQuestion{}, answerable, nil
	}
	questions, err := s.questionStore.ListAgentTaskQuestions(ctx, task.ID)
	return questions, answerable, err
}

// AnswerTaskQuestion unblocks an agent on someone else's behalf.
//
// This is the one place a non-owner changes what an agent does, so it demands
// an explicit grant: "approver" (task.approve) or "operator" (task.control).
// task.contribute is deliberately not enough — sending a message is not the
// same authority as approving a command the agent is about to run.
func (s *Service) AnswerTaskQuestion(
	ctx context.Context, callerID, taskID, questionID, optionID string,
) (TaskQuestion, error) {
	task, permissions, err := s.taskAccess(ctx, callerID, taskID)
	if err != nil || !canAnswer(permissions, task, callerID) {
		return TaskQuestion{}, ErrTaskUnavailable
	}
	if s.questionStore == nil || s.answerer == nil {
		return TaskQuestion{}, ErrInvalidInput
	}
	question, err := s.questionStore.GetAgentTaskQuestion(ctx, task.ID, questionID)
	if err != nil {
		return TaskQuestion{}, err
	}
	if question.Status != QuestionOpen {
		return TaskQuestion{}, ErrQuestionResolved
	}
	if optionID != "" && !offersOption(question.Options, optionID) {
		return TaskQuestion{}, ErrInvalidInput
	}
	if err := s.answerer.Answer(ctx, task, questionID, optionID); err != nil {
		return TaskQuestion{}, err
	}
	question.Status = QuestionAnswered
	question.AnsweredBy = callerID
	question.AnsweredOption = optionID
	question.UpdatedAt = s.now().UTC()
	if err := s.questionStore.UpsertAgentTaskQuestion(ctx, question); err != nil {
		return TaskQuestion{}, err
	}
	s.publishTaskEvent(ctx, task, contracts.EventAgentQuestionAnswered, map[string]any{
		"task_id": task.ID, "thread_id": task.ThreadID, "owner_id": task.OwnerID,
		"agent_id": task.AgentID, "question_id": questionID, "answered_by": callerID,
	}, question.UpdatedAt)
	return question, nil
}

func canAnswer(permissions []string, task AgentTask, callerID string) bool {
	return task.OwnerID == callerID ||
		contains(permissions, "task.approve") || contains(permissions, "task.control")
}

func offersOption(options []QuestionOption, optionID string) bool {
	for _, option := range options {
		if option.ID == optionID {
			return true
		}
	}
	return false
}

func normalizeQuestion(question *TaskQuestion) *TaskQuestion {
	if question == nil {
		return nil
	}
	question.ID = strings.TrimSpace(question.ID)
	question.Title = strings.TrimSpace(question.Title)
	if question.ID == "" || len(question.Options) == 0 {
		return nil
	}
	return question
}
