package cfdp

import (
	"os"
	"path/filepath"
	"sync"
)

// FilestoreAction is a filestore request action code, per CCSDS 727.0-B-5
// table 5-16.
type FilestoreAction uint8

const (
	ActionCreateFile      FilestoreAction = 0x0
	ActionDeleteFile      FilestoreAction = 0x1
	ActionRenameFile      FilestoreAction = 0x2
	ActionAppendFile      FilestoreAction = 0x3
	ActionReplaceFile     FilestoreAction = 0x4
	ActionCreateDirectory FilestoreAction = 0x5
	ActionRemoveDirectory FilestoreAction = 0x6
	ActionDenyFile        FilestoreAction = 0x7
	ActionDenyDirectory   FilestoreAction = 0x8
)

// String names the action.
func (a FilestoreAction) String() string {
	switch a {
	case ActionCreateFile:
		return "create file"
	case ActionDeleteFile:
		return "delete file"
	case ActionRenameFile:
		return "rename file"
	case ActionAppendFile:
		return "append file"
	case ActionReplaceFile:
		return "replace file"
	case ActionCreateDirectory:
		return "create directory"
	case ActionRemoveDirectory:
		return "remove directory"
	case ActionDenyFile:
		return "deny file"
	case ActionDenyDirectory:
		return "deny directory"
	default:
		return "unknown"
	}
}

// NeedsSecondFileName reports whether this action takes two filenames, per
// table 5-16.
func (a FilestoreAction) NeedsSecondFileName() bool {
	switch a {
	case ActionRenameFile, ActionAppendFile, ActionReplaceFile:
		return true
	default:
		return false
	}
}

// Status codes shared across actions, per table 5-18. Code 0 is success for
// every action and 15 means the action was not attempted.
const (
	// StatusSuccessful is status code 0 for every action.
	StatusSuccessful uint8 = 0x0
	// StatusNotPerformed is status code 15 for every action.
	StatusNotPerformed uint8 = 0xF
)

// FilestoreRequest is the value of a filestore request TLV (table 5-15).
type FilestoreRequest struct {
	Action         FilestoreAction
	FirstFileName  LV
	SecondFileName LV
}

// Encode serializes the request as a TLV of type 00.
func (r FilestoreRequest) Encode() (TLV, error) {
	// Action code (4 bits) then 4 spare bits.
	value := []byte{byte(r.Action&0x0F) << 4}

	first, err := r.FirstFileName.Encode()
	if err != nil {
		return TLV{}, err
	}
	value = append(value, first...)

	if r.Action.NeedsSecondFileName() {
		second, err := r.SecondFileName.Encode()
		if err != nil {
			return TLV{}, err
		}
		value = append(value, second...)
	}
	return TLV{Type: TLVFilestoreRequest, Value: value}, nil
}

// DecodeFilestoreRequest parses a filestore request TLV.
func DecodeFilestoreRequest(t TLV) (*FilestoreRequest, error) {
	if t.Type != TLVFilestoreRequest {
		return nil, ErrInvalidDirectiveCode
	}
	if len(t.Value) < 1 {
		return nil, ErrDataTooShort
	}

	r := &FilestoreRequest{Action: FilestoreAction(t.Value[0] >> 4)}
	offset := 1

	first, n, err := DecodeLV(t.Value[offset:])
	if err != nil {
		return nil, err
	}
	r.FirstFileName = first
	offset += n

	if r.Action.NeedsSecondFileName() {
		second, _, err := DecodeLV(t.Value[offset:])
		if err != nil {
			return nil, err
		}
		r.SecondFileName = second
	}
	return r, nil
}

// FilestoreResponse is the value of a filestore response TLV (table 5-17).
type FilestoreResponse struct {
	Action         FilestoreAction
	StatusCode     uint8
	FirstFileName  LV
	SecondFileName LV
	Message        LV
}

