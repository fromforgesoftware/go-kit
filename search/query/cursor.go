package query

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/fromforgesoftware/go-kit/resource"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Cursor interface {
	UUID() string
	CreatedAt() time.Time
	Limit() int
}

type cursor struct {
	createdAt time.Time
	uuid      string
	limit     int
}

func NewCursor(createdAt time.Time, uuid string, limit int) Cursor {
	if uuid == "" || createdAt.IsZero() {
		return nil
	}

	return &cursor{
		uuid:      uuid,
		createdAt: createdAt,
		limit:     limit,
	}
}

func (c *cursor) UUID() string {
	return c.uuid
}

func (c *cursor) CreatedAt() time.Time {
	return c.createdAt
}

func (c *cursor) Limit() int {
	return c.limit
}

const cursorPayloadSep = "|"

// EncodePositionCursor builds the opaque page[after] / page[before]
// value: base64(`<RFC3339Nano>|<uuid>`). Position only — filters and
// sort travel as their own query params, never inside the cursor.
func EncodePositionCursor(createdAt time.Time, id string) string {
	if id == "" || createdAt.IsZero() {
		return ""
	}
	payload := createdAt.UTC().Format(time.RFC3339Nano) + cursorPayloadSep + id
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

// DecodePositionCursor reverses EncodePositionCursor. Returns the
// zero values when the cursor is empty or malformed; callers should
// treat a zero return as "no cursor".
func DecodePositionCursor(encoded string) (time.Time, string) {
	if encoded == "" {
		return time.Time{}, ""
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return time.Time{}, ""
	}
	parts := strings.SplitN(string(raw), cursorPayloadSep, 2)
	if len(parts) != 2 {
		return time.Time{}, ""
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, ""
	}
	return ts, parts[1]
}

// FormatNextURL builds the JSON:API §Pagination `links.next` URL for
// the supplied result page. It preserves every filter / sort /
// include / fields[] param that came in on req and adds (or replaces)
// `page[after]=<opaqueCursor>` plus `page[size]=<limit>`.
//
// Returns "" when the result set is empty, when the request was a
// fetch-by-id (no pagination needed), or when the gRPC metadata
// doesn't contain the forwarded host / path.
func FormatNextURL[R resource.Resource](ctx context.Context, r []R, req protoreflect.ProtoMessage) string {
	if len(r) == 0 {
		return ""
	}

	keep, hasID, limit := paramsFromProto(req)
	if hasID {
		return ""
	}

	md, _ := metadata.FromIncomingContext(ctx)
	host := mdSingle(md, "x-forwarded-host")
	path := mdSingle(md, "x-forwarded-path")
	if host == "" || path == "" {
		return ""
	}

	last := r[len(r)-1]
	after := EncodePositionCursor(last.CreatedAt(), last.ID())

	pairs := make([]string, 0, len(keep)+2)
	for _, p := range keep {
		pairs = append(pairs, p)
	}
	pairs = append(pairs, "page[after]="+after)
	if limit > 0 {
		pairs = append(pairs, fmt.Sprintf("page[size]=%d", limit))
	}

	return fmt.Sprintf("http://%s%s?%s", host, path, strings.Join(pairs, "&"))
}

// paramsFromProto walks the request message, returning the
// query-string fragments to preserve in the next-page URL plus a flag
// for "this is a fetch-by-id, no pagination". Pagination fields
// themselves are stripped (we'll rewrite them).
func paramsFromProto(req protoreflect.ProtoMessage) (keep []string, hasID bool, limit int) {
	if req == nil {
		return nil, false, 0
	}
	rft := req.ProtoReflect()
	rft.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := fd.TextName()
		if name == "id" {
			hasID = true
			return false
		}
		if name == "page" {
			if v.Message().IsValid() {
				size := v.Message().Get(fd.Message().Fields().ByName("size"))
				if size.IsValid() {
					limit = int(size.Int())
				}
			}
			return true
		}
		if fd.IsMap() {
			v.Map().Range(func(mk protoreflect.MapKey, mv protoreflect.Value) bool {
				keep = append(keep, fmt.Sprintf("%s[%s]=%s", name, mk.String(), mv.String()))
				return true
			})
			return true
		}
		keep = append(keep, fmt.Sprintf("%s=%s", name, v.String()))
		return true
	})
	return keep, hasID, limit
}

func mdSingle(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
