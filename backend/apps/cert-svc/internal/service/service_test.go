package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
)

func TestNew_FillsDefaults(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	svc := New(Config{Router: NewRouter(le)})
	assert.Equal(t, DefaultStream, svc.cfg.Stream)
	assert.Equal(t, DefaultGroup, svc.cfg.Group)
	assert.Equal(t, DefaultBlockTimeout, svc.cfg.BlockTimeout)
	assert.Equal(t, DefaultCATimeout, svc.cfg.CARequestTimeout)
	assert.NotNil(t, svc.cfg.Logger)
}

func TestNew_HonoursOverrides(t *testing.T) {
	svc := New(Config{
		Stream:           "x",
		Group:            "y",
		BlockTimeout:     7 * time.Second,
		CARequestTimeout: 3 * time.Minute,
	})
	assert.Equal(t, "x", svc.cfg.Stream)
	assert.Equal(t, "y", svc.cfg.Group)
	assert.Equal(t, 7*time.Second, svc.cfg.BlockTimeout)
	assert.Equal(t, 3*time.Minute, svc.cfg.CARequestTimeout)
}

func TestServiceCAPick_NoRouter(t *testing.T) {
	svc := New(Config{})
	_, err := svc.caPick(context.Background(), nil)
	require.Error(t, err)
}
