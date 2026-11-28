package port

// NotificationTargetScope defines the recipient boundary of a notification.
type NotificationTargetScope string

const (
	ScopeDirect    NotificationTargetScope = "DIRECT"
	ScopeGroup     NotificationTargetScope = "GROUP"
	ScopeBroadcast NotificationTargetScope = "BROADCAST"
)

// NotificationTarget represents the recipient target of a notification.
type NotificationTarget struct {
	Scope   NotificationTargetScope
	UserID  string
	GroupID string
	UserIDs []string
}
