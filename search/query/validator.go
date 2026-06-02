package query

import (
	"fmt"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/filter"
)

type ValidationFunc func(f filter.FieldFilter[any]) error

type ValidationOpt func(c *validator) error

type validator struct {
	mandatoryFields []string
	optionalFields  []string
	groupedFields   [][]string
	sortFields      []string
	groupFields     []string // allowlist for q.Group() dimensions
	aggFields       []string // allowlist for q.Aggregations() source fields
	filterValFuncs  map[string][]ValidationFunc
	mustHaveFilters bool
}

func GroupedFilters(fs ...string) ValidationOpt {
	const (
		minGroupedFilters = 2
	)
	return func(c *validator) error {
		if len(fs) < minGroupedFilters {
			return fmt.Errorf("grouped filters must have at least two fields")
		}
		c.groupedFields = append(c.groupedFields, fs)

		return nil
	}
}

func MandatoryFilters(fs ...string) ValidationOpt {
	return func(c *validator) error {
		c.mandatoryFields = append(c.mandatoryFields, fs...)

		return nil
	}
}

func OptionalFilters(fs ...string) ValidationOpt {
	return func(c *validator) error {
		c.optionalFields = append(c.optionalFields, fs...)

		return nil
	}
}

func SortFields(fs ...string) ValidationOpt {
	return func(c *validator) error {
		c.sortFields = append(c.sortFields, fs...)

		return nil
	}
}

func AtLeastOneFilter() ValidationOpt {
	return func(c *validator) error {
		c.mustHaveFilters = true

		return nil
	}
}

func GroupFields(fs ...string) ValidationOpt {
	return func(c *validator) error {
		c.groupFields = append(c.groupFields, fs...)
		return nil
	}
}

func AggregationFields(fs ...string) ValidationOpt {
	return func(c *validator) error {
		c.aggFields = append(c.aggFields, fs...)
		return nil
	}
}

func ValidFilter(field string, fs ...ValidationFunc) ValidationOpt {
	return func(c *validator) error {
		if c.filterValFuncs[field] == nil {
			c.filterValFuncs[field] = []ValidationFunc{}
		}
		c.filterValFuncs[field] = append(c.filterValFuncs[field], fs...)

		return nil
	}
}

func (v *validator) validate(q Query) error {
	if q == nil {
		return errors.InvalidArgument("query cannot be nil")
	}
	err := v.validateMustHaveFilters(q)
	if err != nil {
		return err
	}
	err = v.validateMandatoryFieldsExist(q)
	if err != nil {
		return err
	}
	err = v.validateGroupedFields(q)
	if err != nil {
		return err
	}
	err = v.validateFieldsAllowed(q)
	if err != nil {
		return err
	}
	err = v.validateFieldFilters(q)
	if err != nil {
		return err
	}
	err = v.validateSortFieldsDenied(q)
	if err != nil {
		return err
	}
	err = v.validateGroup(q)
	if err != nil {
		return err
	}
	err = v.validateAggregations(q)
	if err != nil {
		return err
	}
	return nil
}

func (v *validator) validateGroup(q Query) error {
	dims := q.Group()
	if len(dims) == 0 {
		return nil
	}
	for _, d := range dims {
		found := false
		for _, allowed := range v.groupFields {
			if allowed == d {
				found = true
				break
			}
		}
		if !found {
			return errors.InvalidArgument(fmt.Sprintf("group by %s is not allowed", d))
		}
	}
	return nil
}

func (v *validator) validateAggregations(q Query) error {
	aggs := q.Aggregations()
	if len(aggs) == 0 {
		return nil
	}
	seenAliases := make(map[string]bool, len(aggs))
	for _, a := range aggs {
		if a.Alias == "" {
			return errors.InvalidArgument("aggregation alias cannot be empty")
		}
		if seenAliases[a.Alias] {
			return errors.InvalidArgument(fmt.Sprintf("duplicate aggregation alias: %s", a.Alias))
		}
		seenAliases[a.Alias] = true
		if !a.Operator.Valid() {
			return errors.InvalidArgument(fmt.Sprintf("invalid aggregation operator for %s", a.Alias))
		}
		if a.Field == "*" {
			continue
		}
		found := false
		for _, allowed := range v.aggFields {
			if allowed == a.Field {
				found = true
				break
			}
		}
		if !found {
			return errors.InvalidArgument(fmt.Sprintf("aggregation on %s is not allowed", a.Field))
		}
	}
	return nil
}

func (v *validator) validateSortFieldsDenied(q Query) error {
	for _, key := range q.Sorting().Keys() {
		found := false
		for _, allowed := range v.sortFields {
			if allowed == key {
				found = true
				break
			}
		}
		if !found {
			return errors.InvalidArgument(fmt.Sprintf("sorting by %s is not allowed", key))
		}
	}

	return nil
}

func (v *validator) validateFieldFilters(q Query) error {
	for fName, fs := range v.filterValFuncs {
		found := q.Filters().Get(fName)
		if found == nil {
			continue
		}
		for _, f := range fs {
			err := f(found)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (v *validator) validateGroupedFields(q Query) error {
	qf := q.Filters()
	for _, group := range v.groupedFields {
		anyExist := false
		keysExists := make(map[string]bool)
		for _, k := range group {
			keysExists[k] = qf.Exists(k)
			if qf.Exists(k) {
				anyExist = true
			}
		}
		if !anyExist {
			return nil
		}
		for _, exist := range keysExists {
			if !exist {
				return errors.InvalidArgument(fmt.Sprintf("filters: %+v must go together", group))
			}
		}
	}

	return nil
}

func (v *validator) validateMandatoryFieldsExist(q Query) error {
	for _, field := range v.mandatoryFields {
		if !q.Filters().Exists(field) {
			return errors.InvalidArgument(fmt.Sprintf("missing mandatory filter: %s", field))
		}
	}

	return nil
}

func (v *validator) validateInMandatory(field string) error {
	for _, mandatory := range v.mandatoryFields {
		if mandatory == field {
			return nil
		}
	}

	return errors.InvalidArgument(fmt.Sprintf("filter by %s is not allowed", field))
}

func (v *validator) validateInOptional(field string) error {
	for _, opt := range v.optionalFields {
		if opt == field {
			return nil
		}
	}

	return errors.InvalidArgument(fmt.Sprintf("filter by %s is not allowed", field))
}

func (v *validator) validateFieldsAllowed(q Query) error {
	for _, field := range q.Filters() {
		if err := v.checkFieldAllowed(field.Name()); err != nil {
			return err
		}
	}
	for _, group := range q.OrGroups() {
		for _, field := range group {
			if err := v.checkFieldAllowed(field.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *validator) checkFieldAllowed(field string) error {
	if v.validateInMandatory(field) == nil {
		return nil
	}
	return v.validateInOptional(field)
}

func (v *validator) validateMustHaveFilters(q Query) error {
	if len(v.mandatoryFields) > 0 || v.mustHaveFilters {
		if len(q.Filters()) < 1 {
			return errors.InvalidArgument("at least one filter must be provided")
		}
	}

	return nil
}

func Validate(q Query, opts ...ValidationOpt) error {
	v := &validator{
		mandatoryFields: []string{},
		optionalFields:  []string{},
		sortFields:      []string{},
		filterValFuncs:  make(map[string][]ValidationFunc),
	}
	for _, opt := range opts {
		err := opt(v)
		if err != nil {
			return err
		}
	}

	return v.validate(q)
}
