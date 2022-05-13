package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rsb/failure"
)

func (c *Client) Item(ctx context.Context, key Keyable) (map[string]types.AttributeValue, error) {
	in := c.NewGetItemIn(key)

	if names := key.ExpressionAttrNames(); len(names) > 0 {
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
