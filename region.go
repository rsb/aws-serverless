package sls

import (
	"fmt"
	"strings"

	"github.com/rsb/failure"
)

const (
	UsEast1      = Region("us-east-1")
	UsEast2      = Region("us-east-2")
	UsWest1      = Region("us-west-1")
	UsWest2      = Region("us-west-2")
	AfSouth1     = Region("af-south-1")
	ApEast1      = Region("ap-east-1")
	ApSouth1     = Region("ap-south-1")
	ApNortheast1 = Region("ap-northeast-1")
	ApNortheast2 = Region("ap-northeast-2")
	ApNortheast3 = Region("ap-northeast-3")
	ApSoutheast1 = Region("ap-southeast-1")
	ApSoutheast2 = Region("ap-southeast-2")
	CaCentral1   = Region("ca-central-1")
	EuCentral1   = Region("eu-central-1")
	EuWest1      = Region("eu-west-1")
	EuWest2      = Region("eu-west-2")
	EuWest3      = Region("eu-west-3")
	EuSouth1     = Region("eu-south-1")
	EuNorth1     = Region("eu-north-1")
	MeSouth1     = Region("me-south-1")
	SaEast1      = Region("sa-east-1")
	UsGovEast1   = Region("us-gov-east-1")
	UsGovWest1   = Region("us-gov-west-1")
)

var DefaultRegion = Region("us-east-1")

// Region represents the AWS region identifier like us-east-1
type Region string

func (r Region) String() string {
	return string(r)
}

func (r Region) IsEmpty() bool {
	return r.String() == ""
}

func (r Region) Code() string {
	return RegionCode(r.String())
}

func ToRegion(region string) (Region, error) {
	var r Region
	var err error

	switch region {
	case UsEast1.String(), UsEast1.Code():
		r = UsEast1
	case UsEast2.String(), UsEast2.Code():
		r = UsEast2
	case UsWest1.String(), UsWest1.Code():
		r = UsWest1
	case UsWest2.String(), UsWest2.Code():
		r = UsWest2
	case AfSouth1.String(), AfSouth1.Code():
		r = AfSouth1
	case ApEast1.String(), ApEast1.Code():
		r = ApEast1
	case ApSouth1.String(), ApSouth1.Code():
		r = ApSouth1
	case ApNortheast1.String(), ApNortheast1.Code():
		r = ApNortheast1
	case ApNortheast2.String(), ApNortheast2.Code():
		r = ApNortheast2
	case ApNortheast3.String(), ApNortheast3.Code():
		r = ApNortheast3
	case ApSoutheast1.String(), ApSoutheast1.Code():
		r = ApSoutheast1
	case ApSoutheast2.String(), ApSoutheast2.Code():
		r = ApSoutheast2
	case CaCentral1.String(), CaCentral1.Code():
		r = CaCentral1
	case EuCentral1.String(), EuCentral1.Code():
		r = EuCentral1
	case EuWest1.String(), EuWest1.Code():
		r = EuWest1
	case EuWest2.String(), EuWest2.Code():
		r = EuWest2
	case EuWest3.String(), EuWest3.Code():
		r = EuWest3
	case EuSouth1.String(), EuSouth1.Code():
		r = EuSouth1
	case EuNorth1.String(), EuNorth1.Code():
		r = EuNorth1
	case MeSouth1.String(), MeSouth1.Code():
		r = MeSouth1
	case SaEast1.String(), SaEast1.Code():
		r = SaEast1
	case UsGovEast1.String(), UsGovEast1.Code():
		r = UsGovEast1
	case UsGovWest1.String(), UsGovWest1.Code():
		r = UsGovWest1

	default:
		err = failure.Validation("aws region (%s) is not mapped", region)
	}

	return r, err
}

// RegionCode takes an AWS region like us-east-1 and compresses into a smaller
// code like (us-east-1) -> (use1) which is used in resource naming
func RegionCode(region string) string {
	parts := strings.Split(region, "-")
	if len(parts) != 3 {
		return ""
	}
	return fmt.Sprintf("%s%s%s", parts[0], string(parts[1][0]), parts[2])
}
