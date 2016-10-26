// Package oauth2 implements an authenticator component that provides OAuth2
// compatible authentication.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gonfire/fire"
	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/bearer"
	"github.com/gonfire/oauth2/hmacsha"
	"gopkg.in/mgo.v2/bson"
)

const AccessTokenContextKey = "fire.oauth2.access_token"

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant.
type Authenticator struct {
	Policy  *Policy
	Storage *Storage
}

// New constructs a new Authenticator.
func New(store *fire.Store, policy *Policy) *Authenticator {
	// check secret
	if len(policy.Secret) < 16 {
		panic("Secret must be longer than 16 characters")
	}

	// initialize models
	fire.Init(policy.AccessToken)
	fire.Init(policy.RefreshToken)
	fire.Init(policy.Client)
	fire.Init(policy.ResourceOwner)

	// create storage
	storage := NewStorage(policy, store)

	return &Authenticator{
		Policy:  policy,
		Storage: storage,
	}
}

// NewKeyAndSignature returns a new key with a matching signature that can be
// used to issue custom access tokens.
func (a *Authenticator) NewKeyAndSignature() (string, string, error) {
	token, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return "", "", err
	}

	return token.String(), token.SignatureString(), nil
}

func (a *Authenticator) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

		// try to call the controllers general handler
		if len(s) > 0 {
			if s[0] == "token" {
				a.TokenEndpoint(w, r)
				return
			} else if s[0] == "authorize" {
				a.AuthorizationEndpoint(w, r)
				return
			}
		}

		// write not found error
		w.WriteHeader(http.StatusNotFound)
	})
}

// Authorize can be used to authorize a request by requiring an access token with
// the provided scopes to be granted. The method returns a middleware that can be
// called before any other routes.
func (a *Authenticator) Authorizer(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// parse scope
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, res := bearer.ParseToken(r)
			if res != nil {
				bearer.WriteError(w, res)
				return
			}

			// parse token
			token, err := hmacsha.Parse(a.Policy.Secret, tk)
			if err != nil {
				bearer.WriteError(w, bearer.InvalidToken("Malformed token"))
				return
			}

			// get token
			accessToken, err := a.Storage.GetAccessToken(token.SignatureString())
			if err != nil {
				bearer.WriteError(w, err)
				return
			} else if accessToken == nil {
				bearer.WriteError(w, bearer.InvalidToken("Unkown token"))
				return
			}

			// get additional data
			data := accessToken.GetTokenData()

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				bearer.WriteError(w, bearer.InvalidToken("Expired token"))
				return
			}

			// validate scope
			if !data.Scope.Includes(s) {
				bearer.WriteError(w, bearer.InsufficientScope(s.String()))
				return
			}

			// create new context with access token
			ctx := context.WithValue(r.Context(), AccessTokenContextKey, accessToken)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) AuthorizationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(r)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	}

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Unknown response type"))
		return
	}

	// get client
	client, err := a.Storage.GetClient(req.ClientID)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if client == nil {
		oauth2.WriteError(w, oauth2.InvalidClient(req.State, "Unknown client"))
		return
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Invalid redirect URI"))
		return
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		if a.Policy.ImplicitGrant {
			a.handleImplicitGrant(w, r, req, client)
			return
		}
	}

	// response type is unsupported
	oauth2.WriteError(w, oauth2.UnsupportedResponseType(req.State, oauth2.NoDescription))
}

func (a *Authenticator) handleImplicitGrant(w http.ResponseWriter, r *http.Request, req *oauth2.AuthorizationRequest, client Client) {
	// check request method
	if r.Method == "GET" {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.InvalidRequest(req.State, "Unallowed request method"))
		return
	}

	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner, err := a.Storage.GetResourceOwner(username)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
		return
	} else if resourceOwner == nil {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.InvalidScope(req.State, oauth2.NoDescription))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(false, scope, req.State, client.ID(), &rid)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.RedirectTokenResponse(w, req.RedirectURI, res)
}

