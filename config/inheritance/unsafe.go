package inheritance

import (
	"reflect"
	"unsafe"
)

// unsafePointer returns the address of v's underlying storage, used to
// write into Optional[T]'s unexported fields. Same package as
// Optional, so the access is structurally safe — the unsafe call is
// only required because reflect treats unexported fields as
// non-settable.
func unsafePointer(v reflect.Value) unsafe.Pointer {
	return unsafe.Pointer(v.UnsafeAddr())
}
