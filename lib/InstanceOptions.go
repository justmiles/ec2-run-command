package ec2

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/packer/common/random"
	"golang.org/x/crypto/ssh"
)

// InstanceOptions asdf
type InstanceOptions struct {
	AMI                    string
	AMIID                  string
	AMIFilter              []string
	Subnet                 string
	SubnetID               string
	SubnetFilter           []string
	SecurityGroups         []string
	SecurityGroupIDs       []string
	SecurityGroupFilters   []string
	IamInstanceProfile     string
	Count                  int
	SSHKey                 string
	SSHPort                int
	User                   string
	IdentityFile           string
	Tags                   []string
	InstanceTypes          []string
	BidPrice               float64
	UserDataFile           string
	EntrypointFile         string
	WaitOnCloudInit        bool
	UsePublicIP            bool
	Attach                 bool
	NoTermination          bool
	Command                string
	EnvVars                []string
	CreateFleetRetries     int64
	LaunchTemplateName     string
	BlockDurationInMinutes int64
}

// ttyColors generated with the following
// package main
// import (
// 	"fmt"
// 	colorful "github.com/lucasb-eyer/go-colorful"
// )
//
// func main() {
// 	for i := 1; i <= 30; i++ {
// 		fmt.Printf("\"%s\",\n", colorful.FastWarmColor().Hex())
// 	}
// }
var ttyColors = [...]string{
	"#1c417f",
	"#308163",
	"#543826",
	"#428b30",
	"#536527",
	"#296358",
	"#2c285c",
	"#2f8e3f",
	"#4a8637",
	"#6a821f",
	"#2c5259",
	"#112853",
	"#3e5b81",
	"#417635",
	"#2d5c6d",
	"#2a5862",
	"#783890",
	"#215413",
	"#5e2d33",
	"#302b64",
	"#8a264e",
	"#48316d",
	"#972d6f",
	"#723652",
	"#67163a",
	"#2f2278",
	"#2d3686",
	"#469869",
	"#843566",
	"#3d4580",
}

// Instances returns a slice of Instances
func (opts *InstanceOptions) Instances() (instances []*Instance, err error) {

	amiID, err := opts.DetermineAMIID()
	if err != nil {
		return nil, err
	}

	subnetID, err := opts.DetermineSubnetID()
	if err != nil {
		return nil, err
	}

	sshKeyName, sshConfig, err := opts.DetermineSSHConfigs()
	if err != nil {
		return nil, err
	}

	securityGroupIDs, err := opts.DetermineSecurityGroupIDs()
	if err != nil {
		return nil, err
	}
	tags, err := opts.ParseTags()
	if err != nil {
		return nil, err
	}
	envVars, err := opts.ParseEnvVars()
	if err != nil {
		return nil, err
	}

	// Generate a random launch template name for spot fleet to avoid conflicting with other
	// fleets running in this AWS account
	launchTemplateName := fmt.Sprintf(
		"%s-%s", opts.LaunchTemplateName,
		random.AlphaNum(7))

	// Build each Instance's configs
	for i := 1; i <= opts.Count; i++ {
		var instance Instance
		instance.AMIID = amiID
		instance.SubnetID = subnetID
		instance.SecurityGroupIDs = securityGroupIDs
		instance.sshConfig = sshConfig
		instance.KeyName = sshKeyName
		instance.Tags = tags
		instance.BidPrice = &opts.BidPrice
		instance.WaitOnCloudInit = &opts.WaitOnCloudInit
		instance.Attach = &opts.Attach
		instance.UsePublicIP = &opts.UsePublicIP
		instance.InstanceTypes = &opts.InstanceTypes
		instance.NoTermination = &opts.NoTermination
		instance.SSHPort = &opts.SSHPort
		instance.ExitCode = aws.Int(-1)
		instance.EnvVars = envVars
		instance.LaunchTemplateName = &launchTemplateName
		instance.CreateFleetRetries = &opts.CreateFleetRetries
		instance.BlockDurationInMinutes = &opts.BlockDurationInMinutes

		if i > 1 {
			instance.TTYColor = &ttyColors[i]
		}

		if opts.IamInstanceProfile != "" {
			instance.IamInstanceProfile = &opts.IamInstanceProfile
		}

		if opts.UserDataFile != "" {
			data, err := ioutil.ReadFile(opts.UserDataFile)
			if err != nil {
				return nil, fmt.Errorf("Unable to read user-data file %s: %s", opts.UserDataFile, err)
			}
			encodedData := base64.StdEncoding.EncodeToString(data)
			instance.UserData = &encodedData
		}

		if opts.EntrypointFile != "" {
			instance.EntrypointFile = &opts.EntrypointFile
		}

		if opts.Command != "" {
			instance.Command = &opts.Command
		}

		instances = append(instances, &instance)

	}

	return instances, nil
}

// ParseTags and return a tag map for Instance
func (opts *InstanceOptions) ParseTags() (*map[string]string, error) {
	tags := make(map[string]string)
	for _, tag := range opts.Tags {
		s := strings.SplitN(tag, "=", 2)
		if len(s) != 2 {
			return &tags, fmt.Errorf("unable to derive tag from: %s", tag)
		}
		tags[s[0]] = s[1]
	}

	return &tags, nil
}

// ParseEnvVars and return a EnvVar map for Instance
func (opts *InstanceOptions) ParseEnvVars() (*map[string]string, error) {
	envVars := make(map[string]string)
	for _, envVar := range opts.EnvVars {
		s := strings.SplitN(envVar, "=", 2)
		if len(s) != 2 {
			return &envVars, fmt.Errorf("unable to derive environment from: %s", envVar)
		}
		envVars[s[0]] = s[1]
	}

	return &envVars, nil
}

