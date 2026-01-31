/*
 * Copyright 2026 The HydraSFTP Team
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package hydrasftp

import (
	"context"
	"hydrasftp/internal/network"
	"hydrasftp/internal/pfte"
)

// Client is the main interface for the library
type Client struct {
	engine *pfte.Engine
}

// NewClient creates a new HydraSFTP instance
func NewClient() *Client {
	return &Client{
		engine: pfte.NewEngine(),
	}
}

// Session represents a connection to a remote server
type Session struct {
	inner *network.SftpSession
}

// NewSession prepares a new connection (it doesn't connect yet)
func NewSession(host string, port int, user, password string) *Session {
	return &Session{
		inner: network.NewSession(host, port, user, password),
	}
}

// Connect opens the SSH tunnel and SFTP subsystem
func (s *Session) Connect() error {
	if err := s.inner.Connect(); err != nil {
		return err
	}
	return s.inner.OpenSFTP()
}

// Close shuts down the connection
func (s *Session) Close() {
	s.inner.Close()
}

func (c *Client) Transfer(ctx context.Context, sessions []*Session, operation, source, dest string) error {
	sftpSessions := make([]*network.SftpSession, len(sessions))
	for i, s := range sessions {
		sftpSessions[i] = s.inner
	}
	return c.engine.StartTransfer(ctx, sftpSessions, operation, source, dest)
}

func (c *Client) UploadFile(ctx context.Context, sessions []*Session, local, remote string) error {
	s := make([]*network.SftpSession, len(sessions))
	for i, sess := range sessions {
		s[i] = sess.inner
	}
	return c.engine.UploadSpecificFile(ctx, s, local, remote)
}

func (c *Client) DownloadFile(ctx context.Context, sessions []*Session, remote, local string) error {
	s := make([]*network.SftpSession, len(sessions))
	for i, sess := range sessions {
		s[i] = sess.inner
	}
	return c.engine.DownloadSpecificFile(ctx, s, remote, local)
}

func (c *Client) SetMode(mode pfte.TransferMode) {
	c.engine.Mode = mode
}
