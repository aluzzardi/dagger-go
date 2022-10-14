package embedded

import (
	"context"
	"net"
	"net/http"
	"net/url"

	"go.dagger.io/dagger/engine"
	"go.dagger.io/dagger/router"
	"go.dagger.io/dagger/sdk/go/dagger/engineconn"
)

func init() {
	engineconn.Register("embedded", New)
}

var _ engineconn.EngineConn = &Embedded{}

type Embedded struct {
	stopCh chan struct{}
	doneCh chan error
}

func New(_ *url.URL) (engineconn.EngineConn, error) {
	return &Embedded{
		stopCh: make(chan struct{}),
		doneCh: make(chan error),
	}, nil
}

func (c *Embedded) Connect(ctx context.Context, cfg *engineconn.Config) (*http.Client, error) {
	started := make(chan struct{})
	var client *http.Client

	engineCfg := &engine.Config{
		Workdir:      cfg.Workdir,
		ConfigPath:   cfg.ConfigPath,
		LocalDirs:    cfg.LocalDirs,
		NoExtensions: cfg.NoExtensions,
	}
	go func() {
		defer close(c.doneCh)
		err := engine.Start(ctx, engineCfg, func(ctx context.Context, r *router.Router) error {
			client = &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						// TODO: not efficient, but whatever
						serverConn, clientConn := net.Pipe()
						go r.ServeConn(serverConn)

						return clientConn, nil
					},
				},
			}
			close(started)
			<-c.stopCh
			return nil
		})
		c.doneCh <- err
	}()

	select {
	case <-started:
		return client, nil
	case err := <-c.doneCh:
		return client, err
	}
}

func (c *Embedded) Close() error {
	// Check if it's already closed
	select {
	case <-c.stopCh:
		return nil
	default:
	}

	close(c.stopCh)
	return <-c.doneCh
}
