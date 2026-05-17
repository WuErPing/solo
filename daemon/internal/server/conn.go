package server

import "github.com/WuErPing/solo/daemon/internal/wsconn"

// WSConn is re-exported from the shared wsconn package for convenience.
type WSConn = wsconn.WSConn

// PingableConn is re-exported from the shared wsconn package for convenience.
type PingableConn = wsconn.PingableConn

// WriteDeadlineConn is re-exported from the shared wsconn package for convenience.
type WriteDeadlineConn = wsconn.WriteDeadlineConn
