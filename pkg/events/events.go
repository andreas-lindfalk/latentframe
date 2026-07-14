// Package events is Latent Frame's messaging seam. Domain events are proto payloads
// (gen/go/latentframe/v1) wrapped in a CloudEvents envelope, published through a
// Publisher interface. The sync implementation dispatches in-process — the monolith's
// event backbone — and a GCP Pub/Sub implementation will drop in behind the same
// interface when a stage is split into its own service. We adopt the CloudEvents
// standard rather than defining our own envelope.
package events

import (
	"context"
	"fmt"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

// ContentTypeProto is the datacontenttype for proto-encoded event payloads.
const ContentTypeProto = "application/protobuf"

// CloudEvent `type` values (reverse-DNS, versioned) — see proto/latentframe/v1/events.proto.
const (
	TypePropertyIngested   = "ai.latentframe.property.ingested.v1"
	TypePropertyUnderstood = "ai.latentframe.property.understood.v1"
	TypeRoomRestaged       = "ai.latentframe.room.restaged.v1"
	TypeRoomVerified       = "ai.latentframe.room.verified.v1"
	TypeRoomAnimated       = "ai.latentframe.room.animated.v1"
	TypePropertyAssembled  = "ai.latentframe.property.assembled.v1"
)

// Publisher emits CloudEvents. Implemented in-process by SyncBus today; by a GCP
// Pub/Sub publisher later, unchanged at every call site.
type Publisher interface {
	Publish(ctx context.Context, e cloudevents.Event) error
}

// NewEvent wraps a proto payload in a CloudEvent. eventType is one of the Type*
// constants, source identifies the emitting stage (e.g. "render"), and subject is
// the property or "property/room" id the event concerns.
func NewEvent(eventType, source, subject string, payload proto.Message) (cloudevents.Event, error) {
	e := cloudevents.NewEvent()
	e.SetID(uuid.NewString())
	e.SetSource(source)
	e.SetType(eventType)
	if subject != "" {
		e.SetSubject(subject)
	}
	e.SetTime(time.Now().UTC())

	b, err := proto.Marshal(payload)
	if err != nil {
		return e, fmt.Errorf("marshal %s payload: %w", eventType, err)
	}
	if err := e.SetData(ContentTypeProto, b); err != nil {
		return e, fmt.Errorf("set event data: %w", err)
	}
	return e, nil
}

// UnmarshalData decodes a proto-encoded CloudEvent payload into m. It rejects events
// whose datacontenttype is not application/protobuf, and empty payloads — both of which
// proto.Unmarshal would otherwise silently accept as an empty message, hiding a
// mis-routed or malformed event.
func UnmarshalData(e cloudevents.Event, m proto.Message) error {
	if ct := e.DataContentType(); ct != ContentTypeProto {
		return fmt.Errorf("event %s: unexpected datacontenttype %q, want %s", e.Type(), ct, ContentTypeProto)
	}
	data := e.Data()
	if len(data) == 0 {
		return fmt.Errorf("event %s: empty data payload", e.Type())
	}
	return proto.Unmarshal(data, m)
}
