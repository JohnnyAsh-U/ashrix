package router

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/policy/store"
	"github.com/JohnnyAsh-U/ashrix-api/internal/gateway/registry"
	"github.com/JohnnyAsh-U/ashrix-api/pkg/flow"
	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type mockTunnelSession struct {
	opened bool
	req    flow.OpenRequest
}

func (m *mockTunnelSession) OpenStream(ctx context.Context, req flow.OpenRequest) (flow.Stream, error) {
	m.opened = true
	m.req = req
	return &mockStream{}, nil
}

func (m *mockTunnelSession) Close() error {
	return nil
}

type mockStream struct{}

func (m *mockStream) Read(p []byte) (n int, err error)   { return 0, nil }
func (m *mockStream) Write(p []byte) (n int, err error)  { return len(p), nil }
func (m *mockStream) Close() error                      { return nil }
func (m *mockStream) CloseRead() error                  { return nil }
func (m *mockStream) CloseWrite() error                 { return nil }
func (m *mockStream) Context() context.Context          { return context.Background() }

func TestRouter(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()
	tempDir := t.TempDir()

	boltStore, err := store.OpenBoltStore(tempDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		boltStore.Close()
	})

	reg := registry.New(logger)

	// Create and register a connector
	app := &proto.ConnectorApps{
		Id:        "app-1",
		Name:      "App 1",
		Subdomain: "app-1.sub",
	}
	session := &mockTunnelSession{}

	// Attach Management first
	reg.AttachManagement("conn-1", "tenant-1", []*proto.ConnectorApps{app}, nil, &x509.Certificate{}, "active")
	// Attach Tunnel
	reg.AttachTunnel("conn-1", session, &x509.Certificate{}, "quic")

	// Set up SOCKS5 credentials
	passHash, err := bcrypt.GenerateFromPassword([]byte("secret-pass"), 12)
	require.NoError(t, err)

	cred := &store.SOCKS5Credential{
		ConnectorID:  "conn-1",
		Username:     "socks-user",
		PasswordHash: string(passHash),
		CreatedAt:    time.Now(),
	}
	err = boltStore.SetSOCKS5Credential(ctx, cred)
	require.NoError(t, err)

	// Set up policies: allow m2m group (App-to-App) to access app-1, and allow specific user (User-to-App)
	engine, err := policy.NewEngine(ctx, boltStore, nil, "GATE-1", logger)
	require.NoError(t, err)

	r := NewRouter(reg, engine, boltStore)

	// Policy Rule: Allow group "m2m"
	rule1 := &proto.PolicyRule{
		PolicyId: "allow-m2m",
		TenantId: "tenant-1",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Priority: 10,
		Version:  1,
		Subject: &proto.SubjectSelector{
			Groups: []string{"m2m"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"app-1"},
		},
	}
	rec1 := &proto.PolicyRecord{
		Sequence:  1,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Rule:      rule1,
	}

	// Policy Rule: Allow specific user
	rule2 := &proto.PolicyRule{
		PolicyId: "allow-user",
		TenantId: "tenant-1",
		Effect:   proto.EffectEnum_EFFECT_ENUM_ALLOW,
		Priority: 10,
		Version:  1,
		Subject: &proto.SubjectSelector{
			Users: []string{"user-alice"},
		},
		Resource: &proto.ResourceSelector{
			AppIds: []string{"app-1"},
		},
	}
	rec2 := &proto.PolicyRecord{
		Sequence:  2,
		Operation: proto.OperationEnum_OPERATION_ENUM_UPSERT,
		Rule:      rule2,
	}

	err = engine.UpdatePolicies(ctx, []*proto.PolicyRecord{rec1, rec2}, store.SyncCheckpoint{LastBundleVersion: 1})
	require.NoError(t, err)

	t.Run("User to App: Allowed", func(t *testing.T) {
		req := flow.OpenRequest{
			Version:  1,
			FlowID:   "f-1",
			FlowType: flow.FlowUserToApp,
			Protocol: flow.ProtocolTCP,
			Source: flow.Endpoint{
				Type:        flow.EndpointUser,
				PrincipalID: "user-alice",
			},
			Destination: flow.Endpoint{
				Type:  flow.EndpointApp,
				AppID: "app-1",
			},
		}

		session.opened = false
		stream, err := r.Route(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, stream)
		assert.True(t, session.opened)
		assert.Equal(t, "app-1", session.req.Destination.AppID)
	})

	t.Run("User to App: Denied by Policy", func(t *testing.T) {
		req := flow.OpenRequest{
			Version:  1,
			FlowID:   "f-2",
			FlowType: flow.FlowUserToApp,
			Protocol: flow.ProtocolTCP,
			Source: flow.Endpoint{
				Type:        flow.EndpointUser,
				PrincipalID: "user-bob",
			},
			Destination: flow.Endpoint{
				Type:  flow.EndpointApp,
				AppID: "app-1",
			},
		}

		stream, err := r.Route(ctx, req)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrPolicyDenied)
		assert.Nil(t, stream)
	})

	t.Run("App to App: SOCKS5 Allowed", func(t *testing.T) {
		req := flow.OpenRequest{
			Version:  1,
			FlowID:   "f-3",
			FlowType: flow.FlowAppToApp,
			Protocol: flow.ProtocolTCP,
			Source: flow.Endpoint{
				Type:        flow.EndpointApp,
				PrincipalID: "conn-1", // authenticated mTLS identity
			},
			Destination: flow.Endpoint{
				Type:  flow.EndpointApp,
				AppID: "app-1",
			},
			SocksUsername: "socks-user",
			SocksPassword: "secret-pass",
		}

		session.opened = false
		stream, err := r.Route(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, stream)
		assert.True(t, session.opened)
	})

	t.Run("App to App: SOCKS5 Denied (Wrong Username)", func(t *testing.T) {
		req := flow.OpenRequest{
			Version:  1,
			FlowID:   "f-4",
			FlowType: flow.FlowAppToApp,
			Protocol: flow.ProtocolTCP,
			Source: flow.Endpoint{
				Type:        flow.EndpointApp,
				PrincipalID: "conn-1",
			},
			Destination: flow.Endpoint{
				Type:  flow.EndpointApp,
				AppID: "app-1",
			},
			SocksUsername: "socks-hacker",
			SocksPassword: "secret-pass",
		}

		stream, err := r.Route(ctx, req)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUnauthorized)
		assert.Nil(t, stream)
	})

	t.Run("App to App: SOCKS5 Denied (Wrong Password)", func(t *testing.T) {
		req := flow.OpenRequest{
			Version:  1,
			FlowID:   "f-5",
			FlowType: flow.FlowAppToApp,
			Protocol: flow.ProtocolTCP,
			Source: flow.Endpoint{
				Type:        flow.EndpointApp,
				PrincipalID: "conn-1",
			},
			Destination: flow.Endpoint{
				Type:  flow.EndpointApp,
				AppID: "app-1",
			},
			SocksUsername: "socks-user",
			SocksPassword: "wrong-password",
		}

		stream, err := r.Route(ctx, req)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrUnauthorized)
		assert.Nil(t, stream)
	})
}
