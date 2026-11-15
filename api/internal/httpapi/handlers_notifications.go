package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notification"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/notifications"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/sla"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/ticket"
)

func (s *server) ListNotifications(ctx context.Context, req gen.ListNotificationsRequestObject) (gen.ListNotificationsResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	filter := notification.FilterAll
	if req.Params.Filter != nil {
		filter = notification.Filter(*req.Params.Filter)
	}
	var limit int
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := s.notifications.List(ctx, p, filter, req.Params.Before, limit)
	if err != nil {
		return nil, err
	}
	out := gen.ListNotifications200JSONResponse{
		Items: make([]gen.Notification, len(page.Items)), UnreadCount: page.UnreadCount, NextBefore: page.NextBefore,
	}
	for i, it := range page.Items {
		out.Items[i] = apiNotification(it)
	}
	return out, nil
}

func (s *server) GetUnreadNotificationCount(ctx context.Context, _ gen.GetUnreadNotificationCountRequestObject) (gen.GetUnreadNotificationCountResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	n, err := s.notifications.UnreadCount(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.GetUnreadNotificationCount200JSONResponse{Count: n}, nil
}

func (s *server) MarkAllNotificationsRead(ctx context.Context, _ gen.MarkAllNotificationsReadRequestObject) (gen.MarkAllNotificationsReadResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.notifications.ReadAll(ctx, p); err != nil {
		return nil, err
	}
	return gen.MarkAllNotificationsRead204Response{}, nil
}

func (s *server) SetNotificationRead(ctx context.Context, req gen.SetNotificationReadRequestObject) (gen.SetNotificationReadResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.notifications.SetRead(ctx, p, req.NotificationId, req.Body.Read); err != nil {
		return nil, err
	}
	return gen.SetNotificationRead204Response{}, nil
}

func (s *server) GetNotificationSettings(ctx context.Context, _ gen.GetNotificationSettingsRequestObject) (gen.GetNotificationSettingsResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.notifications.Settings(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.GetNotificationSettings200JSONResponse(apiNotificationSettings(st)), nil
}

func (s *server) SetNotificationPreference(ctx context.Context, req gen.SetNotificationPreferenceRequestObject) (gen.SetNotificationPreferenceResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.notifications.SetEventPreference(ctx, p, notification.Event(req.Event),
		notification.Pref{Web: req.Body.Web, Email: req.Body.Email})
	if err != nil {
		return nil, err
	}
	return gen.SetNotificationPreference200JSONResponse(apiNotificationSettings(st)), nil
}

func (s *server) SetEmailFrequency(ctx context.Context, req gen.SetEmailFrequencyRequestObject) (gen.SetEmailFrequencyResponseObject, error) {
	p, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.notifications.SetEmailFrequency(ctx, p, notification.Frequency(req.Body.EmailFrequency))
	if err != nil {
		return nil, err
	}
	return gen.SetEmailFrequency200JSONResponse(apiNotificationSettings(st)), nil
}

func apiNotification(it notifications.Item) gen.Notification {
	d := it.Data
	out := gen.Notification{
		Id: it.ID, Event: gen.NotificationEvent(it.Event), Ticket: gen.NotificationTicket{Key: it.TicketKey, Subject: it.Subject},
		ActorName: it.ActorName, Quote: it.Quote, CreatedAt: it.CreatedAt, ReadAt: it.ReadAt,
		Kind: enumPtr[gen.NotificationKind](d.Kind), StatusFrom: statusPtr(d.StatusFrom), StatusTo: statusPtr(d.StatusTo),
		PriorityFrom: priorityPtr(d.PriorityFrom), PriorityTo: priorityPtr(d.PriorityTo),
		Reason: strPtr(d.Reason), AssigneeName: strPtr(d.Assignee), SlaClock: enumPtr[gen.SlaClockName](d.SLAClock),
		SlaDueAt: d.SLADueAt, SlaMinutesLeft: d.SLALeftMinutes,
		Channels: gen.NotificationChannels{Web: it.Channels.Web, Email: it.Channels.Email},
		Also:     make([]gen.NotificationAlso, len(d.Also)),
	}
	for i, a := range d.Also {
		out.Also[i] = gen.NotificationAlso{
			Change: gen.NotificationChange(a.Change), StatusFrom: statusPtr(a.StatusFrom), StatusTo: statusPtr(a.StatusTo),
			PriorityFrom: priorityPtr(a.PriorityFrom), PriorityTo: priorityPtr(a.PriorityTo), AssigneeName: strPtr(a.Assignee),
		}
	}
	return out
}

func apiNotificationSettings(st notifications.Settings) gen.NotificationSettings {
	out := gen.NotificationSettings{
		Audience: gen.Audience(st.Audience), EmailFrequency: gen.EmailFrequency(st.Frequency),
		Events: make([]gen.NotificationEventSetting, len(st.Events)),
	}
	for i, e := range st.Events {
		out.Events[i] = gen.NotificationEventSetting{
			Event: gen.NotificationEvent(e.Event), Web: e.Pref.Web, Email: e.Pref.Email, Locked: e.Locked,
		}
	}
	return out
}

// enumPtr is an optional enum value: nil when empty.
func enumPtr[T ~string, S ~string](v S) *T {
	if v == "" {
		return nil
	}
	t := T(v)
	return &t
}

func strPtr(s string) *string { return enumPtr[string](s) }

func statusPtr(s ticket.Status) *gen.TicketStatus { return enumPtr[gen.TicketStatus](s) }

func priorityPtr(p sla.Priority) *gen.Priority { return enumPtr[gen.Priority](p) }
