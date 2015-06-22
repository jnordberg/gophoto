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
#include <stdlib.h>
#include <gphoto2.h>
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

type Error struct {
	code      C.int
	message   string
	portError bool
}

func (e *Error) Error() string {
	var resultStr string
	if e.portError {
		resultStr = C.GoString(C.gp_port_result_as_string(e.code))
	} else {
		resultStr = C.GoString(C.gp_result_as_string(e.code))
	}
	if e.message != "" {
		return fmt.Sprintf("%v (%v)", e.message, resultStr)
	} else {
		return resultStr
	}
}

type contextWrapper struct {
	pointer *C.GPContext
}

func newContext() *contextWrapper {
	rv := &contextWrapper{
		pointer: C.gp_context_new(),
	}
	runtime.SetFinalizer(rv, (*contextWrapper).free)
	return rv
}

func (self *contextWrapper) free() {
	if self.pointer != nil {
		C.gp_context_unref(self.pointer)
		self.pointer = nil
	}
}

type cstringWrapper struct {
	pointer *C.char
}

func newCstring(str string) *cstringWrapper {
	rv := &cstringWrapper{
		pointer: C.CString(str),
	}
	runtime.SetFinalizer(rv, (*cstringWrapper).free)
	return rv
}

func (self *cstringWrapper) free() {
	if self.pointer != nil {
		C.free(unsafe.Pointer(self.pointer))
		self.pointer = nil
	}
}

func ListCameras() ([]*Camera, error) {
	var result C.int

	context := newContext()
	cameraList := &C.CameraList{}

	result = C.gp_list_new(&cameraList)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}
	defer C.gp_list_unref(cameraList)

	result = C.gp_camera_autodetect(cameraList, context.pointer)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}

	numCameras := C.gp_list_count(cameraList)
	cameras := make([]*Camera, numCameras)

	var cameraIdx C.int
	var model *C.char
	var port *C.char

	defer C.free(unsafe.Pointer(model))
	defer C.free(unsafe.Pointer(port))

	for cameraIdx = 0; cameraIdx < numCameras; cameraIdx++ {
		result = C.gp_list_get_name(cameraList, cameraIdx, &model)
		if result != C.GP_OK {
			return nil, &Error{code: result}
		}
		result = C.gp_list_get_value(cameraList, cameraIdx, &port)
		if result != C.GP_OK {
			return nil, &Error{code: result}
		}
		cameras[cameraIdx] = &Camera{
			Model:   C.GoString(model),
			Port:    C.GoString(port),
			context: context,
		}
	}

	return cameras, nil
}
