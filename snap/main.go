package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleMainPage)
	mux.HandleFunc("/create/", handleCreate)
	mux.HandleFunc("/read/", handleRead)

	log.Println("Starting webserver on :80")
	if err := http.ListenAndServe(":80", mux); err != nil {
		log.Fatal("http.ListenAndServe() failed with %s\n", err)
	}
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	log.Printf("create %s", r.URL)
	r.ParseForm()
	data := []byte(r.Form.Get("data"))
	err := ioutil.WriteFile(strings.TrimPrefix(r.URL.Path, "/create"), data, 0666)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot write file: %v", err), http.StatusInternalServerError)
	} else {
		fmt.Fprintf(w, "created OK")
	}
}

func handleRead(w http.ResponseWriter, r *http.Request) {
	log.Printf("read %s", r.URL)
	r.ParseForm()
	data, err := ioutil.ReadFile(strings.TrimPrefix(r.URL.Path, "/read"))
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot read file: %v", err), http.StatusInternalServerError)
	} else {
		w.Write(data)
	}
}

func handleMainPage(w http.ResponseWriter, r *http.Request) {
	log.Printf("got request at %s", r.URL)
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintf(w, "Hello World 0.0.10\n")
}
