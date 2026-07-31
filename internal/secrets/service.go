package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
)

var (
	ErrInvalid  = errors.New("invalid secret input")
	ErrNotFound = errors.New("secret not found")
)

type ChannelResolver interface {
	GetChannel(context.Context, string, string) (collaboration.Channel, error)
}
type Authorizer interface {
	CanWrite(context.Context, string, string) error
	CanRead(context.Context, string, string) error
}

type Metadata struct {
	Name            string    `json:"name"`
	UpdatedAt       time.Time `json:"updated_at"`
	SourceChannelID string    `json:"source_channel_id,omitempty"`
	Inherited       bool      `json:"inherited,omitempty"`
}
type Store interface {
	Set(context.Context, string, string, string, string) error
	List(context.Context, string, string) ([]Metadata, error)
	Resolve(context.Context, string, string, string) (string, error)
	Delete(context.Context, string, string, string) error
}

type DurableStore interface {
	SetEncrypted(context.Context, string, string, []byte, time.Time) error
	ListEncrypted(context.Context, string) ([]EncryptedMetadata, error)
	ResolveEncrypted(context.Context, string, string) ([]byte, error)
	DeleteEncrypted(context.Context, string, string) error
}

type EncryptedMetadata struct {
	Name      string
	UpdatedAt time.Time
}

type Service struct {
	mu       sync.RWMutex
	channels ChannelResolver
	auth     Authorizer
	key      []byte
	clock    func() time.Time
	values   map[string]map[string]entry
	durable  DurableStore
}

func (s *Service) SetPersistence(store DurableStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.durable = store
}

type entry struct {
	sealed  []byte
	updated time.Time
}

func New(channels ChannelResolver, auth Authorizer, key []byte, clock func() time.Time) (*Service, error) {
	if channels == nil || auth == nil {
		return nil, ErrInvalid
	}
	if len(key) != 32 {
		return nil, errors.New("secret key must be 32 bytes")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{channels: channels, auth: auth, key: append([]byte(nil), key...), clock: clock, values: make(map[string]map[string]entry)}, nil
}

func (s *Service) Set(ctx context.Context, userID, channelID, name, value string) error {
	channel, err := s.authorize(ctx, userID, channelID, true)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" || value == "" {
		return ErrInvalid
	}
	sealed, err := s.seal(value)
	if err != nil {
		return err
	}
	now := s.clock().UTC()
	s.mu.Lock()
	if s.values[channel.ID] == nil {
		s.values[channel.ID] = make(map[string]entry)
	}
	s.values[channel.ID][name] = entry{sealed: sealed, updated: now}
	durable := s.durable
	s.mu.Unlock()
	if durable != nil {
		if err := durable.SetEncrypted(ctx, channel.ID, name, sealed, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, userID, channelID string) ([]Metadata, error) {
	channel, err := s.authorize(ctx, userID, channelID, false)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Metadata)
	for current := &channel; current != nil; current = s.parent(ctx, userID, *current) {
		s.loadDurableMetadata(ctx, current.ID, current.ID != channel.ID, result)
		s.mu.RLock()
		for name, item := range s.values[current.ID] {
			if _, exists := result[name]; !exists {
				result[name] = Metadata{Name: name, UpdatedAt: item.updated, SourceChannelID: current.ID, Inherited: current.ID != channel.ID}
			}
		}
		s.mu.RUnlock()
	}
	out := make([]Metadata, 0, len(result))
	for _, item := range result {
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) Resolve(ctx context.Context, userID, channelID, name string) (string, error) {
	channel, err := s.authorize(ctx, userID, channelID, false)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalid
	}
	for current := &channel; current != nil; current = s.parent(ctx, userID, *current) {
		s.mu.RLock()
		item, ok := s.values[current.ID][name]
		s.mu.RUnlock()
		if ok {
			return s.open(item.sealed)
		}
		s.mu.RLock()
		durable := s.durable
		s.mu.RUnlock()
		if durable != nil {
			if sealed, resolveErr := durable.ResolveEncrypted(ctx, current.ID, name); resolveErr == nil {
				return s.open(sealed)
			}
		}
	}
	return "", ErrNotFound
}

func (s *Service) Delete(ctx context.Context, userID, channelID, name string) error {
	channel, err := s.authorize(ctx, userID, channelID, true)
	if err != nil {
		return err
	}
	s.mu.Lock()
	_, found := s.values[channel.ID][strings.TrimSpace(name)]
	delete(s.values[channel.ID], strings.TrimSpace(name))
	durable := s.durable
	s.mu.Unlock()
	if durable != nil {
		if err := durable.DeleteEncrypted(ctx, channel.ID, strings.TrimSpace(name)); err != nil && !found {
			return ErrNotFound
		}
	} else if !found {
		return ErrNotFound
	}
	return nil
}

func (s *Service) loadDurableMetadata(ctx context.Context, channelID string, inherited bool, result map[string]Metadata) {
	s.mu.RLock()
	durable := s.durable
	s.mu.RUnlock()
	if durable == nil {
		return
	}
	items, err := durable.ListEncrypted(ctx, channelID)
	if err != nil {
		return
	}
	for _, item := range items {
		if _, exists := result[item.Name]; !exists {
			result[item.Name] = Metadata{Name: item.Name, UpdatedAt: item.UpdatedAt, SourceChannelID: channelID, Inherited: inherited}
		}
	}
}

func (s *Service) authorize(ctx context.Context, userID, channelID string, write bool) (collaboration.Channel, error) {
	channel, err := s.channels.GetChannel(ctx, userID, channelID)
	if err != nil {
		return collaboration.Channel{}, err
	}
	if write {
		err = s.auth.CanWrite(ctx, channel.WorkspaceID, userID)
	} else {
		err = s.auth.CanRead(ctx, channel.WorkspaceID, userID)
	}
	return channel, err
}
func (s *Service) parent(ctx context.Context, userID string, channel collaboration.Channel) *collaboration.Channel {
	if channel.ParentID == "" {
		return nil
	}
	parent, err := s.channels.GetChannel(ctx, userID, channel.ParentID)
	if err != nil {
		return nil
	}
	return &parent
}
func (s *Service) seal(value string) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(value), nil), nil
}
func (s *Service) open(sealed []byte) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	n := gcm.NonceSize()
	if len(sealed) < n {
		return "", ErrInvalid
	}
	value, err := gcm.Open(nil, sealed[:n], sealed[n:], nil)
	return string(value), err
}

var _ Store = (*Service)(nil)
