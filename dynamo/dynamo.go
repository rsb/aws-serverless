// Package dynamo implements various design patterns for dynamodb and looks
// to simplify the aws api for aws-sdk-go-v2
package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rsb/failure"
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

type FormatterError interface {
	FormatForError() string
}

type Keyable interface {
	Full() map[string]types.AttributeValue
	ConditionExpr() *string
	ExprAttrNames() map[string]string
	FormatterError
}

type Writable interface {
	ToItem() (map[string]types.AttributeValue, error)
	ToDBKey() map[string]types.AttributeValue
	ConditionExpr() *string
	ExprAttrNames() map[string]string
	FormatterError
}

type Client struct {
	api APIBehavior
	tbl Table
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

func (c *Client) NewPutInput(item map[string]types.AttributeValue, cond ...string) *dynamodb.PutItemInput {
	in := dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(c.TableName()),
	}

	if len(cond) > 0 && cond[0] != "" {
		in.ConditionExpression = aws.String(cond[0])
	}

	return &in
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
