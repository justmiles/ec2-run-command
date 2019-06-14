package ec2

import (
	// "encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	// "io/ioutil"
	// "log"
	// "math/rand"
	// "os"
	// "time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

var (
	sess = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	ec2Svc = ec2.New(sess)
	efsSvc = efs.New(sess)
)

// Instance represents a runnable instance
type Instance struct {
	AMIID              *string
	SubnetID           *string
	SecurityGroupIDs   []*string
	IamInstanceProfile *string
	sshConfig          *ssh.ClientConfig
	SSHPort            *int
	KeyName            *string
	Tags               *map[string]string
	Type               *string
	BidPrice           *float64
	SpotPrice          *string
	UserData           *string
	EntrypointFile     *string
	WaitOnCloudInit    *bool
	Attach             *bool
	NoTermination      *bool
	TTYColor           *string
	Reservation        *ec2.Reservation
	PrivateIPAddress   *string
	InstanceID         *string
	ExitCode           *int
	Command            *string
}

// Start the command
func (instance *Instance) Start() error {

	err := instance.StartInstance()
	if err != nil {
		return fmt.Errorf("error starting instance: %s", err)
	}

	return nil
}

// StartInstance launches a new EC2 instance
func (instance *Instance) StartInstance() (err error) {

	ri := ec2.RunInstancesInput{
		ImageId:      instance.AMIID,
		InstanceType: instance.Type,
		KeyName:      instance.KeyName,
		MaxCount:     aws.Int64(1),
		MinCount:     aws.Int64(1),
		UserData:     instance.UserData,

		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			&ec2.InstanceNetworkInterfaceSpecification{
				DeviceIndex:              aws.Int64(0),
				AssociatePublicIpAddress: aws.Bool(false),
				SubnetId:                 instance.SubnetID,
				Groups:                   instance.SecurityGroupIDs,
			},
		},

		InstanceInitiatedShutdownBehavior: aws.String("terminate"),
		InstanceMarketOptions: &ec2.InstanceMarketOptionsRequest{
			MarketType: aws.String("spot"),
			SpotOptions: &ec2.SpotMarketOptions{
				BlockDurationMinutes:         aws.Int64(60),
				InstanceInterruptionBehavior: aws.String("terminate"),
			},
		},
	}

	if len(*instance.Tags) > 0 {
		var ec2Tags []*ec2.Tag
		for key, value := range *instance.Tags {
			ec2Tags = append(ec2Tags, &ec2.Tag{
				Key:   &key,
				Value: &value,
			})
		}

		ri.TagSpecifications = []*ec2.TagSpecification{
			&ec2.TagSpecification{
				ResourceType: aws.String("instance"),
				Tags:         ec2Tags,
			},
			&ec2.TagSpecification{
				ResourceType: aws.String("volume"),
				Tags:         ec2Tags,
			},
		}
	}

	if instance.IamInstanceProfile != nil {
		ri.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{Name: instance.IamInstanceProfile}
	}

	instance.Reservation, err = ec2Svc.RunInstances(&ri)
	if err != nil {
		return err
	}

	for _, ri := range instance.Reservation.Instances {
		instance.PrivateIPAddress = ri.PrivateIpAddress
		instance.InstanceID = ri.InstanceId
	}

	if instance.PrivateIPAddress == nil {
		return errors.New("looks like it didn't get created")
	}

	descSpot, err := ec2Svc.DescribeSpotInstanceRequests(&ec2.DescribeSpotInstanceRequestsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("instance-id"),
				Values: []*string{instance.InstanceID},
			}},
	})
	if err != nil {
		return fmt.Errorf("Unable to describe spot instance request: %s", err)
	}
	for _, sp := range descSpot.SpotInstanceRequests {
		instance.SpotPrice = sp.ActualBlockHourlyPrice
	}

	return nil
}

// WaitForSSH connection and continue
func (instance Instance) WaitForSSH() (err error) {
	const retries = 10
	var attempts = 0
	for {
		if attempts < retries {
			attempts++
			fmt.Printf("Waiting for SSH %s:%d ... ", *instance.PrivateIPAddress, *instance.SSHPort)

			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", *instance.PrivateIPAddress, *instance.SSHPort), 15*time.Second)
			if err != nil {
				fmt.Printf("instance not yet available (attempt %d/%d)\n", attempts, retries)
				time.Sleep(5 * time.Second)
				continue
			}
			if conn != nil {
				fmt.Print("Ready!\n")
				return conn.Close()
			}

		}
		return fmt.Errorf("\nunable to connect to %s:%d", *instance.PrivateIPAddress, *instance.SSHPort)
	}
}

// InvokeCommand over ssh connection
func (instance *Instance) InvokeCommand() (err error) {

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", *instance.PrivateIPAddress, *instance.SSHPort), instance.sshConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to SSH: %s", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("unable to launch SSH session: %s", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdin for session: %v", err)
	}
	go io.Copy(stdin, os.Stdin)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stdout for session: %v", err)
	}
	go io.Copy(os.Stdout, stdout)

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("Unable to setup stderr for session: %v", err)
	}
	go io.Copy(os.Stderr, stderr)

	var commands []string

	if instance.EntrypointFile != nil {
		uploadedFilePath, err := instance.UploadFile(*instance.EntrypointFile)
		if err != nil {
			return err
		}

		if *instance.WaitOnCloudInit {
			commands = append(commands, "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do echo 'Waiting for cloud-init...'; sleep 1; done")
		}

		commands = append(commands, uploadedFilePath)

	}

	if instance.Command != nil {
		commands = append(commands, *instance.Command)

	}
	command := strings.Join(commands, " \\\n && ")
	fmt.Printf("Executing command: \n%s\n", command)
	*instance.ExitCode, err = instance.RunCommand(session, command)
	if err != nil {
		return fmt.Errorf("command exited with code %d: %s", instance.ExitCode, err)
	}

	return nil
}

