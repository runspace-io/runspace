package workspace

import "errors"

var (
	ErrNotFound      = errors.New("workspace resource not found")
	ErrUnauthorized  = errors.New("workspace authorization required")
	ErrForbidden     = errors.New("workspace action forbidden")
	ErrInvalidInput  = errors.New("invalid workspace input")
	ErrAlreadyExists = errors.New("workspace resource already exists")
)

type Action string

const (
	ActionRead            Action = "read"
	ActionCreateWorkspace Action = "create_workspace"
	ActionManageMembers   Action = "manage_members"
	ActionConnectResource Action = "connect_resource"
	ActionConnectRepo     Action = ActionConnectResource
)

func (r Role) valid() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleMember || r == RoleViewer
}

func Allows(role Role, action Action) bool {
	if !role.valid() {
		return false
	}
	switch action {
	case ActionRead:
		return true
	case ActionCreateWorkspace:
		return role == RoleOwner
	case ActionManageMembers, ActionConnectResource:
		return role == RoleOwner || role == RoleAdmin
	default:
		return false
	}
}
