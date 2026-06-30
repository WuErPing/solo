module github.com/WuErPing/solo/cli

go 1.25.6

replace github.com/WuErPing/solo/protocol => ../protocol

require (
	github.com/WuErPing/solo/protocol v0.0.0-00010101000000-000000000000
	github.com/fatih/color v1.19.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/spf13/cobra v1.10.2
	golang.org/x/crypto v0.51.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.44.0 // indirect
)
