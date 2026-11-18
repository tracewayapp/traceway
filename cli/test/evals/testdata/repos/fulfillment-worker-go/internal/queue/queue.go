// Package queue consumes a topic through the queue gateway's HTTP API.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Message is one record of a topic.
type Message struct {
	Topic     string          `json:"topic"`
	Partition int             `json:"partition"`
	Offset    int64           `json:"offset"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
}

// Consumer pulls messages for one consumer group.
type Consumer struct {
	base  string
	group string
	topic string
	http  *http.Client
}

// NewConsumer returns a consumer of topic for group.
func NewConsumer(base, group, topic string) *Consumer {
	return &Consumer{base: base, group: group, topic: topic, http: &http.Client{}}
}

// Group is the consumer group.
func (c *Consumer) Group() string { return c.group }

// Fetch long-polls the next message.
func (c *Consumer) Fetch(ctx context.Context) (*Message, error) {
	u := fmt.Sprintf("%s/v1/groups/%s/topics/%s/next", c.base, url.PathEscape(c.group), url.PathEscape(c.topic))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", c.topic, resp.Status)
	}
	var m Message
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Commit acknowledges a message.
func (c *Consumer) Commit(ctx context.Context, m *Message) error {
	u := fmt.Sprintf("%s/v1/groups/%s/topics/%s/partitions/%d/offset/%d", c.base, url.PathEscape(c.group), url.PathEscape(m.Topic), m.Partition, m.Offset+1)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("commit %s/%d@%d: %s", m.Topic, m.Partition, m.Offset, resp.Status)
	}
	return nil
}
