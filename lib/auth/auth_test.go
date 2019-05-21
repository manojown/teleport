/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	. "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	bk backend.Backend
	a  *AuthServer
}

var _ = Suite(&AuthSuite{})
var _ = fmt.Printf

func (s *AuthSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *AuthSuite) SetUpTest(c *C) {
	var err error
	s.bk, err = boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	authConfig := &InitConfig{
		ClusterName: clusterName,
		Backend:     s.bk,
		Authority:   authority.New(),
	}
	s.a, err = NewAuthServer(authConfig)
	c.Assert(err, IsNil)

	// set cluster name
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "me.localhost")), IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	user := "user1"
	pass := []byte("abc123")

	ws, err := s.a.SignIn(user, pass)
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	ws, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	err = s.a.DeleteWebSession(user, ws.GetName())
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(user, ws.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *AuthSuite) TestUserLock(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "me.localhost")), IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	user := "user1"
	pass := []byte("abc123")

	ws, err := s.a.SignIn(user, pass)
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	// successful log in
	ws, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.clock = fakeClock

	for i := 0; i <= defaults.MaxLoginAttempts; i++ {
		_, err = s.a.SignIn(user, []byte("wrong pass"))
		c.Assert(err, NotNil)
	}

	// make sure user is locked
	_, err = s.a.SignIn(user, pass)
	c.Assert(err, ErrorMatches, ".*locked.*")

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	_, err = s.a.SignIn(user, pass)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	// before we do anything, we should have 0 tokens
	btokens, err := s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(btokens), Equals, 0)

	// generate persistent token
	tok, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}})
	c.Assert(err, IsNil)
	c.Assert(len(tok), Equals, 2*TokenLenBytes)

	tokens, err := s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].Token, Equals, tok)

	roles, err := s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(roles.Include(teleport.RoleProxy), Equals, false)

	// unsuccessful registration (wrong role)
	keys, err := s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    tok,
		HostID:   "bad-host-id",
		NodeName: "bad-node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(keys, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, `node "bad-node-name" \[bad-host-id\] can not join the cluster, the token does not allow "Proxy" role`)

	roles, err = s.a.ValidateToken(tok)
	c.Assert(err, IsNil)

	// generate predefined token
	customToken := "custom token"
	tok, err = s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}, Token: customToken})
	c.Assert(err, IsNil)
	c.Assert(tok, Equals, customToken)

	roles, err = s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(roles.Include(teleport.RoleProxy), Equals, false)

	err = s.a.DeleteToken(customToken)
	c.Assert(err, IsNil)

	// generate multi-use token with long TTL:
	multiUseToken, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleProxy}, TTL: time.Hour})
	c.Assert(err, IsNil)
	_, err = s.a.ValidateToken(multiUseToken)
	c.Assert(err, IsNil)

	// use it twice:
	keys, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:                multiUseToken,
		HostID:               "once",
		NodeName:             "node-name",
		Role:                 teleport.RoleProxy,
		AdditionalPrincipals: []string{"example.com"},
	})
	c.Assert(err, IsNil)

	// along the way, make sure that additional principals work
	key, _, _, _, err := ssh.ParseAuthorizedKey(keys.Cert)
	c.Assert(err, IsNil)
	hostCert := key.(*ssh.Certificate)
	comment := Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(utils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), Equals, true, comment)

	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "twice",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, IsNil)

	// try to use after TTL:
	s.a.clock = clockwork.NewFakeClockAt(time.Now().UTC().Add(time.Hour + 1))
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "late.bird",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, ErrorMatches, `node "node-name" \[late.bird\] can not join the cluster, token has expired`)

	// expired token should be gone now
	err = s.a.DeleteToken(multiUseToken)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	// lets use static tokens now
	roles = teleport.Roles{teleport.RoleProxy}
	st, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{services.ProvisionToken{Token: "static-token-value", Roles: roles, Expires: time.Unix(0, 0).UTC()}},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(st)
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "static.host",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "wrong.role",
		NodeName: "node-name",
		Role:     teleport.RoleAuth,
	})
	c.Assert(err, NotNil)
	r, err := s.a.ValidateToken("static-token-value")
	c.Assert(err, IsNil)
	c.Assert(r, DeepEquals, roles)

	// List tokens (should see 2: one static, one regular)
	tokens, err = s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 2)
}

