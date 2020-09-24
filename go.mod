module github.com/justmiles/ec2-runner

go 1.12

replace github.com/justmiles/ec2-runner/cmd => ./cmd

replace github.com/justmiles/ec2-runner/lib => ./lib

require (
	github.com/abice/go-enum v0.2.3 // indirect
	github.com/aokoli/goutils v1.0.1 // indirect
	github.com/aws/aws-sdk-go v1.30.8
	github.com/bramvdbogaerde/go-scp v0.0.0-20190409174733-583e65a51240
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/hashicorp/packer v1.6.2
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/cobra v0.0.3
	golang.org/x/crypto v0.0.0-20200422194213-44a606286825
)