// Encode serializes the response as a TLV of type 01.
func (r FilestoreResponse) Encode() (TLV, error) {
	value := []byte{byte(r.Action&0x0F)<<4 | byte(r.StatusCode&0x0F)}

	first, err := r.FirstFileName.Encode()
	if err != nil {
		return TLV{}, err
	}
	value = append(value, first...)

	if r.Action.NeedsSecondFileName() {
		second, err := r.SecondFileName.Encode()
		if err != nil {
			return TLV{}, err
		}
		value = append(value, second...)
	}

	msg, err := r.Message.Encode()
	if err != nil {
		return TLV{}, err
	}
	return TLV{Type: TLVFilestoreResponse, Value: append(value, msg...)}, nil
}

// DecodeFilestoreResponse parses a filestore response TLV.
func DecodeFilestoreResponse(t TLV) (*FilestoreResponse, error) {
	if t.Type != TLVFilestoreResponse {
		return nil, ErrInvalidDirectiveCode
	}
	if len(t.Value) < 1 {
		return nil, ErrDataTooShort
	}

	r := &FilestoreResponse{
		Action:     FilestoreAction(t.Value[0] >> 4),
		StatusCode: t.Value[0] & 0x0F,
	}
	offset := 1

	first, n, err := DecodeLV(t.Value[offset:])
	if err != nil {
		return nil, err
	}
	r.FirstFileName = first
	offset += n

	if r.Action.NeedsSecondFileName() {
		second, n, err := DecodeLV(t.Value[offset:])
		if err != nil {
			return nil, err
		}
		r.SecondFileName = second
		offset += n
	}

	if offset < len(t.Value) {
		msg, _, err := DecodeLV(t.Value[offset:])
		if err != nil {
			return nil, err
		}
		r.Message = msg
	}
	return r, nil
}

// Filestore is the file system a CFDP entity reads from and writes to.
//
// It is deliberately small: CFDP needs to read a source file, write a
// destination file at arbitrary offsets, and run the handful of filestore
// actions of table 5-16. Anything richer belongs to the application.
type Filestore interface {
	// Read returns the whole contents of name.
	Read(name string) ([]byte, error)
	// WriteAt writes data at a byte offset, growing the file as needed.
	WriteAt(name string, offset uint64, data []byte) error
	// Create makes an empty file, replacing any existing one.
	Create(name string) error
	// Delete removes a file.
	Delete(name string) error
	// Rename moves a file.
	Rename(from, to string) error
	// Size returns the current length of a file.
	Size(name string) (uint64, error)
	// Exists reports whether a file is present.
	Exists(name string) bool
}

// MemoryFilestore is an in-memory Filestore, useful for tests and for
// entities that never touch a disk. It is safe for concurrent use.
type MemoryFilestore struct {
	mu    sync.Mutex
	files map[string][]byte
}

// NewMemoryFilestore returns an empty in-memory filestore.
func NewMemoryFilestore() *MemoryFilestore {
	return &MemoryFilestore{files: make(map[string][]byte)}
}

// Read returns a copy of the file's contents.
func (f *MemoryFilestore) Read(name string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.files[name]
	if !ok {
		return nil, ErrFileNotFound
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// WriteAt writes data at offset, zero-filling any gap.
func (f *MemoryFilestore) WriteAt(name string, offset uint64, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	end := offset + uint64(len(data))
	buf := f.files[name]
	if uint64(len(buf)) < end {
		grown := make([]byte, end)
		copy(grown, buf)
		buf = grown
	}
	copy(buf[offset:end], data)
	f.files[name] = buf
	return nil
}

// Create makes an empty file, discarding any existing contents.
func (f *MemoryFilestore) Create(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name] = nil
	return nil
}

// Delete removes a file.
func (f *MemoryFilestore) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[name]; !ok {
		return ErrFileNotFound
	}
	delete(f.files, name)
	return nil
}