func (s *AuthSuite) TestBadTokens(c *C) {
	// empty
	_, err := s.a.ValidateToken("")
	c.Assert(err, NotNil)

	// garbage
	_, err = s.a.ValidateToken("bla bla")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleAuth}})
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(tampered)
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestBuildRolesInvalid(c *C) {
	// create a connector
	oidcConnector := services.NewOIDCConnector("example", services.OIDCConnectorSpecV2{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
	})

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	// create an identity for the ttl
	ident := &oidc.Identity{
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	// try and build roles should be invalid since we have no mappings
	_, err := s.a.buildRoles(oidcConnector, ident, claims)
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestBuildRolesStatic(c *C) {
	// create a connector
	oidcConnector := services.NewOIDCConnector("example", services.OIDCConnectorSpecV2{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []services.ClaimMapping{
			services.ClaimMapping{
				Claim: "roles",
				Value: "teleport-user",
				Roles: []string{"user"},
			},
		},
	})

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	// create an identity for the ttl
	ident := &oidc.Identity{
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	// build roles and check that we mapped to "user" role
	roles, err := s.a.buildRoles(oidcConnector, ident, claims)
	c.Assert(err, IsNil)
	c.Assert(roles, HasLen, 1)
	c.Assert(roles[0], Equals, "user")
}

func (s *AuthSuite) TestBuildRolesTemplate(c *C) {
	// create a connector
	oidcConnector := services.NewOIDCConnector("example", services.OIDCConnectorSpecV2{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []services.ClaimMapping{
			services.ClaimMapping{
				Claim: "roles",
				Value: "teleport-user",
				RoleTemplate: &services.RoleV2{
					Kind:    services.KindRole,
					Version: services.V2,
					Metadata: services.Metadata{
						Name:      `{{index . "email"}}`,
						Namespace: defaults.Namespace,
					},
					Spec: services.RoleSpecV2{
						MaxSessionTTL: services.NewDuration(90 * 60 * time.Minute),
						Logins:        []string{`{{index . "nickname"}}`, `root`},
						NodeLabels:    map[string]string{"*": "*"},
						Namespaces:    []string{"*"},
					},
				},
			},
		},
	})

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	// create an identity for the ttl
	ident := &oidc.Identity{
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	// build roles
	roles, err := s.a.buildRoles(oidcConnector, ident, claims)
	c.Assert(err, IsNil)

	// check that the newly created role was both returned and upserted into the backend
	r, err := s.a.GetRoles()
	c.Assert(err, IsNil)
	c.Assert(r, HasLen, 1)
	c.Assert(r[0].GetName(), Equals, "foo@example.com")
	c.Assert(roles, HasLen, 1)
	c.Assert(roles[0], Equals, "foo@example.com")
}

func (s *AuthSuite) TestValidateACRValues(c *C) {

	var tests = []struct {
		inIDToken     string
		inACRValue    string
		inACRProvider string
		outIsValid    bool
	}{
		// 0 - default, acr values match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo",
			"",
			true,
		},
		// 1 - default, acr values do not match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"bar",
			"",
			false,
		},
		// 2 - netiq, acr values match
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			true,
		},
		// 3 - netiq, invalid format
		{
			`
{
    "acr": {
        "values": "foo/bar/baz"
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
		// 4 - netiq, invalid value
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz/qux"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
	}

	for i, tt := range tests {
		comment := Commentf("Test %v", i)

		var claims jose.Claims
		err := json.Unmarshal([]byte(tt.inIDToken), &claims)
		c.Assert(err, IsNil, comment)

		err = s.a.validateACRValues(tt.inACRValue, tt.inACRProvider, claims)
		if tt.outIsValid {
			c.Assert(err, IsNil, comment)
		} else {
			c.Assert(err, NotNil, comment)
		}
	}
}

func (s *AuthSuite) TestUpdateConfig(c *C) {
	cn, err := s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, "me.localhost")
	st, err := s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, []services.ProvisionToken{})

	// try and set cluster name, this should fail because you can only set the
	// cluster name once
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "foo.localhost",
	})
	c.Assert(err, IsNil)
	// use same backend but start a new auth server with different config.
	authConfig := &InitConfig{
		ClusterName: clusterName,
		Backend:     s.bk,
		Authority:   authority.New(),
	}
	authServer, err := NewAuthServer(authConfig)
	c.Assert(err, IsNil)

	err = authServer.SetClusterName(clusterName)
	c.Assert(err, NotNil)
	// try and set static tokens, this should be successful because the last
	// one to upsert tokens wins
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{services.ProvisionToken{
			Token: "bar",
			Roles: teleport.Roles{teleport.Role("baz")},
		}},
	})
	c.Assert(err, IsNil)
	err = authServer.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	// check first auth server and make sure it returns the correct values
	// (original cluster name, new static tokens)
	cn, err = s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, "me.localhost")
	st, err = s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, []services.ProvisionToken{services.ProvisionToken{
		Token: "bar",
		Roles: teleport.Roles{teleport.Role("baz")},
	}})

	// check second auth server and make sure it also has the correct values
	// new static tokens
	st, err = authServer.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, []services.ProvisionToken{services.ProvisionToken{
		Token: "bar",
		Roles: teleport.Roles{teleport.Role("baz")},
	}})
}

// TestMigrateRemote cluster creates remote cluster resource
// after the migration
func (s *AuthSuite) TestMigrateRemoteCluster(c *C) {
	clusterName := "remote.example.com"

	hostCA := suite.NewTestCA(services.HostCA, clusterName)
	hostCA.SetName(clusterName)
	c.Assert(s.a.UpsertCertAuthority(hostCA), IsNil)

	err := migrateRemoteClusters(s.a)
	c.Assert(err, IsNil)

	remoteCluster, err := s.a.GetRemoteCluster(clusterName)
	c.Assert(err, IsNil)
	c.Assert(remoteCluster.GetName(), Equals, clusterName)
}

// TestMigrateEnabledTrustedCluster tests migrations of enabled trusted cluster
func (s *AuthSuite) TestMigrateEnabledTrustedCluster(c *C) {
	clusterName := "example.com"
	resourceName := "trustedcluster1"

	tunnel := services.NewReverseTunnel(resourceName, []string{"addr:5000"})
	err := s.a.UpsertReverseTunnel(tunnel)
	c.Assert(err, IsNil)

	hostCA := suite.NewTestCA(services.HostCA, clusterName)
	hostCA.SetName(resourceName)
	c.Assert(s.a.UpsertCertAuthority(hostCA), IsNil)

	userCA := suite.NewTestCA(services.UserCA, clusterName)
	userCA.SetName(resourceName)
	c.Assert(s.a.UpsertCertAuthority(userCA), IsNil)

	tc, err := services.NewTrustedCluster(resourceName, services.TrustedClusterSpecV2{
		Enabled:      true,
		Token:        "shmoken",
		ProxyAddress: "addr:5000",
		RoleMap: services.RoleMap{
			{Local: []string{"local"}, Remote: "remote"},
		},
	})
	c.Assert(err, IsNil)
	_, err = s.a.Presence.UpsertTrustedCluster(tc)
	c.Assert(err, IsNil)

	err = migrateTrustedClusters(s.a)
	c.Assert(err, IsNil)

	_, err = s.a.GetTrustedCluster(resourceName)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetTrustedCluster(clusterName)
	c.Assert(err, IsNil)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: clusterName}, false)
	c.Assert(err, IsNil)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: resourceName}, false)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: clusterName}, false)
	c.Assert(err, IsNil)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: resourceName}, false)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetReverseTunnel(resourceName)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetReverseTunnel(clusterName)
	c.Assert(err, IsNil)
}

// TestMigrateDisabledTrustedCluster tests migrations of disabled trusted cluster
func (s *AuthSuite) TestMigrateDisabledTrustedCluster(c *C) {
	clusterName := "example.com"
	resourceName := "trustedcluster1"

	hostCA := suite.NewTestCA(services.HostCA, clusterName)
	hostCA.SetName(resourceName)
	c.Assert(s.a.UpsertCertAuthority(hostCA), IsNil)

	userCA := suite.NewTestCA(services.UserCA, clusterName)
	userCA.SetName(resourceName)
	c.Assert(s.a.UpsertCertAuthority(userCA), IsNil)

	err := s.a.DeactivateCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: resourceName})
	c.Assert(err, IsNil)

	err = s.a.DeactivateCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: resourceName})
	c.Assert(err, IsNil)

	tc, err := services.NewTrustedCluster(resourceName, services.TrustedClusterSpecV2{
		Enabled:      false,
		Token:        "shmoken",
		ProxyAddress: "addr",
		RoleMap: services.RoleMap{
			{Local: []string{"local"}, Remote: "remote"},
		},
	})
	c.Assert(err, IsNil)
	_, err = s.a.Presence.UpsertTrustedCluster(tc)
	c.Assert(err, IsNil)

	err = migrateTrustedClusters(s.a)
	c.Assert(err, IsNil)

	_, err = s.a.GetTrustedCluster(resourceName)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetTrustedCluster(clusterName)
	c.Assert(err, IsNil)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: clusterName}, false)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: resourceName}, false)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: clusterName}, false)
	fixtures.ExpectNotFound(c, err)

	_, err = s.a.GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: resourceName}, false)
	fixtures.ExpectNotFound(c, err)
}
