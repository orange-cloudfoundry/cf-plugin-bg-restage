package internal

import (
	"net/http"
)

const (
	GetClientUser         = "GetClientUser"
	GetSSHPasscodeRequest = "GetSSHPasscode"
	PostOAuthTokenRequest = "PostOAuthToken"
	PostUserRequest       = "PostUser"
	ListUsersRequest      = "ListUsers"
	DeleteUserRequest     = "DeleteUser"
	UpdatePasswordRequest = "UpdatePassword"
	DeleteTokenRequest    = "DeleteToken"
)

// APIRoutes is a list of routes used by the router to construct request URLs.
var APIRoutes = []Route{
	{Path: "/Users", Method: http.MethodPost, Name: PostUserRequest, Resource: UAAResource},
	{Path: "/Users", Method: http.MethodGet, Name: ListUsersRequest, Resource: UAAResource},
	{Path: "/Users/:user_guid", Method: http.MethodDelete, Name: DeleteUserRequest, Resource: UAAResource},
	{Path: "/Users/:user_guid/password", Method: http.MethodPut, Name: UpdatePasswordRequest, Resource: UAAResource},
	{Path: "/oauth/authorize", Method: http.MethodGet, Name: GetSSHPasscodeRequest, Resource: UAAResource},
	{Path: "/oauth/clients/:client_id", Method: http.MethodGet, Name: GetClientUser, Resource: UAAResource},
	{Path: "/oauth/token", Method: http.MethodPost, Name: PostOAuthTokenRequest, Resource: AuthorizationResource},
	{Path: "/oauth/token/revoke/:token_id", Method: http.MethodDelete, Name: DeleteTokenRequest, Resource: AuthorizationResource},
}
