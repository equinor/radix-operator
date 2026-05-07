package auth

type anonPrincipal struct{}

func (p *anonPrincipal) Token() string         { return "" }
func (p *anonPrincipal) Id() string            { return "anonymous" }
func (p *anonPrincipal) Name() string          { return "anonymous" }
func (p *anonPrincipal) IsAuthenticated() bool { return false }
