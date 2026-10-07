package vo

type Visibility string

const (
	VisibilityPublic        Visibility = "PUBLIC"
	VisibilityFollowersOnly Visibility = "FOLLOWERS_ONLY"
	VisibilityPrivate       Visibility = "PRIVATE"
)

func NewVisibility(s string) Visibility {
	switch s {
	case string(VisibilityFollowersOnly):
		return VisibilityFollowersOnly
	case string(VisibilityPrivate):
		return VisibilityPrivate
	default:
		return VisibilityPublic
	}
}

func (v Visibility) String() string {
	return string(v)
}
