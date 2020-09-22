package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"

	ec2 "github.com/justmiles/ec2-runner/lib"
	"github.com/spf13/cobra"
)

var opts ec2.InstanceOptions
var dryRun bool

func init() {
	log.SetFlags(0)
	rootCmd.AddCommand(run)
	run.Flags().SetInterspersed(false)

	run.PersistentFlags().StringVar(&opts.AMI, "ami", "", "AMI name. Supports wildcards. Newest image is returned")
	run.PersistentFlags().StringVar(&opts.AMIID, "ami-id", "", "AMI ID, overriding ami-filter or ami")
	// TODO: consider default ami filter for amazon linux 2
	run.PersistentFlags().StringArrayVar(&opts.AMIFilter, "ami-filter", nil, "'Key=Value' filters for your AMI")

	run.PersistentFlags().StringVar(&opts.Subnet, "subnet", "", "Subnet name. First match is returned")
	run.PersistentFlags().StringVar(&opts.SubnetID, "subnet-id", "", "Subnet ID, overriding subnet-filter or subnet")
	run.PersistentFlags().StringArrayVar(&opts.SubnetFilter, "subnet-filter", nil, "'Key=Value' filters for your subnet")

	run.PersistentFlags().StringVar(&opts.IamInstanceProfile, "instance-profile", "", "Role to attach to your instance")

	run.PersistentFlags().IntVarP(&opts.Count, "count", "c", 1, "Number of instances to invoke")

	run.PersistentFlags().StringVar(&opts.SSHKey, "ssh-key", "", "(optional) use this AWS SSH key. If omitted, an ephemeral key will be created")
	run.PersistentFlags().IntVar(&opts.SSHPort, "ssh-port", 22, "SSH port")
	run.PersistentFlags().StringVar(&opts.User, "user", "ec2-user", "SSH user to connect to your instance with")
	run.PersistentFlags().StringVarP(&opts.IdentityFile, "identify-file", "i", "", "If using ssh-key, pass in the identitiy file")

	run.PersistentFlags().StringArrayVar(&opts.Tags, "tag", nil, "Key=Value pair")
	run.PersistentFlags().StringArrayVar(&opts.SecurityGroupFilters, "security-group-filter", nil, "Filters for your Security Groups. Syntax: Name=string,Values=string,string ...")
	run.PersistentFlags().StringArrayVar(&opts.SecurityGroups, "security-group", nil, "Security group name")

	run.PersistentFlags().StringVarP(&opts.Type, "type", "t", "t2.micro", "instance type")

	// run.PersistentFlags().Float64Var(&opts.BidPrice, "bid-price", 0, "")

	run.PersistentFlags().StringArrayVar(&opts.EnvVars, "environment", nil, "Environment variables exported after user-data and before entry-point or command. Syntax: 'Key=Value'")

	run.PersistentFlags().StringVar(&opts.UserDataFile, "user-data", "", "path to user-data script")
	run.PersistentFlags().StringVar(&opts.EntrypointFile, "entrypoint", "", "path to entrypoint script")

	run.PersistentFlags().BoolVar(&opts.WaitOnCloudInit, "no-wait-cloud-init", true, "Do not wait for user-data to complete before invoking entrypoint and command")
	run.PersistentFlags().BoolVar(&opts.NoTermination, "no-terminate", false, "Do not terminate the instance upon completion (default true)")
	// run.PersistentFlags().BoolVarP(&opts.Attach, "attach", "a", false, "")

	run.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show details about the instance it would start, but don't actually start it")

}

// process the list command
var run = &cobra.Command{
	Use:   "run",
	Short: "Run adhoc workloads using EC2 and destroy them upon completion",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) > 0 {
			opts.Command = strings.Join(args, " ")
		}

		instances, err := opts.Instances()
		if err != nil {
			log.Fatal(err)
		}

		if dryRun {
			for _, instance := range instances {
				fmt.Println(instance)
				instance.DestroyKeyPair()
				os.Exit(0)
			}
		}

		var wg sync.WaitGroup

		for _, instance := range instances {
			wg.Add(1)
			go func(instance *ec2.Instance) {
				defer wg.Done()
				err = instance.Start()
				if err != nil {
					fmt.Println(err)
					err = instance.Terminate()
					if err != nil {
						fmt.Println(err)
					}
					return
				}

				fmt.Printf(
					"Instance %s starting with IP %s\n  AMI: %s\n  Spot Price: %s\n  Size: %s\n",
					*instance.InstanceID,
					*instance.PrivateIPAddress,
					*instance.AMIID,
					*instance.SpotPrice,
					*instance.Type,
				)
				instance.DestroyKeyPair()
				err = instance.WaitForSSH()
				if err != nil {
					fmt.Printf("error waiting for ssh: %s", err)
				}

				err = instance.InvokeCommand()
				if err != nil {
					fmt.Printf("error invoking command: %s", err)
				}
			}(instance)
		}

		// catch ctrl+c
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			for range c {
				fmt.Println("got exception")
				for _, instance := range instances {
					if !*instance.NoTermination {
						instance.Terminate()
						if opts.IdentityFile == "" {
							instance.DestroyKeyPair()
						}
					}
				}
				os.Exit(130)
			}
		}()

		wg.Wait()

		for _, instance := range instances {
			if !*instance.NoTermination {
				instance.Terminate()
				if opts.IdentityFile == "" {
					instance.DestroyKeyPair()
				}
			}
		}

		os.Exit(*instances[0].ExitCode)

	},
}
