package saga

import (
	"context"

	"github.com/fromforgesoftware/go-kit/retry"
)

type step[T any] struct {
	step         StepRunner[T]
	compensation StepRunner[T]
}

type Node[T any] struct {
	self      step[T]
	condition func(T) bool
	childs    []Node[T]
}

type nodeOption[T any] func(Node[T]) Node[T]

func WithStep[T any](step StepRunner[T]) nodeOption[T] {
	return func(n Node[T]) Node[T] {
		n.self.step = step
		return n
	}
}

func WithCompensation[T any](compensation StepRunner[T]) nodeOption[T] {
	return func(n Node[T]) Node[T] {
		n.self.compensation = compensation
		return n
	}
}

func WithCondition[T any](condition func(T) bool) nodeOption[T] {
	return func(n Node[T]) Node[T] {
		n.condition = condition
		return n
	}
}

func WithChilds[T any](childs ...Node[T]) nodeOption[T] {
	return func(n Node[T]) Node[T] {
		n.childs = childs
		return n
	}
}

func NewNode[T any](opts ...nodeOption[T]) (n Node[T]) {
	for _, opt := range opts {
		n = opt(n)
	}
	return
}

func (n Node[T]) Run(ctx context.Context, req T) (T, error) {
	return n.self.step(ctx, req)
}

func (n Node[T]) Compensate(ctx context.Context, req T) (T, error) {
	return n.self.compensation(ctx, req)
}

type tree[T any] Node[T]

func New[T any](nodes ...Node[T]) tree[T] {
	return tree[T]{
		childs: nodes,
	}
}

// Run executes the steps provided in sequential order and executes the compensation steps if one of the main steps returns an error
func (t tree[T]) Run(ctx context.Context, req T) (T, error) {
	var roll []StepRunner[T]
	var err error

	req, roll, err = traverseNode(ctx, Node[T](t), req, roll)
	if err != nil {
		return compensate(ctx, req, roll), err
	}

	return req, nil
}

func traverseNode[T any](ctx context.Context, node Node[T], req T, roll []StepRunner[T]) (T, []StepRunner[T], error) {
	if node.condition != nil && !node.condition(req) { // if condition is not met, skip the node
		return req, roll, nil
	}

	var err error
	if len(node.childs) == 0 { // leaf node
		req, err = node.Run(ctx, req)
		if err == nil && node.self.compensation != nil {
			roll = append(roll, node.self.compensation)
		}
		return req, roll, err
	}

	for _, child := range node.childs { // inode
		req, roll, err = traverseNode(ctx, child, req, roll)
		if err != nil {
			return req, roll, err
		}
	}
	return req, roll, err
}

func compensate[T any](ctx context.Context, req T, steps []StepRunner[T]) T {
	for i := len(steps) - 1; i > -1; i-- {
		// execute compensation step with exponential backoff
		// we ignore the error here because we can't do anything about it
		// and we want to continue compensating the other steps
		req, _ = retry.RetryWithDataAndContext(ctx, func() (T, error) {
			return steps[i](ctx, req)
		}, retry.WithExponentialPolicy())
	}
	return req
}
