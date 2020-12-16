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
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/rds"
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

func createAWSClients() (ec2.EC2, elb.ELB, elbv2.ELBV2, rds.RDS) {
	session := session.New()
	config := &aws.Config{
		Region: aws.String(endpoints.EuWest1RegionID),
	}

	ec2Svc := ec2.New(session, config)
	elbSvc := elb.New(session, config)
	elbV2Svc := elbv2.New(session, config)
	rdsSvc := rds.New(session, config)

	return *ec2Svc, *elbSvc, *elbV2Svc, *rdsSvc
}

func fetchInstancesWithPublicIPs(ec2Svc ec2.EC2, region string) []*ec2.Instance {

	request := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running")},
			},
		},
	}

	result, err := ec2Svc.DescribeInstances(request)

	if err != nil {
		fmt.Println("Error", err)
	}

	instancesWithPublicIP := []*ec2.Instance{}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			if instance.PublicIpAddress == nil {
				continue
			}

			instancesWithPublicIP = append(instancesWithPublicIP, instance)
		}
	}

	return instancesWithPublicIP
}

func printInstancesWithPublicIPs(instancesWithPublicIP []*ec2.Instance) {
	printHeader("Instances With Public IPs")

	ipsWriter := newTabWriter()
	for _, instance := range instancesWithPublicIP {
		nameTag := getNameTag(instance.Tags)

		instanceID := *instance.InstanceId
		ip := *instance.PublicIpAddress

		fmt.Fprintln(ipsWriter, nameTag, "\t", instanceID, "\t", ip)
	}
	ipsWriter.Flush()
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
}

type IngressRule struct {
	Cidr                 string
	PortRange            string
	PortRangeDescription string
}

func getSecurityGroupPublicIngressRules(sg ec2.SecurityGroup) []IngressRule {
	ingressRules := []IngressRule{}

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
				rule := IngressRule{
					Cidr:                 cidr,
					PortRange:            portRangeString,
					PortRangeDescription: rangeDescriptionString,
				}
				ingressRules = append(ingressRules, rule)
			}
		}
	}

	return ingressRules
}

func getSecurityGroupByID(ec2Svc ec2.EC2, id string) ec2.SecurityGroup {
	input := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{
			&id,
		},
	}

	result, err := ec2Svc.DescribeSecurityGroups(input)

	if err != nil {
		fmt.Println("Error", err)
	}

	return *result.SecurityGroups[0]
}

func printInstanceInboundRoutes(ec2Svc ec2.EC2, instancesWithPublicIP []*ec2.Instance) {
	printHeader("Open Inbound Instance Routes")

	routesWriter := newTabWriter()
	for _, instance := range instancesWithPublicIP {
		nameTag := getNameTag(instance.Tags)
		ip := *instance.PublicIpAddress

		for _, sgID := range instance.SecurityGroups {

			securityGroup := getSecurityGroupByID(ec2Svc, *sgID.GroupId)

			for _, rule := range getSecurityGroupPublicIngressRules(securityGroup) {
				fmt.Fprintln(routesWriter,
					nameTag, "\t",
					ip, "\t",
					rule.PortRange, "\t",
					rule.Cidr, "\t",
					rule.PortRangeDescription, "\t",
				)
			}
		}
	}

	routesWriter.Flush()
}

func fetchALBs(elbSvc elbv2.ELBV2, region string) []*elbv2.LoadBalancer {
	result, err := elbSvc.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
		PageSize: aws.Int64(400),
	})

	if err != nil {
		fmt.Println("Error", err)
	}

	return result.LoadBalancers
}

func fetchELBs(elbSvc elb.ELB, region string) []*elb.LoadBalancerDescription {
	result, err := elbSvc.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
		PageSize: aws.Int64(400),
	})

	if err != nil {
		fmt.Println("Error", err)
	}

	return result.LoadBalancerDescriptions
}

func printALBInboundRoutes(ec2Svc ec2.EC2, loadBalancers []*elbv2.LoadBalancer) {
	printHeader("Public ALBs (and NLBs)")

	writer := newTabWriter()

	for _, lb := range loadBalancers {

		if *lb.Scheme == "internal" {
			continue
		}

		for _, sgID := range lb.SecurityGroups {

			securityGroup := getSecurityGroupByID(ec2Svc, *sgID)

			for _, ingressRule := range getSecurityGroupPublicIngressRules(securityGroup) {
				fmt.Fprintln(writer,
					*lb.LoadBalancerName, "\t",
					*lb.DNSName, "\t",
					ingressRule.Cidr, "\t",
					ingressRule.PortRange, "\t",
					ingressRule.PortRangeDescription, "\t",
				)
			}
		}

	}

	writer.Flush()
}

func printELBInboundRoutes(ec2Svc ec2.EC2, loadBalancers []*elb.LoadBalancerDescription) {
	printHeader("Public ELBs")

	writer := newTabWriter()

	for _, lb := range loadBalancers {

		if *lb.Scheme == "internal" {
			continue
		}

		for _, sgID := range lb.SecurityGroups {

			securityGroup := getSecurityGroupByID(ec2Svc, *sgID)

			for _, ingressRule := range getSecurityGroupPublicIngressRules(securityGroup) {
				fmt.Fprintln(writer,
					*lb.LoadBalancerName, "\t",
					*lb.DNSName, "\t",
					ingressRule.Cidr, "\t",
					ingressRule.PortRange, "\t",
					ingressRule.PortRangeDescription, "\t",
				)
			}
		}

	}

	writer.Flush()
}

func printRDSInboundRoutes(ec2Svc ec2.EC2, instances []*rds.DBInstance) {
	printHeader("Public RDS Instances")

	writer := newTabWriter()

	for _, db := range instances {

		if !*db.PubliclyAccessible {
			continue
		}

		for _, sgID := range db.VpcSecurityGroups {

			securityGroup := getSecurityGroupByID(ec2Svc, *sgID.VpcSecurityGroupId)

			for _, ingressRule := range getSecurityGroupPublicIngressRules(securityGroup) {
				fmt.Fprintln(writer,
					*db.DBInstanceIdentifier, "\t",
					*db.Endpoint, "\t",
					ingressRule.Cidr, "\t",
					ingressRule.PortRange, "\t",
					ingressRule.PortRangeDescription, "\t",
				)
			}
		}

	}

	writer.Flush()
}

func fetchRDSInstances(rdsSvc rds.RDS, region string) []*rds.DBInstance {
	result, err := rdsSvc.DescribeDBInstances(nil)

	if err != nil {
		fmt.Println("Error", err)
	}

	return result.DBInstances
}

func main() {
	ec2Svc, elbSvc, elbV2Svc, rdsSvc := createAWSClients()

	instancesWithPublicIP := fetchInstancesWithPublicIPs(ec2Svc, endpoints.EuWest1RegionID)
	printInstancesWithPublicIPs(instancesWithPublicIP)
	printInstanceInboundRoutes(ec2Svc, instancesWithPublicIP)

	albs := fetchALBs(elbV2Svc, endpoints.EuWest1RegionID)
	printALBInboundRoutes(ec2Svc, albs)

	elbs := fetchELBs(elbSvc, endpoints.EuWest1RegionID)
	printELBInboundRoutes(ec2Svc, elbs)

	rdsInstances := fetchRDSInstances(rdsSvc, endpoints.EuWest1RegionID)
	printRDSInboundRoutes(ec2Svc, rdsInstances)
}
