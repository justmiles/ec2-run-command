module github.com/justmiles/ec2-runner

go 1.12

replace github.com/justmiles/ec2-runner/cmd => ./cmd

replace github.com/justmiles/ec2-runner/lib => ./lib

require (
	github.com/abice/go-enum v0.2.1 // indirect
	github.com/aws/aws-sdk-go v1.19.29
	github.com/bramvdbogaerde/go-scp v0.0.0-20190409174733-583e65a51240
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/stretchr/testify v1.3.0 // indirect
	golang.org/x/crypto v0.0.0-20190513172903-22d7a77e9e5f
	golang.org/x/net v0.0.0-20190514140710-3ec191127204 // indirect
	golang.org/x/text v0.3.1-0.20180807135948-17ff2d5776d2 // indirect
)
