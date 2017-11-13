// Package auth implements an authentication manager that provides OAuth2
// compatible authentication.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/bearer"
	"github.com/256dpi/oauth2/revocation"
	"github.com/256dpi/stack"
	"github.com/dgrijalva/jwt-go"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type ctxKey int

// AccessTokenContextKey is the key used to save the access token in a context.
const AccessTokenContextKey ctxKey = iota

// A Manager provides OAuth2 based authentication. The implementation currently
// supports the Resource Owner Credentials Grant, Client Credentials Grant and
// Implicit Grant.
type Manager struct {
	store  *fire.Store
	policy *Policy

	Reporter func(error)
}

// New constructs a new Manager from a store and policy.
func New(store *fire.Store, policy *Policy) *Manager {
	// check secret
	if len(policy.Secret) < 16 {
		panic("fire/auth: secret must be longer than 16 characters")
	}

	// initialize models
	fire.Init(policy.AccessToken)
	fire.Init(policy.RefreshToken)

	// initialize clients
	for _, model := range policy.Clients {
		fire.Init(model)
	}

	// initialize resource owners
	for _, model := range policy.ResourceOwners {
		fire.Init(model)
	}

	return &Manager{
		store:  store,
		policy: policy,
	}
}

// Endpoint returns a handler for the common token and authorize endpoint.
func (m *Manager) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write potential oauth2 errors
			if oauth2Error, ok := err.(*oauth2.Error); ok {
				oauth2.WriteError(w, oauth2Error)
				return
			}

			// otherwise report critical errors
			if m.Reporter != nil {
				m.Reporter(err)
			}

			// ignore errors caused by writing critical errors
			oauth2.WriteError(w, oauth2.ServerError(""))
		})

		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

		// try to call the controllers general handler
		if len(s) > 0 {
			if s[0] == "authorize" {
				m.authorizationEndpoint(w, r)
				return
			} else if s[0] == "token" {
				m.tokenEndpoint(w, r)
				return
			} else if s[0] == "revoke" {
				m.revocationEndpoint(w, r)
				return
			}
		}

		// write not found error
		w.WriteHeader(http.StatusNotFound)
	})
}

// Authorizer returns a middleware that can be used to authorize a request by
// requiring an access token with the provided scope to be granted.
func (m *Manager) Authorizer(scope string, force bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// immediately pass on request if force is not set and there is
			// no authentication information provided
			if !force && r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}

			// continue any previous aborts
			defer stack.Resume(func(err error) {
				// directly write potential bearer errors
				if bearerError, ok := err.(*bearer.Error); ok {
					bearer.WriteError(w, bearerError)
					return
				}

				// otherwise report critical errors
				if m.Reporter != nil {
					m.Reporter(err)
				}

				// ignore errors caused by writing critical errors
				bearer.WriteError(w, bearer.ServerError())
			})

			// parse scope
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, err := bearer.ParseToken(r)
			stack.AbortIf(err)

			// parse token and check id
			var claims accessTokenClaims
			_, err = jwt.ParseWithClaims(tk, &claims, func(token *jwt.Token) (interface{}, error) {
				return m.policy.Secret, nil
			})
			if err != nil || !bson.IsObjectIdHex(claims.Id) {
				stack.Abort(bearer.InvalidToken("Malformed token"))
			}

			// get token
			accessToken := m.getAccessToken(bson.ObjectIdHex(claims.Id))
			if accessToken == nil {
				stack.Abort(bearer.InvalidToken("Unknown token"))
			}

			// get additional data
			data := accessToken.GetTokenData()

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				stack.Abort(bearer.InvalidToken("Expired token"))
			}

			// validate scope
			if !oauth2.Scope(data.Scope).Includes(s) {
				stack.Abort(bearer.InsufficientScope(s.String()))
			}

			// create new context with access token
			ctx := context.WithValue(r.Context(), AccessTokenContextKey, accessToken)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (m *Manager) authorizationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(r)
	stack.AbortIf(err)

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		stack.Abort(oauth2.InvalidRequest("Unknown response type"))
	}

	// get client
	client := m.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		stack.Abort(oauth2.InvalidRequest("Invalid redirect URI"))
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		if m.policy.ImplicitGrant {
			m.handleImplicitGrant(w, r, req, client)
			return
		}
	}

	// response type is unsupported
	stack.Abort(oauth2.UnsupportedResponseType(""))
}

