package repository

type DeleteType int

const (
	DeleteTypeSoft DeleteType = iota
	DeleteTypeHard
)
