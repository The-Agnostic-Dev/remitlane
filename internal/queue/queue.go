// Package queue enqueues background jobs on Redis (asynq). The handlers live
// in internal/jobs.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Task types.
const (
	TypeDispatch   = "payout:dispatch"
	TypePoll       = "payout:poll"
	TypeDeliver    = "webhook:deliver"
	TypeRecon      = "recon:daily"
	TypeLowBalance = "alerts:low_balance"
)

// Queue names, with weights set in internal/jobs.
const (
	Critical = "critical"
	Default  = "default"
	Low      = "low"
)

// DispatchPayload is the payload of TypeDispatch.
type DispatchPayload struct {
	PayoutID string `json:"payout_id"`
}

// DeliverPayload is the payload of TypeDeliver.
type DeliverPayload struct {
	DeliveryID int64 `json:"delivery_id"`
	Manual     bool  `json:"manual,omitempty"`
}

// ReconPayload is the payload of TypeRecon. An empty date means yesterday.
type ReconPayload struct {
	Date      string `json:"date,omitempty"`
	Triggered string `json:"triggered_by"`
}

// LowBalancePayload is the payload of TypeLowBalance.
type LowBalancePayload struct {
	PartnerID string `json:"partner_id"`
	Day       string `json:"day"`
}

// Enqueuer is what services need from the queue.
type Enqueuer interface {
	Dispatch(ctx context.Context, payoutID string) error
	Deliver(ctx context.Context, deliveryID int64, manual bool) error
	Recon(ctx context.Context, date, triggeredBy string) error
	LowBalance(ctx context.Context, partnerID, day string) error
}

// Options for retries.
type Options struct {
	DispatchRetries int
	WebhookAttempts int
}

// Client enqueues tasks.
type Client struct {
	c    *asynq.Client
	opts Options
}

// NewClient returns a Client.
func NewClient(r asynq.RedisConnOpt, opts Options) *Client {
	return &Client{c: asynq.NewClient(r), opts: opts}
}

// Close closes the Redis connection.
func (c *Client) Close() error { return c.c.Close() }

func (c *Client) enqueue(ctx context.Context, typ string, payload any, opts ...asynq.Option) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.c.EnqueueContext(ctx, asynq.NewTask(typ, b), opts...)
	if errors.Is(err, asynq.ErrTaskIDConflict) || errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("queue: enqueue %s: %w", typ, err)
	}
	return nil
}

// Dispatch sends a payout to its provider.
func (c *Client) Dispatch(ctx context.Context, payoutID string) error {
	return c.enqueue(ctx, TypeDispatch, DispatchPayload{PayoutID: payoutID},
		asynq.Queue(Critical), asynq.MaxRetry(c.opts.DispatchRetries), asynq.TaskID("dispatch:"+payoutID),
		asynq.Timeout(30*time.Second))
}

// Deliver sends one partner webhook. Manual retries from the console get a
// single attempt.
func (c *Client) Deliver(ctx context.Context, deliveryID int64, manual bool) error {
	retries := c.opts.WebhookAttempts - 1
	if manual {
		retries = 0
	}
	return c.enqueue(ctx, TypeDeliver, DeliverPayload{DeliveryID: deliveryID, Manual: manual},
		asynq.Queue(Default), asynq.MaxRetry(retries), asynq.Timeout(20*time.Second))
}

// Recon builds the reconciliation report for a date.
func (c *Client) Recon(ctx context.Context, date, triggeredBy string) error {
	return c.enqueue(ctx, TypeRecon, ReconPayload{Date: date, Triggered: triggeredBy},
		asynq.Queue(Low), asynq.MaxRetry(2), asynq.Unique(time.Minute))
}

// LowBalance checks a partner's balance once per business day.
func (c *Client) LowBalance(ctx context.Context, partnerID, day string) error {
	return c.enqueue(ctx, TypeLowBalance, LowBalancePayload{PartnerID: partnerID, Day: day},
		asynq.Queue(Low), asynq.MaxRetry(3), asynq.Unique(10*time.Minute))
}
