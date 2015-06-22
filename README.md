
gophoto
-------

go bindings for [libgphoto2](http://www.gphoto.org) which is a free library that allows you to control [wide range of digital cameras](http://gphoto.sourceforge.net/proj/libgphoto2/support.php).


Requirements
============

 * go 1.4+
 * libgphoto2 2.5.7+


Example
=======

```go
package main

import (
	"github.com/jnordberg/gophoto"
	"io"
	"log"
	"net/http"
	"os"
)

var camera *gophoto.Camera

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/html")
	io.WriteString(w, "<html><body><img src=\"image.jpg\" style=\"width: 100%\" /><body><html>")
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	image, err := camera.CaptureImage()
	if err != nil {
		log.Panicln(err)
	}
	w.Header().Set("content-type", "image/jpeg")
	io.Copy(w, image)
}

func main() {
	var err error

	cameras, err := gophoto.ListCameras()
	if err != nil {
		log.Panicln(err)
	}

	if len(cameras) == 0 {
		log.Println("No cameras found")
		os.Exit(1)
	}

	camera = cameras[0]
	log.Printf("Using camera: %v\n", camera)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/image.jpg", imageHandler)

	log.Println("Starting server http://localhost:8000")
	log.Panicln(http.ListenAndServe(":8000", nil))
}
```
