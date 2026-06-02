package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	kiterrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/resource"
)

// File upload + download don't fit the JSON:API wire format — they
// move raw bytes — but they belong here because every service that
// exposes one will want consistent kit-side plumbing for multipart
// parsing, range requests, and error handling.

// MultipartFile holds the file part extracted from a multipart/form-data
// request. The reader is owned by the multipart.Reader; the handler
// closes it after the usecase returns.
type MultipartFile struct {
	FileName    string
	ContentType string
	Size        int64 // -1 when the multipart part doesn't declare a length
	Reader      io.Reader
}

// NewMultipartUploadHandler wires `POST /resources` whose body is a
// multipart/form-data document with one file part plus an arbitrary
// number of metadata fields. The decoder receives the parsed file +
// the *http.Request (path params, form values via req.FormValue) and
// returns a typed Command.
//
//	func decodeDocumentUpload(file rest.MultipartFile, req *http.Request) (UploadDocumentCommand, error) {
//	    return UploadDocumentCommand{
//	        WorkspaceID: req.PathValue("workspaceId"),
//	        FileName:    file.FileName,
//	        ContentType: file.ContentType,
//	    }, nil
//	}
//
//	r.Post("/workspaces/{workspaceId}/documents",
//	    kitrest.NewMultipartUploadHandler(
//	        uc.Upload, decodeDocumentUpload, api.DocumentToDTO))
//
// The cmdFn receives the file reader alongside the Command; it
// streams bytes directly to storage without buffering. fileField is
// the form-field name for the file part (default "file" if empty).
func NewMultipartUploadHandler[Cmd any, R resource.Resource, DTO resource.Resource](
	cmdFn func(ctx context.Context, file io.Reader, cmd Cmd) (R, error),
	decoder func(file MultipartFile, req *http.Request) (Cmd, error),
	encoder func(R) DTO,
	opts ...UploadHandlerOpt,
) http.Handler {
	cfg := defaultUploadConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		file, cmd, cleanup, err := parseMultipartUpload(req, cfg, decoder)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			JsonApiErrorEncoder(ctx, err, w)
			return
		}
		out, err := cmdFn(ctx, file.Reader, cmd)
		if err != nil {
			JsonApiErrorEncoder(ctx, err, w)
			return
		}
		_ = writeBuffered(w, "application/vnd.api+json; charset=utf-8", http.StatusCreated, func(buf io.Writer) error {
			return jsonapiMarshalSingle(buf, encoder(out))
		})
	})
}

// UploadHandlerOpt configures NewMultipartUploadHandler. Options
// cover the field name + the multipart memory cap.
type UploadHandlerOpt func(*uploadConfig)

type uploadConfig struct {
	fileField string
	// maxMemory caps the in-memory portion of multipart parsing; bytes
	// beyond this are spooled to /tmp. Files larger than this are
	// streamed directly so memory stays bounded.
	maxMemory int64
}

func defaultUploadConfig() *uploadConfig {
	return &uploadConfig{
		fileField: "file",
		maxMemory: 8 << 20, // 8 MiB
	}
}

// UploadWithFileField overrides the form-field name for the file
// part (default "file"). Useful when integrating with a client that
// uses a non-standard field name.
func UploadWithFileField(name string) UploadHandlerOpt {
	return func(c *uploadConfig) { c.fileField = name }
}

// UploadWithMaxMemory caps the in-memory portion of the multipart
// parser. Set higher for endpoints that accept many small fields,
// lower to push spooling to /tmp sooner.
func UploadWithMaxMemory(bytes int64) UploadHandlerOpt {
	return func(c *uploadConfig) { c.maxMemory = bytes }
}

func parseMultipartUpload[Cmd any](
	req *http.Request,
	cfg *uploadConfig,
	decoder func(MultipartFile, *http.Request) (Cmd, error),
) (MultipartFile, Cmd, func(), error) {
	var zeroCmd Cmd

	if err := req.ParseMultipartForm(cfg.maxMemory); err != nil {
		return MultipartFile{}, zeroCmd, nil, kiterrors.InvalidArgument("invalid multipart body")
	}

	cleanup := func() {
		if req.MultipartForm != nil {
			_ = req.MultipartForm.RemoveAll()
		}
	}

	fhSlice := req.MultipartForm.File[cfg.fileField]
	if len(fhSlice) == 0 {
		return MultipartFile{}, zeroCmd, cleanup,
			kiterrors.MissingField(cfg.fileField)
	}
	fh := fhSlice[0]
	r, err := fh.Open()
	if err != nil {
		return MultipartFile{}, zeroCmd, cleanup,
			kiterrors.InvalidArgument("file part not readable")
	}

	file := MultipartFile{
		FileName:    fh.Filename,
		ContentType: fh.Header.Get("Content-Type"),
		Size:        fh.Size,
		Reader:      r,
	}

	cmd, err := decoder(file, req)
	if err != nil {
		_ = r.Close()
		return MultipartFile{}, zeroCmd, cleanup, err
	}
	return file, cmd, func() { _ = r.Close(); cleanup() }, nil
}

