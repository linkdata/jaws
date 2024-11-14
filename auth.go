package jaws

type Auth interface {
	Data() map[string]any // returns authenticated user data, or nil
	Email() string        // returns authenticated user email, or an empty string
	IsAdmin() bool        // return true if admins are defined and current user is one, or if no admins are defined
}

type MakeAuthFn func(*Request) Auth

type defaultAuth struct{}

func (defaultAuth) Data() map[string]any { return nil }
func (defaultAuth) Email() string        { return "" }
func (defaultAuth) IsAdmin() bool        { return true }
