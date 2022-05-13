// Package dynamo implements various design patterns for dynamodb and looks
// to simplify the aws api for aws-sdk-go-v2
package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/rsb/failure"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	HashKeyName   = "pk"
	SortKeyName   = "sk"
	DomainKeyName = "dk"
)

type APIBehavior interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	BatchGetItem(context.Context, *dynamodb.BatchGetItemInput, ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error)
	BatchWriteItem(context.Context, *dynamodb.BatchWriteItemInput, ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Query(context.Context, *dynamodb.QueryInput, ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	TransactGetItems(context.Context, *dynamodb.TransactGetItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	UpdateTimeToLive(context.Context, *dynamodb.UpdateTimeToLiveInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateTimeToLiveOutput, error)
}

type Keyable interface {
	Full() map[string]types.AttributeValue
	ConditionExpression() *string
	ExpressionAttrNames() map[string]string
	FormatForError() string
}

type Client struct {
	api APIBehavior
	tbl Table
}

func NewClient(api APIBehavior, tbl Table) *Client {
	return &Client{api: api, tbl: tbl}
}

func (c *Client) TableName() string {
	return c.tbl.Name()
}

func (c *Client) NewGetItemIn(k Keyable) *dynamodb.GetItemInput {
	return &dynamodb.GetItemInput{
		TableName: aws.String(c.TableName()),
		Key:       k.Full(),
	}
}

type Key struct {
	Hash          string             `dynamodbav:"pk"`
	Sort          string             `dynamodbav:"sk"`
	Domain        string             `dynamodbav:"dk"`
	ExprNames     map[string]*string `dynamodbav:"-"`
	ConditionExpr *string            `dynamodbav:"-"`
}

func (k *Key) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	av := types.AttributeValueMemberS{}

	return &av, nil
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

func (k *Key) ExpressionAttrNames() map[string]*string {
	return k.ExprNames
}

type Table struct {
	name    string
	hash    string
	sort    string
	indexes map[string]string
}

func NewTable(name, hash, sort string, idx map[string]string) (Table, error) {
	var err *failure.Multi
	var tbl Table
	if name == "" {
		err = failure.Append(err, failure.InvalidParam("name is empty"))
	}

	if hash == "" {
		err = failure.Append(failure.InvalidParam("hash key is empty"))
	}

	if sort == "" {
		err = failure.Append(failure.InvalidParam("sort key is empty"))
	}

	for k, v := range idx {
		if k == "" {
			err = failure.Append(err, failure.InvalidParam("[idx] a key in index map is empty"))
			continue
		}

		if v == "" {
			err = failure.Append(err, failure.InvalidParam("[idx] key (%s) has empty value", k))
		}
	}

	if e := err.ErrorOrNil(); e != nil {
		return tbl, failure.Wrap(e, "There are on or more invalid params")
	}

	tbl = Table{
		name:    name,
		hash:    hash,
		sort:    sort,
		indexes: idx,
	}

	return tbl, nil
}

func (t Table) Name() string {
	return t.name
}
