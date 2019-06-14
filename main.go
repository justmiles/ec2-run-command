package main

import (
	"github.com/justmiles/ec2-runner/cmd"
)

// Version of ec2-runner. Overwritten during build
var Version = "development"

func main() {
	cmd.Execute(Version )
}
