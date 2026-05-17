module github.com/WuErPing/solo/daemon

go 1.25.6

require (
	github.com/WuErPing/solo/protocol v0.0.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
)

require (
	github.com/creack/pty v1.1.21 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace github.com/WuErPing/solo/protocol => ../protocol