func (m *Manager) handleImplicitGrant(w http.ResponseWriter, r *http.Request, req *oauth2.AuthorizationRequest, client Client) {
	// check request method
	if r.Method == "GET" {
		stack.Abort(oauth2.InvalidRequest("Unallowed request method").SetRedirect(req.RedirectURI, req.State, true))
	}

	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner := m.findFirstResourceOwner(username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate & grant scope
	scope, err := m.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := m.issueTokens(false, scope, client, resourceOwner)

	// redirect response
	res.SetRedirect(req.RedirectURI, req.State, true)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (m *Manager) tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse token request
	req, err := oauth2.ParseTokenRequest(r)
	stack.AbortIf(err)

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		stack.Abort(oauth2.InvalidRequest("Unknown grant type"))
	}

	// get client
	client := m.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		if m.policy.PasswordGrant {
			m.handleResourceOwnerPasswordCredentialsGrant(w, req, client)
			return
		}
	case oauth2.ClientCredentialsGrantType:
		if m.policy.ClientCredentialsGrant {
			m.handleClientCredentialsGrant(w, req, client)
			return
		}
	case oauth2.RefreshTokenGrantType:
		m.handleRefreshTokenGrant(w, req, client)
		return
	}

	// grant type is unsupported
	stack.Abort(oauth2.UnsupportedGrantType(""))
}

