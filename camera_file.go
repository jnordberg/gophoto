// gophoto - libgphoto2 bindings for golang
// Copyright (C) 2015 Johan Nordberg <code@johan-nordberg.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package gophoto

/*
#cgo pkg-config: libgphoto2
#include <gphoto2.h>

extern int handle_write(void *priv, unsigned char *data, uint64_t *len);
extern int handle_size(void *priv, uint64_t *len);

static inline int cf_handle_write(void *priv, unsigned char *data, uint64_t *len) {
	return handle_write(priv, data, len);
}

static inline int cf_handle_size(void *priv, uint64_t *size) {
	return handle_size(priv, size);
}

static inline void cf_setup_handler(CameraFileHandler *handler) {
	handler->write = cf_handle_write;
	handler->size = cf_handle_size;
}
*/
import "C"

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"runtime"
	"unsafe"
)

//export handle_write
func handle_write(file unsafe.Pointer, data *C.uchar, size *C.uint64_t) C.int {
	length := int(*size)
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(data)),
		Len:  length,
		Cap:  length,
	}
	slice := *(*[]byte)(unsafe.Pointer(&hdr))
	written, err := (*CameraFile)(file).Write(slice)
	*size = C.uint64_t(written)
	if err == nil {
		return C.GP_OK
	} else {
		return C.GP_ERROR
	}
}

//export handle_size
func handle_size(file unsafe.Pointer, size *C.uint64_t) C.int {
	(*CameraFile)(file).size = int(*size)
	return C.GP_OK
}

type fileWrapper struct {
	pointer *C.CameraFile
}

func newFile() *fileWrapper {
	rv := &fileWrapper{}
	runtime.SetFinalizer(rv, (*fileWrapper).free)
	return rv
}

func (self *fileWrapper) free() {
	if self.pointer != nil {
		C.gp_file_free(self.pointer)
		self.pointer = nil
	}
}

type CameraFile struct {
	camera      *cameraWrapper
	context     *contextWrapper
	folder      *cstringWrapper
	name        *cstringWrapper
	deleteAfter bool

	file      *fileWrapper
	buffer    bytes.Buffer
	bytesRead int
	size      int
}

func (self *CameraFile) Write(p []byte) (n int, err error) {
	return self.buffer.Write(p)
}

func (self *CameraFile) setupHandler() error {
	if self.file != nil {
		return errors.New("Handler already set up")
	}

	var result C.int

	self.file = newFile()

	filePtr := (**C.CameraFile)(unsafe.Pointer(&self.file.pointer))
	dataPtr := unsafe.Pointer(self)
	handler := C.CameraFileHandler{}

	C.cf_setup_handler(&handler)

	result = C.gp_file_new_from_handler(filePtr, &handler, dataPtr)
	if result != C.GP_OK {
		return &Error{code: result, message: "Unable to create new file from handler"}
	}

	result = C.gp_camera_file_get(self.camera.pointer, self.folder.pointer,
		self.name.pointer, C.GP_FILE_TYPE_NORMAL, self.file.pointer, self.context.pointer)
	if result != C.GP_OK {
		return &Error{code: result, message: "Unable to get file"}
	}

	return nil
}

func (self *CameraFile) Read(p []byte) (n int, err error) {
	if self.file == nil {
		err := self.setupHandler()
		if err != nil {
			return 0, err
		}
	}
	read, err := self.buffer.Read(p)
	self.bytesRead += read
	if err == io.EOF {
		if self.size > self.bytesRead {
			return read, nil
		} else {
			if self.deleteAfter {
				result := C.gp_camera_file_delete(self.camera.pointer, self.folder.pointer,
					self.name.pointer, self.context.pointer)
				if result != C.GP_OK {
					return read, &Error{code: result, message: "Unable to delete file after downloading"}
				}
			}
			return read, err
		}
	}
	return read, err
}
