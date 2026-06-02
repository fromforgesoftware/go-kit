package helpers

func Map[I any, O any](input []I, transform func(I) O) []O {
	o := make([]O, len(input))
	if len(input) < 1 {
		return o
	}
	for i, e := range input {
		o[i] = transform(e)
	}
	return o
}
