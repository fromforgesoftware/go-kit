package resource

type RelationshipDTO struct {
	RestDTO
}

type RelationshipDTOOpt func(r *RelationshipDTO)

func RelFromIdentifier(id Identifier) RelationshipDTOOpt {
	return func(r *RelationshipDTO) {
		if id == nil {
			return
		}
		RelFromIDAndType(id.ID(), Type(id.Type()))(r)
	}
}

func RelFromIDAndType(id string, kind Type) RelationshipDTOOpt {
	return func(r *RelationshipDTO) {
		r.RestDTO.RID = id
		r.RestDTO.RType = kind
	}
}

func RelationshipToDTO(opts ...RelationshipDTOOpt) *RelationshipDTO {
	r := new(RelationshipDTO)
	for _, opt := range opts {
		opt(r)
	}

	if len(r.ID()) < 1 || len(r.Type()) < 1 {
		return nil
	}

	return r
}