// UploadFile to instance, dropping in user home
func (instance Instance) UploadFile(filename string) (string, error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("File does not exist: %s", filename)
	}
	if err != nil {
		return "", fmt.Errorf("Error uploading file: %s", err.Error())
	}

	if info.IsDir() {
		return "", fmt.Errorf("Unable to upload %s. It's a directory", filename)
	}

	// Open file
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("Could not open local file %s", err.Error())
	}

	// Close the file after it has been copied
	defer f.Close()

	client := scp.NewClient(fmt.Sprintf("%s:%d", *instance.PrivateIPAddress, *instance.SSHPort), instance.sshConfig)

	// Close client connection after the file has been copied
	defer client.Close()

	// Connect to the remote server
	err = client.Connect()
	if err != nil {
		return "", fmt.Errorf("Couldn't establish an SCP connection to %s:%d: %s", *instance.PrivateIPAddress, *instance.SSHPort, err)
	}

	// Finaly, copy the file over
	err = client.CopyFile(f, "/tmp/"+f.Name(), "0755")
	if err != nil {
		return "", fmt.Errorf("Error while copying file: %s", err.Error())
	}

	return "/tmp/" + f.Name(), nil
}

// Terminate this instance
func (instance Instance) Terminate() error {
	res, err := ec2Svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{instance.InstanceID},
	})
	if err != nil {
		return err
	}

	for _, terminatingInstance := range res.TerminatingInstances {
		fmt.Printf("\nInstance %s %s\n", *terminatingInstance.InstanceId, *terminatingInstance.CurrentState.Name)
	}
	return nil
}

// RunCommand against remote instance using SSH
func (instance Instance) RunCommand(session *ssh.Session, command string) (int, error) {

	err := session.Run(command)
	if err != nil {
		switch v := err.(type) {
		case *ssh.ExitError:
			return v.Waitmsg.ExitStatus(), nil
		default:
			return 1, err
		}
	}

	return 0, err
}

// DestroyKeyPair after instance termination
func (instance Instance) DestroyKeyPair() {
	_, err := ec2Svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: instance.KeyName,
	})

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Printf("Destroyed key pair %s\n", *instance.KeyName)
	}

}
func (instance *Instance) String() string {
	var s string

	if instance.AMIID != nil {
		s = s + fmt.Sprintf("AMIID: %s\n", *instance.AMIID)
	}

	if instance.SubnetID != nil {
		s = s + fmt.Sprintf("SubnetID: %s\n", *instance.SubnetID)
	}

	if instance.SecurityGroupIDs != nil {
		var ss []string
		for _, id := range instance.SecurityGroupIDs {
			ss = append(ss, *id)
		}
		s = s + fmt.Sprintf("SecurityGroupIDs: %s\n", strings.Join(ss, ","))
	}

	if instance.IamInstanceProfile != nil {
		s = s + fmt.Sprintf("IamInstanceProfile: %s\n", *instance.IamInstanceProfile)
	}

	if instance.SSHPort != nil {
		s = s + fmt.Sprintf("SSHPort: %d\n", *instance.SSHPort)
	}

	if instance.KeyName != nil {
		s = s + fmt.Sprintf("KeyName: %s\n", *instance.KeyName)
	}

	if instance.Tags != nil {
		s = s + "Tags:\n"
		for key, value := range *instance.Tags {
			s = s + fmt.Sprintf("\t%s: %s\n", key, value)

		}
	}

	if instance.Type != nil {
		s = s + fmt.Sprintf("Type: %s\n", *instance.Type)
	}

	if instance.BidPrice != nil {
		s = s + fmt.Sprintf("BidPrice: %f\n", *instance.BidPrice)
	}

	if instance.SpotPrice != nil {
		s = s + fmt.Sprintf("SpotPrice: %s\n", *instance.SpotPrice)
	}

	if instance.UserData != nil {
		s = s + fmt.Sprintf("UserData: %s\n", *instance.UserData)
	}

	if instance.EntrypointFile != nil {
		s = s + fmt.Sprintf("EntrypointFile: %s\n", *instance.EntrypointFile)
	}

	if instance.WaitOnCloudInit != nil {
		s = s + fmt.Sprintf("WaitOnCloudInit: %t\n", *instance.WaitOnCloudInit)
	}

	if instance.Attach != nil {
		s = s + fmt.Sprintf("Attach: %t\n", *instance.Attach)
	}

	if instance.NoTermination != nil {
		s = s + fmt.Sprintf("NoTermination: %t\n", *instance.NoTermination)
	}

	if instance.TTYColor != nil {
		s = s + fmt.Sprintf("TTYColor: %s\n", *instance.TTYColor)
	}

	if instance.PrivateIPAddress != nil {
		s = s + fmt.Sprintf("PrivateIPAddress: %s\n", *instance.PrivateIPAddress)
	}

	if instance.InstanceID != nil {
		s = s + fmt.Sprintf("InstanceID: %s\n", *instance.InstanceID)
	}

	if instance.ExitCode != nil {
		s = s + fmt.Sprintf("ExitCode: %b\n", *instance.ExitCode)
	}

	if instance.Command != nil {
		s = s + fmt.Sprintf("Command: %s\n", *instance.Command)
	}

	return s
}
