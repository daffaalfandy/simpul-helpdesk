package httpapi

import (
	"context"

	"github.com/daffaalfandy/simpul-helpdesk/api/internal/auth"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

// ListMyTokens lists the user's personal access tokens in the active organization, with the
// MCP endpoint they connect to.
func (s *server) ListMyTokens(ctx context.Context, _ gen.ListMyTokensRequestObject) (gen.ListMyTokensResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	list, err := s.auth.ListTokens(ctx, p)
	if err != nil {
		return nil, err
	}
	out := gen.ListMyTokens200JSONResponse{McpUrl: s.mcpURL(), Items: make([]gen.PersonalToken, len(list))}
	for i, t := range list {
		out.Items[i] = apiPersonalToken(t)
	}
	return out, nil
}

// CreateMyToken makes a token; its secret is in this response only.
func (s *server) CreateMyToken(ctx context.Context, req gen.CreateMyTokenRequestObject) (gen.CreateMyTokenResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	t, secret, err := s.auth.CreateToken(ctx, p, auth.TokenInput{
		Name: req.Body.Name, Access: auth.TokenAccess(req.Body.Access), LifetimeDays: int(req.Body.ExpiresInDays),
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateMyToken201JSONResponse{Token: apiPersonalToken(t), Secret: secret}, nil
}

// RevokeMyToken deletes one of the user's tokens.
func (s *server) RevokeMyToken(ctx context.Context, req gen.RevokeMyTokenRequestObject) (gen.RevokeMyTokenResponseObject, error) {
	p, err := requireOrg(ctx, auth.CapAgentApp)
	if err != nil {
		return nil, err
	}
	if err := s.auth.RevokeToken(ctx, p, req.TokenId); err != nil {
		return nil, err
	}
	return gen.RevokeMyToken204Response{}, nil
}

// mcpURL is the MCP endpoint's absolute address under APP_URL.
func (s *server) mcpURL() string {
	return s.cfg.AppURL.JoinPath(apiRoot, mcpPath).String()
}

func apiPersonalToken(t auth.PersonalToken) gen.PersonalToken {
	return gen.PersonalToken{
		Id: t.ID, Name: t.Name, LastFour: t.LastFour, Access: gen.TokenAccess(t.Access),
		CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt,
	}
}
