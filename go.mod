module github.com/morozov/rtm-cli-go

go 1.26.2

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/morozov/rtm-gen-go v0.0.0-00010101000000-000000000000 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
)

tool github.com/morozov/rtm-gen-go/cmd/rtm-gen

replace github.com/morozov/rtm-gen-go => ../rtm-gen-go
