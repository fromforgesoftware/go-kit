package query

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/filter"
)

func FiltersToURLValues(filters Filters[any]) url.Values {
	out := url.Values{}
	addFiltersWithPrefix(out, filters, "filter")
	return out
}

func OrGroupsToURLValues(groups []Filters[any]) url.Values {
	out := url.Values{}
	for i, g := range groups {
		addFiltersWithPrefix(out, g, fmt.Sprintf("filter[or][%d]", i))
	}
	return out
}

func addFiltersWithPrefix(out url.Values, filters Filters[any], prefix string) {
	for fieldName, f := range filters {
		if f == nil {
			continue
		}
		operator := MarshalOperator(f.Operator())
		key := fmt.Sprintf("%s[%s][%s]", prefix, fieldName, operator)
		value := f.Value()
		switch v := value.(type) {
		case nil:
			out.Add(key, "null")
		case []string:
			if len(v) > 0 {
				out.Add(key, strings.Join(v, ","))
			}
		case []interface{}:
			strs := make([]string, 0, len(v))
			for _, item := range v {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
			if len(strs) > 0 {
				out.Add(key, strings.Join(strs, ","))
			}
		default:
			valueType := reflect.TypeOf(value)
			if valueType != nil && valueType.Kind() == reflect.Slice {
				sliceValue := reflect.ValueOf(value)
				strs := make([]string, 0, sliceValue.Len())
				for i := 0; i < sliceValue.Len(); i++ {
					strs = append(strs, fmt.Sprintf("%v", sliceValue.Index(i).Interface()))
				}
				if len(strs) > 0 {
					out.Add(key, strings.Join(strs, ","))
				}
			} else {
				out.Add(key, fmt.Sprintf("%v", value))
			}
		}
	}
}

// AddFilterParam is a convenience function to add a single filter parameter
func AddFilterParam(queryParams url.Values, fieldName string, operator filter.Operator, value interface{}) {
	filters := make(Filters[any])
	filters[fieldName] = filter.NewFieldFilter(operator, fieldName, value)

	// Merge the new filter into existing params
	newParams := FiltersToURLValues(filters)
	for key, values := range newParams {
		for _, v := range values {
			queryParams.Add(key, v)
		}
	}
}

func SortingToURLValue(sp *SortingParams) string {
	if sp == nil {
		return ""
	}
	parts := []string{}
	for _, key := range sp.Keys() {
		if key == "" {
			continue
		}
		if sp.Get(key) == SortDesc {
			parts = append(parts, "-"+key)
		} else {
			parts = append(parts, key)
		}
	}
	return strings.Join(parts, ",")
}

func SparseFieldsetsToURLValues(fields SparseFieldsets) url.Values {
	out := url.Values{}
	for resourceType, names := range fields {
		if resourceType == "" || len(names) == 0 {
			continue
		}
		out.Set(fmt.Sprintf("fields[%s]", resourceType), strings.Join(names, ","))
	}
	return out
}

func GroupToURLValue(group []string) string {
	clean := []string{}
	for _, g := range group {
		if g = strings.TrimSpace(g); g != "" {
			clean = append(clean, g)
		}
	}
	return strings.Join(clean, ",")
}

func AggregationsToURLValues(aggs []Aggregation) url.Values {
	out := url.Values{}
	for _, a := range aggs {
		if a.Alias == "" || !a.Operator.Valid() {
			continue
		}
		field := a.Field
		if field == "" {
			field = "*"
		}
		out.Set(
			fmt.Sprintf("agg[%s][%s]", a.Alias, strings.ToLower(a.Operator.String())),
			field,
		)
	}
	return out
}

var bucketUnits = []struct {
	d    time.Duration
	unit string
}{
	{time.Hour, "h"},
	{time.Minute, "m"},
	{time.Second, "s"},
}

func BucketToURLValue(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	for _, u := range bucketUnits {
		if d%u.d == 0 {
			return fmt.Sprintf("%d%s", d/u.d, u.unit)
		}
	}
	return d.String()
}

func PaginationToURLValues(p *PaginationParams) url.Values {
	out := url.Values{}
	if p == nil {
		return out
	}
	if p.IsCursor() {
		if p.Before != "" {
			out.Set("page[before]", p.Before)
		}
		if p.After != "" {
			out.Set("page[after]", p.After)
		}
		if p.Size > 0 {
			out.Set("page[size]", fmt.Sprintf("%d", p.Size))
		}
		return out
	}
	if p.Limit > 0 {
		out.Set("page[limit]", fmt.Sprintf("%d", p.Limit))
	}
	if p.Offset > 0 {
		out.Set("page[offset]", fmt.Sprintf("%d", p.Offset))
	}
	return out
}
