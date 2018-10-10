package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func getNameTag(tags []*ec2.Tag) string {
	for _, tag := range tags {
		key := *tag.Key
		value := *tag.Value
		if key == "Name" {
			return value
		}
	}

	return "unnamed"
}

func printHeader(header string) {
	fmt.Println("\n================================")
	fmt.Println(header)
	fmt.Println("================================")
}

func isCidrBlockPublic(cidrBlockString string) bool {
	privateBlocks := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
	}

	_, cidrBlock, _ := net.ParseCIDR(cidrBlockString)

	for _, privateBlockString := range privateBlocks {
		_, privateBlock, _ := net.ParseCIDR(privateBlockString)

		if privateBlock.Contains(cidrBlock.IP) {
			return false
		}
	}

	return true
}

func main() {
	svc := ec2.New(session.New(), &aws.Config{
		Region: aws.String(endpoints.EuWest1RegionID),
	})

	request := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running")},
			},
		},
	}

	result, err := svc.DescribeInstances(request)

	if err != nil {
		fmt.Println("Error", err)
	}

	instancesWithPublicIP := []*ec2.Instance{}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			nameTag := getNameTag(instance.Tags)

			if instance.PublicIpAddress == nil {
				fmt.Println(nameTag, "has no public IP")
				continue
			}

			instancesWithPublicIP = append(instancesWithPublicIP, instance)
		}
	}

	printHeader("Instances With Public IPs")

	ipsWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, instance := range instancesWithPublicIP {
		nameTag := getNameTag(instance.Tags)

		instanceID := *instance.InstanceId
		ip := *instance.PublicIpAddress

		fmt.Fprintln(ipsWriter, nameTag, "\t", instanceID, "\t", ip)
	}
	ipsWriter.Flush()

	printHeader("Open Inbound Routes")

	routesWriter := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, instance := range instancesWithPublicIP {
		nameTag := getNameTag(instance.Tags)
		ip := *instance.PublicIpAddress

		for _, sgID := range instance.SecurityGroups {

			input := &ec2.DescribeSecurityGroupsInput{
				GroupIds: []*string{
					sgID.GroupId,
				},
			}

			result, err := svc.DescribeSecurityGroups(input)

			if err != nil {
				fmt.Println("Error", err)
			}

			for _, sg := range result.SecurityGroups {
				for _, ingress := range sg.IpPermissions {

					portRangeString := "n/a"
					if ingress.FromPort != nil {
						fromString := strconv.FormatInt(*ingress.FromPort, 10)
						toString := strconv.FormatInt(*ingress.ToPort, 10)
						if fromString == "-1" {
							portRangeString = "*"
						} else if fromString == toString {
							portRangeString = fromString
						} else {
							portRangeString = fromString + " - " + toString
						}
					}

					for _, ipRange := range ingress.IpRanges {

						rangeDescriptionString := "no description"
						if ipRange.Description != nil {
							rangeDescriptionString = *ipRange.Description
						}

						cidr := *ipRange.CidrIp
						if isCidrBlockPublic(cidr) {
							fmt.Fprintln(routesWriter,
								nameTag, "\t",
								ip, "\t",
								portRangeString, "\t",
								cidr, "\t",
								rangeDescriptionString, "\t",
							)
						}
					}
				}
			}
		}
	}

	routesWriter.Flush()
}