func (a *Authenticator) TokenEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse token request
	req, err := oauth2.ParseTokenRequest(r)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	}

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		oauth2.WriteError(w, oauth2.InvalidRequest(oauth2.NoState, "Unknown grant type"))
		return
	}

	// get client
	client, err := a.Storage.GetClient(req.ClientID)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if client == nil {
		oauth2.WriteError(w, oauth2.InvalidClient(oauth2.NoState, "Unknown client"))
		return
	}

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		if a.Policy.PasswordGrant {
			a.handleResourceOwnerPasswordCredentialsGrant(w, req, client)
			return
		}
	case oauth2.ClientCredentialsGrantType:
		if a.Policy.ClientCredentialsGrant {
			a.handleClientCredentialsGrant(w, req, client)
			return
		}
	case oauth2.RefreshTokenGrantType:
		a.handleRefreshTokenGrant(w, req, client)
		return
	}

	// grant type is unsupported
	oauth2.WriteError(w, oauth2.UnsupportedGrantType(oauth2.NoState, oauth2.NoDescription))
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// get resource owner
	resourceOwner, err := a.Storage.GetResourceOwner(req.Username)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if resourceOwner == nil {
		oauth2.WriteError(w, oauth2.AccessDenied(oauth2.NoState, oauth2.NoDescription))
		return
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		oauth2.WriteError(w, oauth2.AccessDenied(oauth2.NoState, oauth2.NoDescription))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, oauth2.NoDescription))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(true, scope, oauth2.NoState, client.ID(), &rid)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) handleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		oauth2.WriteError(w, oauth2.InvalidClient(oauth2.NoState, "Unknown client"))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if !granted {
		oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, oauth2.NoDescription))
		return
	}

	// issue access token
	res, err := a.issueTokens(true, scope, oauth2.NoState, client.ID(), nil)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) handleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse refresh token
	refreshToken, err := hmacsha.Parse(a.Policy.Secret, req.RefreshToken)
	if err != nil {
		oauth2.WriteError(w, oauth2.InvalidRequest(oauth2.NoState, err.Error()))
		return
	}

	// get stored refresh token by signature
	rt, err := a.Storage.GetRefreshToken(refreshToken.SignatureString())
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if rt == nil {
		oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Unknown refresh token"))
		return
	}

	// get data
	data := rt.GetTokenData()

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Expired refresh token"))
		return
	}

	// validate ownership
	if data.ClientID != client.ID() {
		oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Invalid refresh token ownership"))
		return
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, "Scope exceeds the originally granted scope"))
		return
	}

	// issue tokens
	res, err := a.issueTokens(true, req.Scope, oauth2.NoState, client.ID(), data.ResourceOwnerID)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// delete refresh token
	err = a.Storage.DeleteRefreshToken(refreshToken.SignatureString())
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) issueTokens(issueRefreshToken bool, scope oauth2.Scope, state string, clientID bson.ObjectId, resourceOwnerID *bson.ObjectId) (*oauth2.TokenResponse, error) {
	// generate new access token
	accessToken, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return nil, err
	}

	// generate new refresh token
	refreshToken, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return nil, err
	}

	// prepare response
	res := bearer.NewTokenResponse(accessToken.String(), int(a.Policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// set state
	res.State = state

	// set refresh token if requested
	if issueRefreshToken {
		res.RefreshToken = refreshToken.String()
	}

	// create access token data
	accessTokenData := &TokenData{
		Signature:       accessToken.SignatureString(),
		Scope:           scope,
		ExpiresAt:       time.Now().Add(a.Policy.AccessTokenLifespan),
		ClientID:        clientID,
		ResourceOwnerID: resourceOwnerID,
	}

	// save access token
	_, err = a.Storage.SaveAccessToken(accessTokenData)
	if err != nil {
		return nil, err
	}

	if issueRefreshToken {
		// create refresh token data
		refreshTokenData := &TokenData{
			Signature:       refreshToken.SignatureString(),
			Scope:           scope,
			ExpiresAt:       time.Now().Add(a.Policy.RefreshTokenLifespan),
			ClientID:        clientID,
			ResourceOwnerID: resourceOwnerID,
		}

		// save refresh token
		_, err := a.Storage.SaveRefreshToken(refreshTokenData)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
