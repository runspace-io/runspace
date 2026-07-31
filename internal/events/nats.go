package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/runspace/runspace/internal/contracts"
)

// StreamName remains FORGE_EVENTS so existing JetStream data and consumers are
// not disrupted. New deployments may migrate it to RUNSPACE_EVENTS after a
// coordinated stream migration; subjects are intentionally unchanged.
const StreamName = "FORGE_EVENTS"
const RunspaceStreamName = "RUNSPACE_EVENTS"

// Publisher is the event-driven boundary used by chat, runs, and Git.
// Implementations must publish facts only after the owning state change is
// durable (the database outbox does this in production).
type Publisher interface {
	Publish(context.Context, contracts.EventEnvelope) error
}

type Replayer interface {
	Replay(context.Context, string, string) ([]contracts.EventEnvelope, error)
}

type NATSPublisher struct {
	connection *nats.Conn
	jetStream  nats.JetStreamContext
}

func ConnectNATS(url string) (*NATSPublisher, error) {
	connection, err := nats.Connect(url,
		nats.Timeout(5*time.Second),
		nats.Name("runspace-event-publisher"),
	)
	if err != nil {
		return nil, fmt.Errorf("connect NATS: %w", err)
	}
	jetStream, err := connection.JetStream()
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}
	publisher := &NATSPublisher{connection: connection, jetStream: jetStream}
	if err := publisher.EnsureStream(); err != nil {
		publisher.Close()
		return nil, err
	}
	return publisher, nil
}

func (p *NATSPublisher) EnsureStream() error {
	_, err := p.jetStream.AddStream(&nats.StreamConfig{
		Name:      StreamName,
		Subjects:  []string{"evt.>"},
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		MaxAge:    7 * 24 * time.Hour,
		MaxMsgs:   5_000_000,
		Discard:   nats.DiscardOld,
	})
	if err != nil && !strings.Contains(err.Error(), "stream name already in use") {
		return fmt.Errorf("ensure NATS stream: %w", err)
	}
	return nil
}

func (p *NATSPublisher) Publish(ctx context.Context, event contracts.EventEnvelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := p.jetStream.Publish(eventSubject(event), payload); err != nil {
		return fmt.Errorf("publish event %q: %w", event.ID, err)
	}
	return nil
}

// Replay scans the durable event stream and returns workspace events after the
// supplied cursor. JetStream preserves stream order; the cursor is exclusive.
func (p *NATSPublisher) Replay(ctx context.Context, workspaceID, lastEventID string) ([]contracts.EventEnvelope, error) {
	subscription, err := p.jetStream.SubscribeSync("evt.>", nats.DeliverAll(), nats.AckNone())
	if err != nil {
		return nil, err
	}
	defer func() { _ = subscription.Unsubscribe() }()
	result := make([]contracts.EventEnvelope, 0)
	all := make([]contracts.EventEnvelope, 0)
	seenCursor := strings.TrimSpace(lastEventID) == ""
	for {
		message, err := subscription.NextMsg(100 * time.Millisecond)
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				if !seenCursor {
					return all, nil
				}
				return result, nil
			}
			return nil, err
		}
		var event contracts.EventEnvelope
		if json.Unmarshal(message.Data, &event) != nil || event.WorkspaceID != workspaceID {
			continue
		}
		all = append(all, event)
		if !seenCursor {
			seenCursor = event.ID == lastEventID
			continue
		}
		result = append(result, event)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}

func eventSubject(event contracts.EventEnvelope) string {
	eventType := strings.NewReplacer(" ", "-", "/", ".").Replace(strings.TrimSpace(event.Type))
	eventType = strings.TrimSuffix(eventType, fmt.Sprintf(".v%d", event.Version))
	return fmt.Sprintf("evt.%s.v%d", eventType, event.Version)
}

func (p *NATSPublisher) Subscribe(durable, filter string, handler func(context.Context, contracts.EventEnvelope) error) (*nats.Subscription, error) {
	if strings.TrimSpace(durable) == "" || strings.TrimSpace(filter) == "" || handler == nil {
		return nil, fmt.Errorf("durable, filter, and handler are required")
	}
	return p.jetStream.Subscribe(filter, func(message *nats.Msg) {
		var event contracts.EventEnvelope
		if err := json.Unmarshal(message.Data, &event); err != nil {
			_ = message.Term()
			return
		}
		if err := handler(context.Background(), event); err != nil {
			_ = message.Nak()
			return
		}
		_ = message.Ack()
	}, nats.Durable(durable), nats.ManualAck(), nats.AckExplicit())
}

func (p *NATSPublisher) Close() {
	if p == nil || p.connection == nil {
		return
	}
	_ = p.connection.Drain()
	p.connection.Close()
}