// Rename moves a file.
func (f *MemoryFilestore) Rename(from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.files[from]
	if !ok {
		return ErrFileNotFound
	}
	f.files[to] = data
	delete(f.files, from)
	return nil
}

// Size returns the file length.
func (f *MemoryFilestore) Size(name string) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.files[name]
	if !ok {
		return 0, ErrFileNotFound
	}
	return uint64(len(data)), nil
}

// Exists reports whether the file is present.
func (f *MemoryFilestore) Exists(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.files[name]
	return ok
}

// OSFilestore is a Filestore backed by a directory on disk. Every name is
// resolved inside Root, and names that would escape it are refused.
type OSFilestore struct {
	Root string
}

// NewOSFilestore returns a filestore rooted at dir.
func NewOSFilestore(dir string) *OSFilestore {
	return &OSFilestore{Root: dir}
}

// resolve joins name onto the root and refuses anything that climbs out of it.
func (f *OSFilestore) resolve(name string) (string, error) {
	clean := filepath.Clean("/" + name) // strips any leading ../ sequences
	full := filepath.Join(f.Root, clean)

	rel, err := filepath.Rel(f.Root, full)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 2 && rel[:2] == ".." {
		return "", ErrFilestoreRejection
	}
	return full, nil
}

// Read returns the whole contents of the file.
func (f *OSFilestore) Read(name string) ([]byte, error) {
	path, err := f.resolve(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	return data, err
}

// WriteAt writes data at offset, creating the file and any parent directories.
func (f *OSFilestore) WriteAt(name string, offset uint64, data []byte) error {
	path, err := f.resolve(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteAt(data, int64(offset)); err != nil {
		return err
	}
	return file.Sync()
}

// Create makes an empty file, truncating any existing one.
func (f *OSFilestore) Create(name string) error {
	path, err := f.resolve(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

// Delete removes a file.
func (f *OSFilestore) Delete(name string) error {
	path, err := f.resolve(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}
		return err
	}
	return nil
}

// Rename moves a file.
func (f *OSFilestore) Rename(from, to string) error {
	src, err := f.resolve(from)
	if err != nil {
		return err
	}
	dst, err := f.resolve(to)
	if err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		if os.IsNotExist(err) {
			return ErrFileNotFound
		}
		return err
	}
	return nil
}

// Size returns the file length.
func (f *OSFilestore) Size(name string) (uint64, error) {
	path, err := f.resolve(name)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrFileNotFound
		}
		return 0, err
	}
	return uint64(info.Size()), nil
}

// Exists reports whether the file is present.
func (f *OSFilestore) Exists(name string) bool {
	path, err := f.resolve(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ExecuteFilestoreRequest runs one filestore action and returns the response
// TLV value that belongs in the Finished PDU.
//
// Actions this package does not execute — append, replace, and the directory
// actions — come back with status "not performed" rather than an error, which
// is what table 5-18 provides for.
func ExecuteFilestoreRequest(fs Filestore, req *FilestoreRequest) FilestoreResponse {
	resp := FilestoreResponse{
		Action:         req.Action,
		FirstFileName:  req.FirstFileName,
		SecondFileName: req.SecondFileName,
		StatusCode:     StatusSuccessful,
	}

	name := req.FirstFileName.String()
	var err error

	switch req.Action {
	case ActionCreateFile:
		err = fs.Create(name)
	case ActionDeleteFile:
		err = fs.Delete(name)
	case ActionRenameFile:
		err = fs.Rename(name, req.SecondFileName.String())
	case ActionDenyFile:
		// "Delete if present" — absence is not a failure.
		if fs.Exists(name) {
			err = fs.Delete(name)
		}
	default:
		resp.StatusCode = StatusNotPerformed
		resp.Message = LV{Value: []byte("action not implemented")}
		return resp
	}

	if err != nil {
		resp.StatusCode = StatusNotPerformed
		resp.Message = LV{Value: []byte(err.Error())}
	}
	return resp
}
