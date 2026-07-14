package events_test

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/require"

	latentframev1 "github.com/andreas-lindfalk/latentframe/gen/go/latentframe/v1"
	"github.com/andreas-lindfalk/latentframe/pkg/events"
)

// A proto event round-trips through the CloudEvents envelope and the in-process bus:
// publish → the type's handler runs → payload decodes back to the same proto.
func TestSyncBusRoundTrip(t *testing.T) {
	bus := events.NewSyncBus()

	var got *latentframev1.RoomRestaged
	bus.Subscribe(events.TypeRoomRestaged, func(_ context.Context, e cloudevents.Event) error {
		var p latentframev1.RoomRestaged
		if err := events.UnmarshalData(e, &p); err != nil {
			return err
		}
		got = &p
		return nil
	})

	evt, err := events.NewEvent(events.TypeRoomRestaged, "render", "prop_1/room_1", &latentframev1.RoomRestaged{
		PropertyId:    "prop_1",
		RoomId:        "room_1",
		AfterStillUri: "gs://bucket/after.png",
	})
	require.NoError(t, err)
	require.NoError(t, bus.Publish(context.Background(), evt))

	require.NotNil(t, got, "handler should have been invoked")
	require.Equal(t, "gs://bucket/after.png", got.AfterStillUri)
	require.Equal(t, "room_1", got.RoomId)
	require.Equal(t, events.TypeRoomRestaged, evt.Type())
	require.Equal(t, events.ContentTypeProto, evt.DataContentType())
	require.Equal(t, "prop_1/room_1", evt.Subject())
}

// A handler that isn't subscribed to the event's type must not fire.
func TestSyncBusTypeRouting(t *testing.T) {
	bus := events.NewSyncBus()
	fired := false
	bus.Subscribe(events.TypeRoomVerified, func(_ context.Context, _ cloudevents.Event) error {
		fired = true
		return nil
	})

	evt, err := events.NewEvent(events.TypeRoomRestaged, "render", "prop_1/room_1", &latentframev1.RoomRestaged{})
	require.NoError(t, err)
	require.NoError(t, bus.Publish(context.Background(), evt))
	require.False(t, fired, "verified handler must not fire for a restaged event")
}
