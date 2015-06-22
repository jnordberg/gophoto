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
*/
import "C"

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

type cameraWrapper struct {
	pointer *C.Camera
}

func newCamera() *cameraWrapper {
	rv := &cameraWrapper{}
	C.gp_camera_new(&rv.pointer)
	runtime.SetFinalizer(rv, (*cameraWrapper).free)
	return rv
}

func (self *cameraWrapper) free() {
	if self.pointer != nil {
		C.gp_camera_unref(self.pointer)
		self.pointer = nil
	}
}

type Camera struct {
	Model string
	Port  string

	context *contextWrapper
	camera  *cameraWrapper
}

func (self *Camera) String() string {
	return fmt.Sprintf("%v on %v", self.Model, self.Port)
}

func (self *Camera) getCamera() (*cameraWrapper, error) {
	if self.camera == nil {
		var result C.int

		self.camera = newCamera()

		abilitiesList := &C.CameraAbilitiesList{}

		result = C.gp_abilities_list_new(&abilitiesList)
		if result < C.GP_OK {
			return nil, &Error{code: result}
		}
		defer C.gp_abilities_list_free(abilitiesList)

		result = C.gp_abilities_list_load(abilitiesList, self.context.pointer)
		if result != C.GP_OK {
			return nil, &Error{code: result}
		}

		result = C.gp_abilities_list_lookup_model(abilitiesList, C.CString(self.Model))
		if result < C.GP_OK {
			return nil, &Error{code: result}
		}
		abilitiesIdx := result

		var cameraAbilities C.CameraAbilities

		result = C.gp_abilities_list_get_abilities(abilitiesList, abilitiesIdx, &cameraAbilities)
		if result < C.GP_OK {
			return nil, &Error{code: result}
		}

		result = C.gp_camera_set_abilities(self.camera.pointer, cameraAbilities)
		if result < C.GP_OK {
			return nil, &Error{code: result}
		}

		portList := &C.GPPortInfoList{}

		result = C.gp_port_info_list_new(&portList)
		if result != C.GP_OK {
			return nil, &Error{code: result, portError: true}
		}
		defer C.gp_port_info_list_free(portList)

		result = C.gp_port_info_list_load(portList)
		if result < C.GP_OK {
			return nil, &Error{code: result, portError: true}
		}

		result = C.gp_port_info_list_lookup_path(portList, C.CString(self.Port))
		if result < C.GP_OK {
			return nil, &Error{code: result, portError: true}
		}
		portIdx := result

		var portInfo C.GPPortInfo

		result = C.gp_port_info_list_get_info(portList, portIdx, &portInfo)
		if result < C.GP_OK {
			return nil, &Error{code: result, portError: true}
		}

		result = C.gp_camera_set_port_info(self.camera.pointer, portInfo)
		if result < C.GP_OK {
			return nil, &Error{code: result, portError: true}
		}
	}

	return self.camera, nil
}

func (self *Camera) ListDirectory(dir string) ([]string, error) {
	var result C.int

	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	camera, err := self.getCamera()
	if err != nil {
		return nil, err
	}

	folderList := &C.CameraList{}
	result = C.gp_list_new(&folderList)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}
	defer C.gp_list_unref(folderList)

	result = C.gp_camera_folder_list_folders(camera.pointer, C.CString(dir), folderList, self.context.pointer)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}

	numFolders := C.gp_list_count(folderList)
	folders := make([]string, numFolders)

	var folderIdx C.int
	var folderName *C.char

	for folderIdx = 0; folderIdx < numFolders; folderIdx++ {
		result = C.gp_list_get_name(folderList, folderIdx, &folderName)
		if result != C.GP_OK {
			return nil, &Error{code: result}
		}
		folders[folderIdx] = fmt.Sprintf("%s%s/", dir, C.GoString(folderName))
	}

	fileList := &C.CameraList{}
	result = C.gp_list_new(&fileList)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}
	defer C.gp_list_unref(fileList)

	result = C.gp_camera_folder_list_files(camera.pointer, C.CString(dir), fileList, self.context.pointer)
	if result < C.GP_OK {
		return nil, &Error{code: result}
	}

	numFiles := C.gp_list_count(fileList)
	files := make([]string, numFiles)

	var fileIdx C.int
	var fileName *C.char

	for fileIdx = 0; fileIdx < numFiles; fileIdx++ {
		result = C.gp_list_get_name(fileList, fileIdx, &fileName)
		if result != C.GP_OK {
			return nil, &Error{code: result}
		}
		files[fileIdx] = fmt.Sprintf("%s%s", dir, C.GoString(fileName))
	}

	return append(folders, files...), nil
}

func (self *Camera) ListDirectoryRecursive(dir string) ([]string, error) {
	var result []string
	contents, err := self.ListDirectory(dir)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(contents); i++ {
		item := contents[i]
		if strings.HasSuffix(item, "/") {
			items, err := self.ListDirectoryRecursive(item)
			if err != nil {
				return nil, err
			}
			result = append(result, items...)
		} else {
			result = append(result, item)
		}
	}
	return result, nil
}

func (self *Camera) GetFile(path string) (*CameraFile, error) {
	parts := strings.Split(path, "/")
	name := newCstring(parts[len(parts)-1])
	folder := newCstring(strings.Join(parts[:len(parts)-1], "/") + "/")

	camera, err := self.getCamera()
	if err != nil {
		return nil, err
	}

	reader := &CameraFile{
		camera:  camera,
		context: self.context,
		folder:  folder,
		name:    name,
	}

	return reader, nil
}

func (self *Camera) CaptureImage() (*CameraFile, error) {
	camera, err := self.getCamera()
	if err != nil {
		return nil, err
	}

	// wait for a timeout to make sure camera is ready? not sure if it is needed
	// var eventData *interface{}
	// var eventType C.CameraEventType
	// p := unsafe.Pointer(eventData)
	// numTimeouts := 0
	// for {
	// 	result = C.gp_camera_wait_for_event(camera.pointer, 100, &eventType, &p, self.context.pointer)
	// 	switch eventType {
	// 	case C.GP_EVENT_TIMEOUT:
	// 		fmt.Println("timeout")
	// 		break
	// 	case C.GP_EVENT_CAPTURE_COMPLETE:
	// 		fmt.Println("capture complete!")
	// 		break
	// 	case C.GP_EVENT_FILE_ADDED:
	// 		fmt.Println("file added")
	// 		break
	// 	}
	// }

	var result C.int
	path := C.CameraFilePath{}

	result = C.gp_camera_capture(camera.pointer, C.GP_CAPTURE_IMAGE, &path, self.context.pointer)
	if result != C.GP_OK {
		return nil, &Error{code: result}
	}

	folder := C.GoString(&path.folder[0])
	name := C.GoString(&path.name[0])

	reader := &CameraFile{
		camera:      camera,
		context:     self.context,
		folder:      newCstring(folder),
		name:        newCstring(name),
		deleteAfter: true,
	}

	return reader, nil
}

func (self *Camera) CaptureImageTo(file string) error {
	var err error
	reader, err := self.CaptureImage()
	if err != nil {
		return err
	}

	writer, err := os.Create(file)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, reader)
	return err
}
