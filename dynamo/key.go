package dynamo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Key struct {
	Hash   string `dynamodbav:"pk"`
	Sort   string `dynamodbav:"sk"`
	Domain string `dynamodbav:"dk"`
	ENames map[string]string
	CExpr  *string
}

func (k *Key) Full() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		HashKeyName: &types.AttributeValueMemberS{Value: k.Hash},
		SortKeyName: &types.AttributeValueMemberS{Value: k.Sort},
	}
}

func (k *Key) FormatForError() string {
	return fmt.Sprintf("pk: %s, sk: %s, domain: %s", k.Hash, k.Sort, k.Domain)
}

func (k *Key) ConditionExpr() *string {
	return k.CExpr
}

func (k *Key) ExprAttrNames() map[string]string {
	return k.ENames
}
