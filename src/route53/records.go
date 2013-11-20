package route53

import (
	"encoding/xml"
	"errors"
	"fmt"
)

// XML RPC types.

type ChangeRRSetRequest struct {
	XMLName xml.Name      `xml:"ChangeResourceRecordSetsRequest"`
	XMLNS   string        `xml:"xmlns,attr"`
	Comment string        `xml:"ChangeBatch>Comment"`
	Changes []RRSetChange `xml:"ChangeBatch>Changes>Change"`
}

type RRSetChange struct {
	Action string
	RRSet  RRSet `xml:"ResourceRecordSet"`
}

type RRSet struct {
	Name          string
	Type          string
	TTL           uint
	Values        []string `xml:"ResourceRecords>ResourceRecord>Value"`
	HealthCheckId string   `xml:",omitempty"`

	// Optional Unique Identifier
	SetIdentifier string `xml:",omitempty"`

	// Weight Syntax
	Weight uint8 `xml:",omitempty"`

	// Alias Syntax
	HostedZoneId         string `xml:"AliasTarget>HostedZoneId,omitempty"`
	DNSName              string `xml:"AliasTarget>DNSName,omitempty"`
	EvaluateTargetHealth bool   `xml:"AliasTarget>EvaluateTargetHealth,omitempty"`

	// Fail Syntax
	Failover string `xml:",omitempty"`

	// Latency Syntax
	Region string `xml:",omitempty"`
}

type ChangeRRSetsResponse struct {
	XMLName    xml.Name `xml:"ChangeResourceRecordSetsResponse"`
	ChangeInfo ChangeInfo
}

type ListRRSetResponse struct {
	XMLName              xml.Name `xml:"ListResourceRecordSetsResponse"`
	RRSets               []RRSet  `xml:"ResourceRecordSets>ResourceRecordSet"`
	IsTruncated          bool
	NextRecordName       string
	NextRecordIdentifier string
	MaxItems             uint
}

// Route53 API requests.

func (r53 *Route53) ChangeRRSet(zoneId string, changes []RRSetChange, comment string) (ChangeInfo, error) {
	xmlReq := &ChangeRRSetRequest{
		XMLNS:   "https://route53.amazonaws.com/doc/2012-12-12/",
		Comment: comment,
		Changes: changes,
	}

	req := request{
		method: "POST",
		path:   fmt.Sprintf("/2012-12-12/hostedzone/%s/rrset", zoneId),
		body:   xmlReq,
	}

	xmlRes := &ChangeRRSetsResponse{}

	if err := r53.run(req, xmlRes); err != nil {
		return ChangeInfo{}, err
	}

	return xmlRes.ChangeInfo, nil
}

func (r53 *Route53) ListRRSets(zoneId string) ([]RRSet, error) {
	req := request{
		method: "GET",
		path:   fmt.Sprintf("/2012-12-12/hostedzone/%s/rrset", zoneId),
	}

	xmlRes := &ListRRSetResponse{}

	if err := r53.run(req, xmlRes); err != nil {
		return []RRSet{}, err
	}
	if xmlRes.IsTruncated {
		return []RRSet{}, errors.New("cannot handle truncated responses")
	}

	return xmlRes.RRSets, nil
}

// Convenience functions on AWS APIs.

func (z *HostedZone) ChangeRRSet(changes []RRSetChange, comment string) (ChangeInfo, error) {
	return z.r53.ChangeRRSet(z.Id, changes, comment)
}

func (z *HostedZone) ListRRSets() ([]RRSet, error) {
	return z.r53.ListRRSets(z.Id)
}

func (z *HostedZone) CreateRRSet(rrset RRSet, comment string) (ChangeInfo, error) {
	change := RRSetChange{
		Action: "CREATE",
		RRSet:  rrset,
	}

	return z.ChangeRRSet([]RRSetChange{change}, comment)
}

func (z *HostedZone) DeleteRRSet(rrset RRSet, comment string) (ChangeInfo, error) {
	change := RRSetChange{
		Action: "DELETE",
		RRSet:  rrset,
	}

	return z.ChangeRRSet([]RRSetChange{change}, comment)
}
