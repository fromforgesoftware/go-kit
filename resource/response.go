package resource

type ListResponse[T any] interface {
	Results() []T
	TotalCount() int
}

type listRes[T any] struct {
	items []T
	count int
}

func (lr *listRes[T]) Results() []T {
	return lr.items
}

func (lr *listRes[T]) TotalCount() int {
	return lr.count
}

func NewEmptyListResponse[T any]() *listRes[T] {
	return &listRes[T]{
		items: []T{},
		count: 0,
	}
}

func NewListResponse[T any](items []T, count int) *listRes[T] {
	return &listRes[T]{
		items: items,
		count: count,
	}
}

type (
	ListResponseDTO[DTO any] struct {
		RResults   []DTO          `json:"results"`
		Pagination *PaginationDTO `json:"pagination"`
	}

	PaginationDTO struct {
		TotalCount int `json:"totalCount"`
	}
)

func ListResponseToDTO[DTO any, R any](
	resMapper func(R) DTO,
) func(res ListResponse[R]) *ListResponseDTO[DTO] {
	return func(coll ListResponse[R]) *ListResponseDTO[DTO] {
		var items []R
		var count int
		if coll != nil {
			items = coll.Results()
			count = coll.TotalCount()
		}

		dtos := make([]DTO, len(items))
		for i, item := range items {
			dtos[i] = resMapper(item)
		}

		return &ListResponseDTO[DTO]{
			RResults: dtos,
			Pagination: &PaginationDTO{
				TotalCount: count,
			},
		}
	}
}

// Results returns the list of DTOs, implementing jsonapi.ListResponse[DTO]
func (l *ListResponseDTO[DTO]) Results() []DTO {
	return l.RResults
}
