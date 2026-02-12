package core

type Auth interface {
	Data() map[string]any // returns authenticated user data, or nil
	Email() string        // returns authenticated user email, or an empty string
	IsAdmin() bool        // return true if admins are defined and current user is one, or if no admins are defined
}

type MakeAuthFn func(*Request) Auth

type DefaultAuth struct{}

func (DefaultAuth) Data() map[string]any { return nil }
func (DefaultAuth) Email() string        { return "" }
func (DefaultAuth) IsAdmin() bool        { return true }
