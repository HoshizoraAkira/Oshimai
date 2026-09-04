// Package cloudverify answers one narrow question a DNS/well-known challenge can't: "does this IP
// address actually belong to a cloud account the operator controls?" — a second, independent line
// of evidence for target-ownership verification (see pkg/verify), useful when an operator can't
// modify DNS for a target (e.g. a shared corporate domain) but does hold IAM credentials for the
// AWS account the target runs in.
package cloudverify

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AWSChecker confirms whether a public IP is allocated within one AWS account, using the EC2
// Query API directly (no AWS SDK — see sigv4.go for why).
type AWSChecker struct {
	AccessKey  string
	SecretKey  string
	Region     string
	httpClient *http.Client
	endpoint   string // Overridable in tests; defaults to the real regional EC2 endpoint.
}

// NewAWSChecker builds a checker for the given credentials/region.
func NewAWSChecker(accessKey, secretKey, region string) *AWSChecker {
	if region == "" {
		region = "us-east-1"
	}
	return &AWSChecker{
		AccessKey: accessKey, SecretKey: secretKey, Region: region,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		endpoint:   fmt.Sprintf("https://ec2.%s.amazonaws.com/", region),
	}
}

type describeAddressesResponse struct {
	XMLName      xml.Name `xml:"DescribeAddressesResponse"`
	AddressesSet struct {
		Items []struct {
			PublicIP     string `xml:"publicIp"`
			InstanceID   string `xml:"instanceId"`
			AllocationID string `xml:"allocationId"`
		} `xml:"item"`
	} `xml:"addressesSet"`
}

type errorResponse struct {
	XMLName xml.Name `xml:"Response"`
	Errors  struct {
		Error struct {
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Error"`
	} `xml:"Errors"`
}

// OwnsIP reports whether ip is allocated as an Elastic IP within this checker's AWS account. A
// false result means "not found as an Elastic IP in this account" — it does NOT mean the IP is
// unowned by anyone; ephemeral (non-Elastic) public IPs on EC2/ELB aren't visible to this specific
// check, a limitation documented here rather than silently glossed over.
func (c *AWSChecker) OwnsIP(ctx context.Context, ip string) (owns bool, resource string, err error) {
	endpoint := c.endpoint
	form := url.Values{}
	form.Set("Action", "DescribeAddresses")
	form.Set("Version", "2016-11-15")
	form.Set("Filter.1.Name", "public-ip")
	form.Set("Filter.1.Value.1", ip)
	body := []byte(form.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

	if err := signAWSRequestV4(req, body, c.AccessKey, c.SecretKey, c.Region, "ec2", time.Now()); err != nil {
		return false, "", fmt.Errorf("failed to sign AWS request: %w", err)
	}
	// signAWSRequestV4 sets the Host header on the request struct directly, which net/http
	// requires to be set via req.Host (not req.Header) to actually take effect on the wire.
	req.Host = req.URL.Host

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("EC2 API request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return false, "", err
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr errorResponse
		if xml.Unmarshal(data, &apiErr) == nil && apiErr.Errors.Error.Message != "" {
			return false, "", fmt.Errorf("EC2 API error (%s): %s", apiErr.Errors.Error.Code, apiErr.Errors.Error.Message)
		}
		return false, "", fmt.Errorf("EC2 API returned status %d", resp.StatusCode)
	}

	var parsed describeAddressesResponse
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return false, "", fmt.Errorf("failed to parse EC2 API response: %w", err)
	}

	for _, item := range parsed.AddressesSet.Items {
		if item.PublicIP == ip {
			id := item.InstanceID
			if id == "" {
				id = item.AllocationID
			}
			return true, fmt.Sprintf("Elastic IP allocated to %s in %s", id, c.Region), nil
		}
	}
	return false, "", nil
}
