package dynamo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Key struct {
	Hash          string `dynamodbav:"pk"`
	Sort          string `dynamodbav:"sk"`
	Domain        string `dynamodbav:"dk"`
	ExprNames     map[string]string
	ConditionExpr *string
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

func (k *Key) ConditionExpression() *string {
	return k.ConditionExpr
}

func (k *Key) ExpressionAttrNames() map[string]string {
	return k.ExprNames
}