func (m *Manager) handleResourceOwnerPasswordCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// get resource owner
	resourceOwner := m.findFirstResourceOwner(req.Username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// validate & grant scope
	scope, err := m.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := m.issueTokens(true, scope, client, resourceOwner)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (m *Manager) handleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// validate & grant scope
	scope, err := m.policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := m.issueTokens(true, scope, client, nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (m *Manager) handleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse token
	var claims refreshTokenClaims
	_, err := jwt.ParseWithClaims(req.RefreshToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return m.policy.Secret, nil
	})
	if err != nil || !bson.IsObjectIdHex(claims.Id) {
		stack.Abort(oauth2.InvalidRequest("Malformed token"))
	}

	// get stored refresh token by signature
	rt := m.getRefreshToken(bson.ObjectIdHex(claims.Id))
	if rt == nil {
		stack.Abort(oauth2.InvalidGrant("Unknown refresh token"))
	}

	// get data
	data := rt.GetTokenData()

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		stack.Abort(oauth2.InvalidGrant("Expired refresh token"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		stack.Abort(oauth2.InvalidGrant("Invalid refresh token ownership"))
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !oauth2.Scope(data.Scope).Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("Scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = m.getFirstResourceOwner(*data.ResourceOwnerID)
	}

	// issue tokens
	res := m.issueTokens(true, req.Scope, client, ro)

	// delete refresh token
	m.deleteRefreshToken(rt.ID(), client.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (m *Manager) revocationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := revocation.ParseRequest(r)
	stack.AbortIf(err)

	// get client
	client := m.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// parse token
	var claims jwt.StandardClaims
	_, err = jwt.ParseWithClaims(req.Token, &claims, func(token *jwt.Token) (interface{}, error) {
		return m.policy.Secret, nil
	})
	if err != nil || !bson.IsObjectIdHex(claims.Id) {
		return
	}

	// delete access token
	m.deleteAccessToken(bson.ObjectIdHex(claims.Id), client.ID())

	// delete refresh token
	m.deleteRefreshToken(bson.ObjectIdHex(claims.Id), client.ID())

	// write header
	w.WriteHeader(http.StatusOK)
}

func (m *Manager) issueTokens(refreshable bool, scope oauth2.Scope, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// prepare expiration
	atExpiry := time.Now().Add(m.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(m.policy.RefreshTokenLifespan)

	// create access token data
	accessTokenData := &TokenData{
		Scope:     scope,
		ExpiresAt: atExpiry,
		ClientID:  client.ID(),
	}

	// set resource owner id if available
	if resourceOwner != nil {
		roID := resourceOwner.ID()
		accessTokenData.ResourceOwnerID = &roID
	}

	// save access token
	at := m.saveAccessToken(accessTokenData)

	// generate new access token
	atSignature, err := generateAccessToken(at.ID(), m.policy.Secret, time.Now(), atExpiry, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := bearer.NewTokenResponse(atSignature, int(m.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	if refreshable {
		// create refresh token data
		refreshTokenData := &TokenData{
			Scope:     scope,
			ExpiresAt: rtExpiry,
			ClientID:  client.ID(),
		}

		// set resource owner id if available
		if resourceOwner != nil {
			roID := resourceOwner.ID()
			refreshTokenData.ResourceOwnerID = &roID
		}

		// save refresh token
		rt := m.saveRefreshToken(refreshTokenData)

		// generate new refresh token
		rtSignature, err := generateRefreshToken(rt.ID(), m.policy.Secret, time.Now(), rtExpiry)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	// run automated cleanup if enabled
	if m.policy.AutomatedCleanup {
		m.cleanup()
	}

	return res
}

func (m *Manager) getFirstClient(id string) Client {
	// check all available models in order
	for _, model := range m.policy.Clients {
		c := m.getClient(model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (m *Manager) getClient(model Client, id string) Client {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := m.store.Copy()
	defer store.Close()

	// get description
	desc := model.DescribeClient()

	// get id field
	field := model.Meta().FindField(desc.IdentifierField)

	// query db
	err := store.C(model).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	client := fire.Init(obj).(Client)

	return client
}

func (m *Manager) findFirstResourceOwner(id string) ResourceOwner {
	// check all available models in order
	for _, model := range m.policy.ResourceOwners {
		ro := m.findResourceOwner(model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (m *Manager) findResourceOwner(model ResourceOwner, id string) ResourceOwner {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := m.store.Copy()
	defer store.Close()

	// get description
	desc := model.DescribeResourceOwner()

	// get id field
	field := model.Meta().FindField(desc.IdentifierField)

	// query db
	err := store.C(model).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := fire.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (m *Manager) getFirstResourceOwner(id bson.ObjectId) ResourceOwner {
	// check all available models in order
	for _, model := range m.policy.ResourceOwners {
		ro := m.getResourceOwner(model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (m *Manager) getResourceOwner(model ResourceOwner, id bson.ObjectId) ResourceOwner {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := m.store.Copy()
	defer store.Close()

	// query db
	err := store.C(model).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := fire.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (m *Manager) getAccessToken(id bson.ObjectId) Token {
	return m.getToken(m.policy.AccessToken, id)
}

func (m *Manager) getRefreshToken(id bson.ObjectId) Token {
	return m.getToken(m.policy.RefreshToken, id)
}

func (m *Manager) getToken(t Token, id bson.ObjectId) Token {
	// prepare object
	obj := t.Meta().Make()

	// get store
	store := m.store.Copy()
	defer store.Close()

	// fetch access token
	err := store.C(t).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize access token
	accessToken := fire.Init(obj).(Token)

	return accessToken
}

func (m *Manager) saveAccessToken(d *TokenData) Token {
	return m.saveToken(m.policy.AccessToken, d)
}

func (m *Manager) saveRefreshToken(d *TokenData) Token {
	return m.saveToken(m.policy.RefreshToken, d)
}

func (m *Manager) saveToken(t Token, d *TokenData) Token {
	// prepare access token
	token := t.Meta().Make().(Token)

	// set access token data
	token.SetTokenData(d)

	// get store
	store := m.store.Copy()
	defer store.Close()

	// save access token
	err := store.C(token).Insert(token)

	// abort on error
	stack.AbortIf(err)

	return token
}

func (m *Manager) deleteAccessToken(id bson.ObjectId, clientID bson.ObjectId) {
	m.deleteToken(m.policy.AccessToken, id, clientID)
}

func (m *Manager) deleteRefreshToken(id bson.ObjectId, clientID bson.ObjectId) {
	m.deleteToken(m.policy.RefreshToken, id, clientID)
}

func (m *Manager) deleteToken(t Token, id bson.ObjectId, clientID bson.ObjectId) {
	// get store
	store := m.store.Copy()
	defer store.Close()

	// delete token
	err := store.C(t).RemoveId(id)
	if err == mgo.ErrNotFound {
		err = nil
	}

	// abort on critical error
	stack.AbortIf(err)
}

func (m *Manager) cleanup() {
	// remove all expired access tokens
	m.cleanupToken(m.policy.AccessToken)

	// remove all expired refresh tokens
	m.cleanupToken(m.policy.RefreshToken)
}

func (m *Manager) cleanupToken(t Token) {
	// get store
	store := m.store.Copy()
	defer store.Close()

	// get description
	desc := t.DescribeToken()

	// get expires at field
	field := t.Meta().FindField(desc.ExpiresAtField)

	// remove all records
	_, err := store.C(t).RemoveAll(bson.M{
		field.BSONName: bson.M{
			"$lt": time.Now(),
		},
	})

	// abort on error
	stack.AbortIf(err)
}
