package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/coreos/container-images/tectonic-error-server/binassets"
)

var (
	addr = flag.String("addr", "0.0.0.0:8080", "address to serve default backend.")

	errorPage = binassets.MustAsset("error.html")
	indexPage = binassets.MustAsset("index.html")
)

type templateData struct {
	ErrCode int
	ErrMsg  string
}

func handleErrorPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("").Parse(string(errorPage))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var errorCode int
	xcodeHeader := r.Header.Get("X-Code")
	if xcodeHeader != "" {
		errorCode, err = strconv.Atoi(xcodeHeader)
		if err != nil {
			msg := "unable to get error code"
			data := templateData{500, msg}
			w.WriteHeader(http.StatusInternalServerError)
			if err := tmpl.Execute(w, data); err != nil {
				log.Println(msg)
			}
			return
		}
	}

	var errMsg string
	switch errorCode {
	case 400:
		errMsg = "Bad Request"
	case 401:
		errMsg = "Unauthorized Access"
	case 403:
		errMsg = "Forbidden"
	case 404:
		errMsg = "Not Found"
	case 500:
		errMsg = "Internal Server Error"
	case 503:
		errMsg = "Service Unavailable"
	case 504:
		errMsg = "Gateway Time-out"
	default:
		w.WriteHeader(404)
		w.Write(indexPage)
		return
	}

	data := templateData{errorCode, errMsg}
	w.WriteHeader(errorCode)
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Unable to execute template.")
		return
	}

}

func main() {
	flag.Parse()
	http.HandleFunc("/", handleErrorPage)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	http.ListenAndServe(fmt.Sprintf("%s", *addr), nil)
}