// --- Download ---------------------------------------------------------------

// DownloadResponse is what a streaming-download usecase returns. The
// kit handler writes Content-Type / Content-Length / Content-Disposition
// off this struct, sets Accept-Ranges, parses Range: requests, and
// streams the body. Size is required so HEAD requests and Range
// validation can do their thing.
type DownloadResponse struct {
	ContentType string
	Size        int64
	// Filename, when non-empty, populates the
	// `Content-Disposition: attachment; filename="..."` header so
	// browsers offer a save-as dialog.
	Filename string
	// Body is the reader for the response bytes. Closed by the
	// handler after the body is fully written (or on range error).
	Body io.ReadCloser
}

// NewStreamingDownloadHandler wires a GET endpoint whose response
// body is a raw byte stream (a document, an export, a generated
// report) — not a JSON:API document.
//
// The usecase decodes path/query params into a typed Command, opens
// the underlying reader, and returns it; the handler is responsible
// for streaming, range support (`Accept-Ranges: bytes`, parsing the
// client's `Range:` header), and content headers.
//
//	r.Get("/documents/{id}/content",
//	    kitrest.NewStreamingDownloadHandler(
//	        uc.Download,
//	        func(req *http.Request) (DownloadCommand, error) {
//	            return DownloadCommand{ID: req.PathValue("id")}, nil
//	        },
//	    ),
//	)
func NewStreamingDownloadHandler[Cmd any](
	cmdFn func(ctx context.Context, cmd Cmd) (*DownloadResponse, error),
	decoder func(*http.Request) (Cmd, error),
	opts ...HandlerOpt,
) http.Handler {
	_ = opts // accepted for symmetry / future use; no fields consumed today
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		cmd, err := decoder(req)
		if err != nil {
			JsonApiErrorEncoder(ctx, err, w)
			return
		}
		resp, err := cmdFn(ctx, cmd)
		if err != nil {
			JsonApiErrorEncoder(ctx, err, w)
			return
		}
		if resp == nil || resp.Body == nil {
			JsonApiErrorEncoder(ctx, kiterrors.NotFound("download", ""), w)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		writeDownload(w, req, resp)
	})
}

// writeDownload handles the actual streaming: sets headers, honours
// Range requests if the underlying Body supports io.Seeker, falls
// back to a full body otherwise.
func writeDownload(w http.ResponseWriter, req *http.Request, resp *DownloadResponse) {
	if resp.ContentType == "" {
		resp.ContentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", resp.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	if resp.Filename != "" {
		// mime.FormatMediaType safely escapes non-ASCII filenames.
		w.Header().Set("Content-Disposition",
			mime.FormatMediaType("attachment", map[string]string{"filename": resp.Filename}))
	}

	rangeHdr := req.Header.Get("Range")
	seeker, seekable := resp.Body.(io.ReadSeeker)
	if rangeHdr == "" || !seekable {
		// Full-body path. Size header avoids chunked transfer when
		// known; clients can still call HEAD to discover length.
		if resp.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(resp.Size, 10))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	start, end, err := parseSingleByteRange(rangeHdr, resp.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", resp.Size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if _, err := seeker.Seek(start, io.SeekStart); err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", resp.Size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, resp.Size))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = io.CopyN(w, resp.Body, end-start+1)
}

// parseSingleByteRange handles the common case of a single
// `bytes=START-END` range header. Multi-range requests are
// intentionally rejected — the multipart/byteranges shape is rare in
// API traffic and adds notable complexity.
func parseSingleByteRange(header string, size int64) (int64, int64, error) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, errors.New("unsupported range unit")
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, errors.New("multi-range requests not supported")
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid range spec")
	}
	var start, end int64
	var err error
	if parts[0] == "" {
		// Suffix range: "-N" = last N bytes.
		end = size - 1
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, errors.New("invalid suffix length")
		}
		start = size - n
		if start < 0 {
			start = 0
		}
		return start, end, nil
	}
	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, errors.New("invalid start")
	}
	if parts[1] == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid end")
		}
		if end >= size {
			end = size - 1
		}
	}
	if end < start {
		return 0, 0, errors.New("end before start")
	}
	return start, end, nil
}

// jsonapiMarshalSingle writes a single-resource jsonapi document.
// Multipart uploads don't honour `?include=` (rare to need sideloads
// on a creation response), so we don't carry the include list
// through. Kept package-private to discourage reuse outside the
// upload path.
func jsonapiMarshalSingle(buf io.Writer, dto any) error {
	return jsonapi.MarshalPayload(buf, dto)
}
