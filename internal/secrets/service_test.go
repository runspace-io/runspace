package secrets

import (
	"context"
	"testing"
	"time"

	"github.com/runspace/runspace/internal/collaboration"
)

type durable struct{ values map[string][]byte }

func (d *durable) SetEncrypted(_ context.Context, channel, name string, sealed []byte, _ time.Time) error {
	if d.values == nil {
		d.values = map[string][]byte{}
	}
	d.values[channel+":"+name] = sealed
	return nil
}
func (d *durable) ListEncrypted(_ context.Context, channel string) ([]EncryptedMetadata, error) {
	var out []EncryptedMetadata
	for key := range d.values {
		if len(key) > len(channel) && key[:len(channel)] == channel {
			out = append(out, EncryptedMetadata{Name: key[len(channel)+1:]})
		}
	}
	return out, nil
}
func (d *durable) ResolveEncrypted(_ context.Context, channel, name string) ([]byte, error) {
	return d.values[channel+":"+name], nil
}
func (d *durable) DeleteEncrypted(_ context.Context, channel, name string) error {
	delete(d.values, channel+":"+name)
	return nil
}

type auth struct{}

func (auth) CanRead(context.Context, string, string) error  { return nil }
func (auth) CanWrite(context.Context, string, string) error { return nil }

type channels struct {
	items map[string]collaboration.Channel
}

func (c channels) GetChannel(_ context.Context, _ string, id string) (collaboration.Channel, error) {
	item, ok := c.items[id]
	if !ok {
		return collaboration.Channel{}, collaboration.ErrNotFound
	}
	return item, nil
}

func TestSecretsAreRedactedAndInherited(t *testing.T) {
	clock := func() time.Time { return time.Unix(10, 0) }
	resolver := channels{items: map[string]collaboration.Channel{
		"parent": {ID: "parent", WorkspaceID: "ws"}, "child": {ID: "child", WorkspaceID: "ws", ParentID: "parent"},
	}}
	service, err := New(resolver, auth{}, make([]byte, 32), clock)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Set(context.Background(), "u", "parent", "API_KEY", "super-secret"); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(context.Background(), "u", "child")
	if err != nil || len(items) != 1 || items[0].Name != "API_KEY" {
		t.Fatalf("unexpected metadata: %+v %v", items, err)
	}
	if items[0].Name == "super-secret" {
		t.Fatal("secret value leaked")
	}
	value, err := service.Resolve(context.Background(), "u", "child", "API_KEY")
	if err != nil || value != "super-secret" {
		t.Fatalf("resolve: %q %v", value, err)
	}
}

func TestSecretsWriteThroughDurableStore(t *testing.T) {
	resolver := channels{items: map[string]collaboration.Channel{"channel": {ID: "channel", WorkspaceID: "ws"}}}
	service, err := New(resolver, auth{}, make([]byte, 32), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	store := &durable{}
	service.SetPersistence(store)
	if err := service.Set(context.Background(), "u", "channel", "TOKEN", "value"); err != nil {
		t.Fatal(err)
	}
	if len(store.values) != 1 {
		t.Fatalf("durable values=%d", len(store.values))
	}
	items, err := service.List(context.Background(), "u", "channel")
	if err != nil || len(items) != 1 || items[0].Name != "TOKEN" {
		t.Fatalf("items=%v err=%v", items, err)
	}
}