// DetermineAMIID returns the AMI id using the options for AMI name or AMI Filter
func (opts *InstanceOptions) DetermineAMIID() (*string, error) {

	// if we already have an ID, return its pointer
	if opts.AMIID != "" {
		return &opts.AMIID, nil
	}

	// No AMI Id provided, look for it in AWS using filters

	var filters []*ec2.Filter

	// if the Subnet name is set, find and return AMI by name
	if opts.AMI != "" {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("name"),
			Values: []*string{&opts.AMI},
		})
	}

	// if filters are set, find and return AMI by filters
	for _, filter := range opts.AMIFilter {
		s := strings.SplitN(filter, "=", 2)
		if len(s) != 2 {
			return nil, fmt.Errorf("unable to derive filter from: %s", filter)
		}
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(s[0]),
			Values: aws.StringSlice(strings.Split(s[1], ",")),
		})
	}

	result, err := ec2Svc.DescribeImages(&ec2.DescribeImagesInput{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to find AMI: %s", err)
	}

	if len(result.Images) < 1 {
		return nil, errors.New("no AMI found matching your filters")
	}

	// Sort by created date
	sort.Slice(result.Images, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, *result.Images[i].CreationDate)
		t2, _ := time.Parse(time.RFC3339, *result.Images[j].CreationDate)
		return t1.Unix() < t2.Unix()
	})

	return result.Images[len(result.Images)-1].ImageId, nil
}

// DetermineSecurityGroupIDs returns the AMI id using the options for AMI name or AMI Filter
func (opts *InstanceOptions) DetermineSecurityGroupIDs() ([]*string, error) {
	var securityGroupIds []*string

	// if we already have an IDs, their pointer
	for _, id := range opts.SecurityGroupIDs {
		securityGroupIds = append(securityGroupIds, &id)
	}

	if len(opts.SecurityGroupFilters) < 1 && len(opts.SecurityGroups) < 1 {
		return securityGroupIds, nil
	}

	var filters []*ec2.Filter

	// if filters are set, find and return AMI by filters
	for _, filter := range opts.SecurityGroupFilters {
		s := strings.SplitN(filter, "=", 2)
		if len(s) != 2 {
			return nil, fmt.Errorf("unable to derive filter from: %s", filter)
		}

		filters = append(filters, &ec2.Filter{
			Name:   aws.String(s[0]),
			Values: aws.StringSlice(strings.Split(s[1], ",")),
		})
	}

	if len(opts.SecurityGroups) > 0 {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("group-name"),
			Values: aws.StringSlice(opts.SecurityGroups),
		})
	}

	result, err := ec2Svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("Unable to find AMI: %s", err)
	}

	if len(result.SecurityGroups) < 1 {
		return nil, errors.New("no security groups found matching your filters")
	}

	for _, id := range result.SecurityGroups {
		securityGroupIds = append(securityGroupIds, id.GroupId)
	}

	return securityGroupIds, nil
}

// DetermineSubnetID returns the Subnet id using the options for Subnet name or Subnet Filter
func (opts *InstanceOptions) DetermineSubnetID() (*string, error) {

	// if we already have an ID, return its pointer
	if opts.SubnetID != "" {
		return &opts.SubnetID, nil
	}

	var filters []*ec2.Filter

	if opts.Subnet != "" {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String("tag:Name"),
			Values: []*string{&opts.Subnet},
		})
	}

	// if filters are set, find and return subnet by filters
	for _, filter := range opts.SubnetFilter {
		s := strings.SplitN(filter, "=", 2)
		if len(s) != 2 {
			return nil, fmt.Errorf("unable to derive filter from: %s", filter)
		}
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(s[0]),
			Values: aws.StringSlice(strings.Split(s[1], ",")),
		})
	}

	result, err := ec2Svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to find subnet: %s", err)
	}

	if len(result.Subnets) == 0 {
		return nil, fmt.Errorf("No subnets matching current filters")
	}

	// TODO: return random subnet instead of just the first
	return result.Subnets[0].SubnetId, nil
}

// DetermineSSHConfigs returns pointers to the key name and identity
func (opts *InstanceOptions) DetermineSSHConfigs() (sshKeyName *string, sshConfig *ssh.ClientConfig, err error) {
	var sshKeyIdentity *string

	// Pull in identiity filef
	if opts.SSHKey != "" || opts.IdentityFile != "" {

		// Handle missing identity file
		if opts.IdentityFile == "" {
			return nil, nil, fmt.Errorf("Unable to use private key '%s' without identity file", opts.SSHKey)
		}

		// Handle SSHKey
		if opts.SSHKey == "" {
			return nil, nil, fmt.Errorf("Unable to use identity file '%s' without an AWS SSH key set", opts.IdentityFile)
		}

		data, err := ioutil.ReadFile(opts.IdentityFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Unable to read identity-file %s: %s", opts.IdentityFile, err)
		}

		sshKeyName = &opts.SSHKey
		sshKeyIdentity = aws.String(string(data))

		// Generate an ephemeral SSHKey if one is not set
	} else {
		sshKeyName, sshKeyIdentity, err = newKeyPair(opts.NoTermination)
		if err != nil {
			return nil, nil, err
		}
	}

	signer, err := ssh.ParsePrivateKey([]byte(*sshKeyIdentity))

	sshConfig = &ssh.ClientConfig{
		User:            opts.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return sshKeyName, sshConfig, nil
}
