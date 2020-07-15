module github.com/justmiles/ec2-runner

go 1.12

replace github.com/justmiles/ec2-runner/cmd => ./cmd

replace github.com/justmiles/ec2-runner/lib => ./lib

require (
	github.com/abice/go-enum v0.2.3 // indirect
	github.com/aokoli/goutils v1.0.1 // indirect
	github.com/aws/aws-sdk-go v1.19.29
	github.com/bramvdbogaerde/go-scp v0.0.0-20190409174733-583e65a51240
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/cobra v0.0.3
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d
	golang.org/x/text v0.3.1-0.20180807135948-17ff2d5776d2 // indirect
)
