// Package saga implements the Saga pattern for distributed transactions.
//
// It allows defining a sequence of steps where each step has a corresponding compensation.
// If any step fails, the compensations of all executed steps are triggered in reverse order.
// It supports tree-structured sagas with conditional execution and retries for compensation.
package saga
