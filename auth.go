package jaws

type Auth interface {
	Data() map[string]any // returns authenticated user data, or nil
	Email() string        // returns authenticated user email, or an empty string
	IsAdmin() bool        // return true if admins are defined and current user is one
}

type MakeAuthFn func(*Request) Auth
