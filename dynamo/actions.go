package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rsb/failure"
)

func (c *Client) Item(ctx context.Context, key Keyable) (map[string]types.AttributeValue, error) {
	in := c.NewGetItemIn(key)

	if names := key.ExprAttrNames(); len(names) > 0 {
		in.ExpressionAttributeNames = names
	}

	out, err := c.api.GetItem(ctx, in)
	if err != nil {
		return nil, failure.ToSystem(err, "c.api.GetItem failed (%+v)", in)
	}

	if len(out.Item) == 0 {
		return nil, failure.NotFound("%s", key.FormatForError())
	}

	return out.Item, nil
}

func (c *Client) Put(ctx context.Context, item map[string]types.AttributeValue) error {
	in := c.NewPutInput(item)

	if _, err := c.api.PutItem(ctx, in); err != nil {
		return failure.ToSystem(err, "c.api.PutItem failed")
	}

	return nil
}

func (c *Client) Write(ctx context.Context, row Writable) error {
	item, err := row.ToItem()
	if err != nil {
		return failure.Wrap(err, "row.ToTime failed")
	}

	in := c.NewPutInput(item)
	if expr := row.ConditionExpr(); err != nil {
		in.ConditionExpression = expr
		if names := row.ExprAttrNames(); len(names) > 0 {
			in.ExpressionAttributeNames = names
		}
	}

	if _, err = c.api.PutItem(ctx, in); err != nil {
		return failure.Wrap(err, "c.api.PutItem failed")
	}

	return nil
}
