package agentregistry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/contracts"
)

type memoryQuestionStore struct{ questions map[string]TaskQuestion }

func (s *memoryQuestionStore) UpsertAgentTaskQuestion(
	_ context.Context, question TaskQuestion,
) error {
	if s.questions == nil {
		s.questions = map[string]TaskQuestion{}
	}
	s.questions[question.TaskID+"/"+question.ID] = question
	return nil
}

func (s *memoryQuestionStore) ListAgentTaskQuestions(
	_ context.Context, taskID string,
) ([]TaskQuestion, error) {
	items := make([]TaskQuestion, 0)
	for _, question := range s.questions {
		if question.TaskID == taskID {
			items = append(items, question)
		}
	}
	return items, nil
}

func (s *memoryQuestionStore) GetAgentTaskQuestion(
	_ context.Context, taskID, questionID string,
) (TaskQuestion, error) {
	question, ok := s.questions[taskID+"/"+questionID]
	if !ok {
		return TaskQuestion{}, ErrTaskUnavailable
	}
	return question, nil
}

func (s *memoryQuestionStore) CancelOpenAgentTaskQuestions(
	_ context.Context, taskID string, at time.Time,
) error {
	for key, question := range s.questions {
		if question.TaskID == taskID && question.Status == QuestionOpen {
			question.Status, question.UpdatedAt = QuestionCancelled, at
			s.questions[key] = question
		}
	}
	return nil
}

type recordingAnswerer struct {
	calls    int
	question string
	option   string
	err      error
}

func (a *recordingAnswerer) Answer(
	_ context.Context, _ AgentTask, questionID, optionID string,
) error {
	a.calls++
	a.question, a.option = questionID, optionID
	return a.err
}

func questionService(t *testing.T) (
	*Service, *recordingPublisher, *memoryQuestionStore, *recordingAnswerer,
) {
	t.Helper()
	service, publisher, _ := streamService(t)
	questions := &memoryQuestionStore{}
	answerer := &recordingAnswerer{}
	service.SetTaskQuestionStore(questions)
	service.SetQuestionAnswerer(answerer)
	return service, publisher, questions, answerer
}

func askedUpdate() TaskStreamUpdate {
	update := sampleUpdate("Reading main.go", "m1")
	update.Status = "waiting_approval"
	update.Question = &TaskQuestion{
		ID: "q_1_7", Title: "Run rm -rf build/",
		Options: []QuestionOption{
			{ID: "once", Name: "Allow once", Kind: "allow_once"},
			{ID: "reject", Name: "Reject", Kind: "reject_once"},
		},
	}
	return update
}

func askQuestion(t *testing.T, service *Service) {
	t.Helper()
	if err := service.RecordTaskStream(
		context.Background(), "admin", "local_session_1", askedUpdate(),
	); err != nil {
		t.Fatal(err)
	}
}

func grantRole(t *testing.T, service *Service, principal, role string) {
	t.Helper()
	if _, err := service.GrantTaskAccess(context.Background(), "admin", TaskGrant{
		TaskID: "local_session_1", WorkspaceID: "ws_1", AgentID: "local_agent_abc",
		PrincipalID: principal, Role: role,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestQuestionIsStoredAndAnnouncedWithoutItsText(t *testing.T) {
	service, publisher, questions, _ := questionService(t)
	askQuestion(t, service)
	stored, _, err := service.ListTaskQuestions(context.Background(), "admin", "local_session_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Status != QuestionOpen ||
		stored[0].Title != "Run rm -rf build/" || len(stored[0].Options) != 2 {
		t.Fatalf("question not stored: %+v", questions.questions)
	}
	var asked bool
	for _, event := range publisher.events {
		if event.Type != contracts.EventAgentQuestionAsked {
			continue
		}
		asked = true
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			t.Fatal(err)
		}
		// The title describes what the agent is about to do inside a private
		// chat, and the bus reaches every workspace member.
		if strings.Contains(string(payload), "rm -rf") {
			t.Fatalf("question text leaked onto the bus: %s", payload)
		}
	}
	if !asked {
		t.Fatal("no agent.question.asked event was published")
	}
}

// Answering is the one place a non-owner redirects an agent, so it needs an
// explicit grant rather than mere read or contribute access.
func TestAnsweringRequiresApproveOrControl(t *testing.T) {
	for _, testCase := range []struct {
		role    string
		allowed bool
	}{
		{"viewer", false},
		{"contributor", false},
		{"approver", true},
		{"operator", true},
	} {
		t.Run(testCase.role, func(t *testing.T) {
			service, _, _, answerer := questionService(t)
			askQuestion(t, service)
			grantRole(t, service, "nahid", testCase.role)
			_, err := service.AnswerTaskQuestion(
				context.Background(), "nahid", "local_session_1", "q_1_7", "once",
			)
			if testCase.allowed && err != nil {
				t.Fatalf("%s could not answer: %v", testCase.role, err)
			}
			if !testCase.allowed {
				if err == nil {
					t.Fatalf("%s answered without authority", testCase.role)
				}
				if answerer.calls != 0 {
					t.Fatalf("%s reached the agent anyway", testCase.role)
				}
			}
		})
	}
}

// A viewer must be able to see why the agent is stopped while being told they
// cannot resolve it, so the UI never offers a control the server would reject.
func TestListReportsWhetherTheCallerMayAnswer(t *testing.T) {
	for _, testCase := range []struct {
		role       string
		answerable bool
	}{
		{"viewer", false},
		{"contributor", false},
		{"approver", true},
		{"operator", true},
	} {
		t.Run(testCase.role, func(t *testing.T) {
			service, _, _, _ := questionService(t)
			askQuestion(t, service)
			grantRole(t, service, "nahid", testCase.role)
			questions, answerable, err := service.ListTaskQuestions(
				context.Background(), "nahid", "local_session_1",
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(questions) != 1 {
				t.Fatalf("%s could not see the question: %+v", testCase.role, questions)
			}
			if answerable != testCase.answerable {
				t.Fatalf("%s can_answer=%v", testCase.role, answerable)
			}
		})
	}
}
